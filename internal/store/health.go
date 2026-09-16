package store

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type HealthMetricRow struct {
	DeviceID string
	TsUTC    time.Time
	Metric   string
	Value    *float64
	Values   map[string]float64
	Unit     string
	Source   string
	Metadata map[string]any
}

type HealthMetricResponse struct {
	DeviceID string             `json:"device_id"`
	TsUTC    string             `json:"ts_utc"`
	Metric   string             `json:"metric"`
	Value    *float64           `json:"value,omitempty"`
	Values   map[string]float64 `json:"values,omitempty"`
	Unit     string             `json:"unit,omitempty"`
	Source   string             `json:"source"`
	Metadata map[string]any     `json:"metadata,omitempty"`
	SyncedAt string             `json:"synced_at"`
}

type DailyActivityRow struct {
	DeviceID       string
	DeviceDate     string
	TzOffsetMin    int
	TotalSteps     int
	RunningSteps   int
	Calories       int
	WalkDistanceM  int
	SportDurationS int
	SleepDurationS int
	Source         string
}

type DailyActivityResponse struct {
	DeviceID       string `json:"device_id"`
	DeviceDate     string `json:"device_date"`
	TzOffsetMin    int    `json:"tz_offset_min"`
	TotalSteps     int    `json:"total_steps"`
	RunningSteps   int    `json:"running_steps"`
	Calories       int    `json:"calories"`
	WalkDistanceM  int    `json:"walk_distance_m"`
	SportDurationS int    `json:"sport_duration_s"`
	SleepDurationS int    `json:"sleep_duration_s"`
	Source         string `json:"source"`
	SyncedAt       string `json:"synced_at"`
}

type healthMetricDoc struct {
	DeviceID string             `bson:"device_id"`
	TsUTC    time.Time          `bson:"ts_utc"`
	Metric   string             `bson:"metric"`
	Value    *float64           `bson:"value,omitempty"`
	Values   map[string]float64 `bson:"values,omitempty"`
	Unit     string             `bson:"unit,omitempty"`
	Source   string             `bson:"source"`
	Metadata map[string]any     `bson:"metadata,omitempty"`
	SyncedAt time.Time          `bson:"synced_at"`
}

type dailyActivityDoc struct {
	DeviceID       string    `bson:"device_id"`
	DeviceDate     string    `bson:"device_date"`
	TzOffsetMin    int       `bson:"tz_offset_min"`
	TotalSteps     int       `bson:"total_steps"`
	RunningSteps   int       `bson:"running_steps"`
	Calories       int       `bson:"calories"`
	WalkDistanceM  int       `bson:"walk_distance_m"`
	SportDurationS int       `bson:"sport_duration_s"`
	SleepDurationS int       `bson:"sleep_duration_s"`
	Source         string    `bson:"source"`
	SyncedAt       time.Time `bson:"synced_at"`
}

func (s *Store) IngestHealthMetricsBulk(ctx context.Context, points []HealthMetricRow) (int, error) {
	if len(points) == 0 {
		return 0, nil
	}

	cctx, cancel := ctxTimeout(ctx)
	defer cancel()

	now := time.Now().UTC()
	metrics := s.db.Collection("health_metrics")
	keys := s.db.Collection("health_metric_keys")
	inserted := 0

	for _, p := range points {
		tsUTC := p.TsUTC.UTC()
		src := p.Source
		if src == "" {
			src = "device"
		}

		keyFilter := bson.M{"device_id": p.DeviceID, "metric": p.Metric, "ts_utc": tsUTC, "source": src}
		if _, err := keys.InsertOne(cctx, bson.M{
			"device_id":  p.DeviceID,
			"metric":     p.Metric,
			"ts_utc":     tsUTC,
			"source":     src,
			"created_at": now,
		}); err != nil {
			if mongo.IsDuplicateKeyError(err) {
				continue
			}
			return inserted, err
		}

		doc := healthMetricDoc{
			DeviceID: p.DeviceID,
			TsUTC:    tsUTC,
			Metric:   p.Metric,
			Value:    p.Value,
			Values:   p.Values,
			Unit:     p.Unit,
			Source:   src,
			Metadata: p.Metadata,
			SyncedAt: now,
		}
		if _, err := metrics.InsertOne(cctx, doc); err != nil {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 2*time.Second)
			_, _ = keys.DeleteOne(cleanupCtx, keyFilter)
			cleanupCancel()
			return inserted, err
		}
		inserted++
	}

	return inserted, nil
}

