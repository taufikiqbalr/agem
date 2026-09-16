package store

import (
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"
)

func (s *Store) ingestV3SleepSegment(ctx context.Context, deviceID, sleepDate string, tzOffsetMin int, sessionID, protocol, source string, segment normalizedSleepSegment) (int, error) {
	now := time.Now().UTC()
	keyFilter := bson.M{"device_id": deviceID, "session_id": sessionID, "start_utc": segment.start}
	if _, err := s.db.Collection("sleep_segment_v3_keys").InsertOne(ctx, bson.M{
		"device_id": deviceID, "session_id": sessionID, "start_utc": segment.start, "created_at": now,
	}); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return 0, nil
		}
		return 0, err
	}

	doc := bson.M{
		"device_id":     deviceID,
		"sleep_date":    sleepDate,
		"tz_offset_min": tzOffsetMin,
		"session_id":    sessionID,
		"protocol":      protocol,
		"start_utc":     segment.start,
		"end_utc":       segment.end,
		"stage":         segment.stage,
		"source":        source,
		"synced_at":     now,
	}
	if segment.stageCode != nil {
		doc["stage_code"] = *segment.stageCode
	}
	if _, err := s.db.Collection("sleep_segments").InsertOne(ctx, doc); err != nil {
		_, _ = s.db.Collection("sleep_segment_v3_keys").DeleteOne(context.Background(), keyFilter)
		return 0, err
	}
	return 1, nil
}

func (s *Store) refreshV3SleepSummary(ctx context.Context, deviceID, sleepDate string, tzOffsetMin int) error {
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cursor, err := s.db.Collection("sleep_sessions").Find(cctx, bson.M{"device_id": deviceID, "sleep_date": sleepDate})
	if err != nil {
		return err
	}
	defer cursor.Close(cctx)

	var total, deep, light, rem, awake, waking, sessionCount int
	var firstStart, lastEnd *time.Time
	for cursor.Next(cctx) {
		var doc struct {
			StartUTC    time.Time `bson:"start_utc"`
			EndUTC      time.Time `bson:"end_utc"`
			TotalSleepS int       `bson:"total_sleep_s"`
			DeepS       int       `bson:"deep_s"`
			LightS      int       `bson:"light_s"`
			RemS        int       `bson:"rem_s"`
			AwakeS      int       `bson:"awake_s"`
			WakingCount int       `bson:"waking_count"`
		}
		if err := cursor.Decode(&doc); err != nil {
			return err
		}
		total += doc.TotalSleepS
		deep += doc.DeepS
		light += doc.LightS
		rem += doc.RemS
		awake += doc.AwakeS
		waking += doc.WakingCount
		sessionCount++
		start := doc.StartUTC.UTC()
		end := doc.EndUTC.UTC()
		if firstStart == nil || start.Before(*firstStart) {
			firstStart = &start
		}
		if lastEnd == nil || end.After(*lastEnd) {
			lastEnd = &end
		}
	}
	if err := cursor.Err(); err != nil {
		return err
	}
	if sessionCount == 0 {
		return nil
	}

	now := time.Now().UTC()
	set := bson.M{
		"device_id":      deviceID,
		"sleep_date":     sleepDate,
		"tz_offset_min":  tzOffsetMin,
		"total_sleep_s":  total,
		"deep_s":         deep,
		"light_s":        light,
		"rem_s":          rem,
		"awake_s":        awake,
		"waking_count":   waking,
		"sessions_count": sessionCount,
		"source":         wearableV3Source,
		"synced_at":      now,
	}
	if firstStart != nil {
		set["sleep_start_utc"] = *firstStart
	}
	if lastEnd != nil {
		set["wake_utc"] = *lastEnd
	}
	_, err = s.db.Collection("sleep_summary").UpdateOne(
		cctx,
		bson.M{"device_id": deviceID, "sleep_date": sleepDate},
		bson.M{"$set": set, "$setOnInsert": bson.M{"_id": primitive.NewObjectID(), "created_at": now}},
		options.Update().SetUpsert(true),
	)
	return err
}
