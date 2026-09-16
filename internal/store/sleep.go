package store

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type SleepSummaryRow struct {
	DeviceID      string
	SleepDate     string
	TzOffsetMin   int
	SleepStartUTC *time.Time
	WakeUTC       *time.Time
	TotalSleepS   int
	DeepS         int
	LightS        int
	AwakeS        int
	RemS          int
	Score         *int
	StageData     []int
	Source        string
}

type SleepSummaryResponse struct {
	DeviceID      string  `json:"device_id"`
	SleepDate     string  `json:"sleep_date"`
	TzOffsetMin   int     `json:"tz_offset_min"`
	SleepStartUTC *string `json:"sleep_start_utc,omitempty"`
	WakeUTC       *string `json:"wake_utc,omitempty"`
	TotalSleepS   int     `json:"total_sleep_s"`
	DeepS         int     `json:"deep_s"`
	LightS        int     `json:"light_s"`
	AwakeS        int     `json:"awake_s"`
	RemS          int     `json:"rem_s"`
	Score         *int    `json:"score,omitempty"`
	StageData     []int   `json:"stage_data,omitempty"`
	Source        string  `json:"source"`
	SyncedAt      string  `json:"synced_at"`
}

type SleepSegmentRow struct {
	DeviceID    string
	StartUTC    time.Time
	EndUTC      time.Time
	SleepDate   string
	TzOffsetMin int
	Stage       string
	Source      string
}

type SleepSegmentResponse struct {
	StartUTC    string `json:"start_utc"`
	EndUTC      string `json:"end_utc"`
	SleepDate   string `json:"sleep_date"`
	TzOffsetMin int    `json:"tz_offset_min"`
	Stage       string `json:"stage"`
	Source      string `json:"source"`
	SyncedAt    string `json:"synced_at"`
}

type sleepSummaryDoc struct {
	DeviceID      string     `bson:"device_id"`
	SleepDate     string     `bson:"sleep_date"`
	TzOffsetMin   int        `bson:"tz_offset_min"`
	SleepStartUTC *time.Time `bson:"sleep_start_utc,omitempty"`
	WakeUTC       *time.Time `bson:"wake_utc,omitempty"`
	TotalSleepS   int        `bson:"total_sleep_s"`
	DeepS         int        `bson:"deep_s"`
	LightS        int        `bson:"light_s"`
	AwakeS        int        `bson:"awake_s"`
	RemS          int        `bson:"rem_s"`
	Score         *int       `bson:"score,omitempty"`
	StageData     []int      `bson:"stage_data,omitempty"`
	Source        string     `bson:"source"`
	SyncedAt      time.Time  `bson:"synced_at"`
}

type sleepSegmentDoc struct {
	DeviceID    string    `bson:"device_id"`
	StartUTC    time.Time `bson:"start_utc"`
	EndUTC      time.Time `bson:"end_utc"`
	SleepDate   string    `bson:"sleep_date"`
	TzOffsetMin int       `bson:"tz_offset_min"`
	Stage       string    `bson:"stage"`
	Source      string    `bson:"source"`
	SyncedAt    time.Time `bson:"synced_at"`
}

func (s *Store) UpsertSleepSummary(ctx context.Context, p SleepSummaryRow) (*SleepSummaryResponse, error) {
	cctx, cancel := ctxTimeout(ctx)
	defer cancel()

	src := p.Source
	if src == "" {
		src = "device"
	}

	set := bson.M{
		"device_id":       p.DeviceID,
		"sleep_date":      p.SleepDate,
		"tz_offset_min":   p.TzOffsetMin,
		"sleep_start_utc": p.SleepStartUTC,
		"wake_utc":        p.WakeUTC,
		"total_sleep_s":   p.TotalSleepS,
		"deep_s":          p.DeepS,
		"light_s":         p.LightS,
		"awake_s":         p.AwakeS,
		"rem_s":           p.RemS,
		"source":          src,
		"synced_at":       time.Now().UTC(),
	}
	if p.Score != nil {
		set["score"] = *p.Score
	}
	if p.StageData != nil {
		set["stage_data"] = p.StageData
	}

	var doc sleepSummaryDoc
	err := s.db.Collection("sleep_summary").FindOneAndUpdate(
		cctx,
		bson.M{"device_id": p.DeviceID, "sleep_date": p.SleepDate},
		bson.M{"$set": set},
		options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After),
	).Decode(&doc)
	if err != nil {
		return nil, err
	}
	return sleepSummaryDocToResponse(doc), nil
}