func (s *Store) ListHealthMetrics(ctx context.Context, deviceID, metric string, from, to time.Time) ([]HealthMetricResponse, error) {
	cctx, cancel := ctxTimeout(ctx)
	defer cancel()

	filter := bson.M{
		"device_id": deviceID,
		"ts_utc": bson.M{
			"$gte": from.UTC(),
			"$lt":  to.UTC(),
		},
	}
	if metric != "" {
		filter["metric"] = metric
	}

	cursor, err := s.db.Collection("health_metrics").Find(
		cctx,
		filter,
		options.Find().SetSort(bson.D{{Key: "ts_utc", Value: 1}, {Key: "metric", Value: 1}}),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(cctx)

	var out []HealthMetricResponse
	for cursor.Next(cctx) {
		var doc healthMetricDoc
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		out = append(out, healthMetricDocToResponse(doc))
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) UpsertDailyActivity(ctx context.Context, p DailyActivityRow) (*DailyActivityResponse, error) {
	cctx, cancel := ctxTimeout(ctx)
	defer cancel()

	src := p.Source
	if src == "" {
		src = "device"
	}
	set := bson.M{
		"device_id":        p.DeviceID,
		"device_date":      p.DeviceDate,
		"tz_offset_min":    p.TzOffsetMin,
		"total_steps":      p.TotalSteps,
		"running_steps":    p.RunningSteps,
		"calories":         p.Calories,
		"walk_distance_m":  p.WalkDistanceM,
		"sport_duration_s": p.SportDurationS,
		"sleep_duration_s": p.SleepDurationS,
		"source":           src,
		"synced_at":        time.Now().UTC(),
	}

	var doc dailyActivityDoc
	err := s.db.Collection("daily_activity").FindOneAndUpdate(
		cctx,
		bson.M{"device_id": p.DeviceID, "device_date": p.DeviceDate},
		bson.M{"$set": set},
		options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After),
	).Decode(&doc)
	if err != nil {
		return nil, err
	}
	return dailyActivityDocToResponse(doc), nil
}

func (s *Store) ListDailyActivity(ctx context.Context, deviceID, fromDate, toDate string) ([]DailyActivityResponse, error) {
	cctx, cancel := ctxTimeout(ctx)
	defer cancel()

	cursor, err := s.db.Collection("daily_activity").Find(
		cctx,
		bson.M{
			"device_id": deviceID,
			"device_date": bson.M{
				"$gte": fromDate,
				"$lte": toDate,
			},
		},
		options.Find().SetSort(bson.D{{Key: "device_date", Value: 1}}),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(cctx)

	var out []DailyActivityResponse
	for cursor.Next(cctx) {
		var doc dailyActivityDoc
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		out = append(out, *dailyActivityDocToResponse(doc))
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func healthMetricDocToResponse(doc healthMetricDoc) HealthMetricResponse {
	return HealthMetricResponse{
		DeviceID: doc.DeviceID,
		TsUTC:    doc.TsUTC.Format(time.RFC3339),
		Metric:   doc.Metric,
		Value:    doc.Value,
		Values:   doc.Values,
		Unit:     doc.Unit,
		Source:   doc.Source,
		Metadata: doc.Metadata,
		SyncedAt: doc.SyncedAt.Format(time.RFC3339),
	}
}

func dailyActivityDocToResponse(doc dailyActivityDoc) *DailyActivityResponse {
	return &DailyActivityResponse{
		DeviceID:       doc.DeviceID,
		DeviceDate:     doc.DeviceDate,
		TzOffsetMin:    doc.TzOffsetMin,
		TotalSteps:     doc.TotalSteps,
		RunningSteps:   doc.RunningSteps,
		Calories:       doc.Calories,
		WalkDistanceM:  doc.WalkDistanceM,
		SportDurationS: doc.SportDurationS,
		SleepDurationS: doc.SleepDurationS,
		Source:         doc.Source,
		SyncedAt:       doc.SyncedAt.Format(time.RFC3339),
	}
}
