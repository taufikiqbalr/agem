package store

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Step15mRow struct {
	DeviceID       string
	TsUTC          time.Time
	DeviceDate     string
	TimeIndex      int
	TzOffsetMin    int
	WalkSteps      int
	RunSteps       int
	Calories       int
	DistanceM      int
	SportDurationS int
	Source         string
}

type Steps15mResponse struct {
	TsUTC          string `json:"ts_utc"`
	DeviceDate     string `json:"device_date"`
	TimeIndex      int    `json:"time_index"`
	WalkSteps      int    `json:"walk_steps"`
	RunSteps       int    `json:"run_steps"`
	Calories       int    `json:"calories"`
	DistanceM      int    `json:"distance_m"`
	SportDurationS int    `json:"sport_duration_s"`
	TzOffsetMin    int    `json:"tz_offset_min"`
	SyncedAt       string `json:"synced_at"`
}

type StepsDailyRow struct {
	DayUTC       string `json:"day_utc"`
	TotalSteps   int64  `json:"total_steps"`
	RunningSteps int64  `json:"running_steps"`
	Calories     int64  `json:"calories"`
	DistanceM    int64  `json:"distance_m"`
	LastSyncedAt string `json:"last_synced_at"`
}

type step15mDoc struct {
	DeviceID       string    `bson:"device_id"`
	TsUTC          time.Time `bson:"ts_utc"`
	DeviceDate     string    `bson:"device_date"`
	TimeIndex      int       `bson:"time_index"`
	TzOffsetMin    int       `bson:"tz_offset_min"`
	WalkSteps      int       `bson:"walk_steps"`
	RunSteps       int       `bson:"run_steps"`
	Calories       int       `bson:"calories"`
	DistanceM      int       `bson:"distance_m"`
	SportDurationS int       `bson:"sport_duration_s"`
	Source         string    `bson:"source"`
	SyncedAt       time.Time `bson:"synced_at"`
}

func (s *Store) UpsertSteps15mBulk(ctx context.Context, points []Step15mRow) (int, error) {
	if len(points) == 0 {
		return 0, nil
	}

	cctx, cancel := ctxTimeout(ctx)
	defer cancel()

	now := time.Now().UTC()
	steps := s.db.Collection("steps_15m")
	keys := s.db.Collection("steps_15m_keys")
	inserted := 0

	for _, p := range points {
		tsUTC := p.TsUTC.UTC()
		src := p.Source
		if src == "" {
			src = "device"
		}

		keyFilter := bson.M{"device_id": p.DeviceID, "ts_utc": tsUTC}
		if _, err := keys.InsertOne(cctx, bson.M{
			"device_id":  p.DeviceID,
			"ts_utc":     tsUTC,
			"created_at": now,
		}); err != nil {
			if mongo.IsDuplicateKeyError(err) {
				continue
			}
			return inserted, err
		}

		doc := step15mDoc{
			DeviceID:       p.DeviceID,
			TsUTC:          tsUTC,
			DeviceDate:     p.DeviceDate,
			TimeIndex:      p.TimeIndex,
			TzOffsetMin:    p.TzOffsetMin,
			WalkSteps:      p.WalkSteps,
			RunSteps:       p.RunSteps,
			Calories:       p.Calories,
			DistanceM:      p.DistanceM,
			SportDurationS: p.SportDurationS,
			Source:         src,
			SyncedAt:       now,
		}
		if _, err := steps.InsertOne(cctx, doc); err != nil {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 2*time.Second)
			_, _ = keys.DeleteOne(cleanupCtx, keyFilter)
			cleanupCancel()
			return inserted, err
		}
		inserted++
	}

	return inserted, nil
}

func (s *Store) ListSteps15mByDeviceDate(ctx context.Context, deviceID, deviceDate string) ([]Steps15mResponse, error) {
	cctx, cancel := ctxTimeout(ctx)
	defer cancel()

	cursor, err := s.db.Collection("steps_15m").Find(
		cctx,
		bson.M{"device_id": deviceID, "device_date": deviceDate},
		options.Find().SetSort(bson.D{{Key: "time_index", Value: 1}}),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(cctx)

	var out []Steps15mResponse
	for cursor.Next(cctx) {
		var doc step15mDoc
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		out = append(out, step15mDocToResponse(doc))
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) ListStepsDailyCagg(ctx context.Context, deviceID string, from, to time.Time) ([]StepsDailyRow, error) {
	cctx, cancel := ctxTimeout(ctx)
	defer cancel()

	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"device_id": deviceID,
			"ts_utc": bson.M{
				"$gte": from.UTC(),
				"$lt":  to.UTC(),
			},
		}}},
		{{Key: "$addFields", Value: bson.M{
			"day_utc": bson.M{"$dateTrunc": bson.M{"date": "$ts_utc", "unit": "day", "timezone": "UTC"}},
		}}},
		{{Key: "$group", Value: bson.M{
			"_id":            "$day_utc",
			"total_steps":    bson.M{"$sum": bson.M{"$add": bson.A{"$walk_steps", "$run_steps"}}},
			"running_steps":  bson.M{"$sum": "$run_steps"},
			"calories":       bson.M{"$sum": "$calories"},
			"distance_m":     bson.M{"$sum": "$distance_m"},
			"last_synced_at": bson.M{"$max": "$synced_at"},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "_id", Value: 1}}}},
	}

	cursor, err := s.db.Collection("steps_15m").Aggregate(cctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(cctx)

	var out []StepsDailyRow
	for cursor.Next(cctx) {
		var row struct {
			DayUTC       time.Time `bson:"_id"`
			TotalSteps   int64     `bson:"total_steps"`
			RunningSteps int64     `bson:"running_steps"`
			Calories     int64     `bson:"calories"`
			DistanceM    int64     `bson:"distance_m"`
			LastSyncedAt time.Time `bson:"last_synced_at"`
		}
		if err := cursor.Decode(&row); err != nil {
			return nil, err
		}
		out = append(out, StepsDailyRow{
			DayUTC:       row.DayUTC.Format(time.RFC3339),
			TotalSteps:   row.TotalSteps,
			RunningSteps: row.RunningSteps,
			Calories:     row.Calories,
			DistanceM:    row.DistanceM,
			LastSyncedAt: row.LastSyncedAt.Format(time.RFC3339),
		})
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func step15mDocToResponse(doc step15mDoc) Steps15mResponse {
	return Steps15mResponse{
		TsUTC:          doc.TsUTC.Format(time.RFC3339),
		DeviceDate:     doc.DeviceDate,
		TimeIndex:      doc.TimeIndex,
		WalkSteps:      doc.WalkSteps,
		RunSteps:       doc.RunSteps,
		Calories:       doc.Calories,
		DistanceM:      doc.DistanceM,
		SportDurationS: doc.SportDurationS,
		TzOffsetMin:    doc.TzOffsetMin,
		SyncedAt:       doc.SyncedAt.Format(time.RFC3339),
	}
}