func (s *Store) GetSleepSummaryByDate(ctx context.Context, deviceID, date string) (*SleepSummaryResponse, error) {
	cctx, cancel := ctxTimeout(ctx)
	defer cancel()

	var doc sleepSummaryDoc
	if err := s.db.Collection("sleep_summary").FindOne(cctx, bson.M{"device_id": deviceID, "sleep_date": date}).Decode(&doc); err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return sleepSummaryDocToResponse(doc), nil
}

func (s *Store) UpsertSleepSegmentsBulk(ctx context.Context, segments []SleepSegmentRow) (int, error) {
	if len(segments) == 0 {
		return 0, nil
	}

	cctx, cancel := ctxTimeout(ctx)
	defer cancel()

	now := time.Now().UTC()
	segmentCol := s.db.Collection("sleep_segments")
	keyCol := s.db.Collection("sleep_segments_keys")
	inserted := 0

	for _, seg := range segments {
		startUTC := seg.StartUTC.UTC()
		stage := seg.Stage
		if stage == "" {
			stage = "unknown"
		}
		src := seg.Source
		if src == "" {
			src = "device"
		}

		keyFilter := bson.M{"device_id": seg.DeviceID, "start_utc": startUTC}
		if _, err := keyCol.InsertOne(cctx, bson.M{
			"device_id":  seg.DeviceID,
			"start_utc":  startUTC,
			"created_at": now,
		}); err != nil {
			if mongo.IsDuplicateKeyError(err) {
				continue
			}
			return inserted, err
		}

		doc := sleepSegmentDoc{
			DeviceID:    seg.DeviceID,
			StartUTC:    startUTC,
			EndUTC:      seg.EndUTC.UTC(),
			SleepDate:   seg.SleepDate,
			TzOffsetMin: seg.TzOffsetMin,
			Stage:       stage,
			Source:      src,
			SyncedAt:    now,
		}
		if _, err := segmentCol.InsertOne(cctx, doc); err != nil {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 2*time.Second)
			_, _ = keyCol.DeleteOne(cleanupCtx, keyFilter)
			cleanupCancel()
			return inserted, err
		}
		inserted++
	}

	return inserted, nil
}

func (s *Store) ListSleepSegmentsByDate(ctx context.Context, deviceID, sleepDate string) ([]SleepSegmentResponse, error) {
	cctx, cancel := ctxTimeout(ctx)
	defer cancel()

	cursor, err := s.db.Collection("sleep_segments").Find(
		cctx,
		bson.M{"device_id": deviceID, "sleep_date": sleepDate},
		options.Find().SetSort(bson.D{{Key: "start_utc", Value: 1}}),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(cctx)

	var out []SleepSegmentResponse
	for cursor.Next(cctx) {
		var doc sleepSegmentDoc
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		out = append(out, sleepSegmentDocToResponse(doc))
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func sleepSummaryDocToResponse(doc sleepSummaryDoc) *SleepSummaryResponse {
	resp := &SleepSummaryResponse{
		DeviceID:    doc.DeviceID,
		SleepDate:   doc.SleepDate,
		TzOffsetMin: doc.TzOffsetMin,
		TotalSleepS: doc.TotalSleepS,
		DeepS:       doc.DeepS,
		LightS:      doc.LightS,
		AwakeS:      doc.AwakeS,
		RemS:        doc.RemS,
		Score:       doc.Score,
		StageData:   doc.StageData,
		Source:      doc.Source,
		SyncedAt:    doc.SyncedAt.Format(time.RFC3339),
	}
	if doc.SleepStartUTC != nil {
		start := doc.SleepStartUTC.Format(time.RFC3339)
		resp.SleepStartUTC = &start
	}
	if doc.WakeUTC != nil {
		wake := doc.WakeUTC.Format(time.RFC3339)
		resp.WakeUTC = &wake
	}
	return resp
}

func sleepSegmentDocToResponse(doc sleepSegmentDoc) SleepSegmentResponse {
	return SleepSegmentResponse{
		StartUTC:    doc.StartUTC.Format(time.RFC3339),
		EndUTC:      doc.EndUTC.Format(time.RFC3339),
		SleepDate:   doc.SleepDate,
		TzOffsetMin: doc.TzOffsetMin,
		Stage:       doc.Stage,
		Source:      doc.Source,
		SyncedAt:    doc.SyncedAt.Format(time.RFC3339),
	}
}
