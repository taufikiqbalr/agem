package store

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
	"strings"
	"time"
)

func (s *Store) upsertV3SleepSession(ctx context.Context, deviceID, sleepDate string, tzOffsetMin int, session WearableSleepSession) (int, int, error) {
	if strings.TrimSpace(session.Start) == "" {
		return 0, 0, wearablePayloadError("sleep session start is required")
	}
	start, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(session.Start))
	if err != nil {
		return 0, 0, wearablePayloadError("invalid sleep session start")
	}
	start = start.UTC()
	protocol := sanitizeName(firstNonEmpty(session.Protocol, "unknown"))
	sessionType := sanitizeName(firstNonEmpty(session.Type, "main"))
	sessionID := strings.TrimSpace(session.SessionID)
	if sessionID == "" {
		sessionID = fmt.Sprintf("%s:%s:%d", sleepDate, sessionType, start.Unix())
	}

	normalizedSegments, derived, end, err := normalizeSleepSegments(start, session.End, protocol, session.Segments)
	if err != nil {
		return 0, 0, err
	}
	if end.IsZero() {
		return 0, 0, wearablePayloadError("sleep session end or segment durations are required")
	}

	totalSleep := valueOrDefault(session.TotalSleepS, derived.totalSleepS)
	deep := valueOrDefault(session.DeepS, derived.deepS)
	light := valueOrDefault(session.LightS, derived.lightS)
	rem := valueOrDefault(session.RemS, derived.remS)
	awake := valueOrDefault(session.AwakeS, derived.awakeS)
	wakingCount := valueOrDefault(session.WakingCount, derived.wakingCount)
	now := time.Now().UTC()
	source := firstNonEmpty(strings.TrimSpace(session.Source), wearableV3Source)

	set := bson.M{
		"device_id":     deviceID,
		"sleep_date":    sleepDate,
		"tz_offset_min": tzOffsetMin,
		"session_id":    sessionID,
		"session_type":  sessionType,
		"protocol":      protocol,
		"start_utc":     start,
		"end_utc":       end,
		"total_sleep_s": totalSleep,
		"deep_s":        deep,
		"light_s":       light,
		"rem_s":         rem,
		"awake_s":       awake,
		"waking_count":  wakingCount,
		"source":        source,
		"synced_at":     now,
		"updated_at":    now,
	}
	if session.StageData != nil {
		set["stage_data"] = session.StageData
	}
	if session.Metadata != nil {
		set["metadata"] = session.Metadata
	}

	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if _, err := s.db.Collection("sleep_sessions").UpdateOne(
		cctx,
		bson.M{"device_id": deviceID, "session_id": sessionID},
		bson.M{"$set": set, "$setOnInsert": bson.M{"_id": primitive.NewObjectID(), "created_at": now}},
		options.Update().SetUpsert(true),
	); err != nil {
		return 0, 0, err
	}

	segmentCount := 0
	for _, segment := range normalizedSegments {
		inserted, err := s.ingestV3SleepSegment(cctx, deviceID, sleepDate, tzOffsetMin, sessionID, protocol, source, segment)
		if err != nil {
			return 1, segmentCount, err
		}
		segmentCount += inserted
	}
	return 1, segmentCount, nil
}

type normalizedSleepSegment struct {
	start     time.Time
	end       time.Time
	stage     string
	stageCode *int
}

type derivedSleepTotals struct {
	totalSleepS int
	deepS       int
	lightS      int
	remS        int
	awakeS      int
	wakingCount int
}
