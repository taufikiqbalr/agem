package store

import (
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"strings"
	"time"
)

func (s *Store) upsertV3Target(ctx context.Context, deviceID, deviceUID, deviceDate string, target WearableTargetProgress, index int) error {
	kind := sanitizeName(target.Kind)
	if kind == "" {
		return wearablePayloadError("target.kind is required")
	}
	now := time.Now().UTC()
	set := bson.M{
		"device_id":   deviceID,
		"device_uid":  deviceUID,
		"device_date": deviceDate,
		"kind":        kind,
		"index":       index,
		"source":      firstNonEmpty(strings.TrimSpace(target.Source), wearableV3Source),
		"synced_at":   now,
	}
	if target.Value != nil {
		set["value"] = *target.Value
	}
	if target.Target != nil {
		set["target"] = *target.Target
	}
	if unit := strings.TrimSpace(target.Unit); unit != "" {
		set["unit"] = unit
	}
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := s.db.Collection("total_activities").UpdateOne(
		cctx,
		bson.M{"device_id": deviceID, "device_date": deviceDate, "kind": kind},
		bson.M{"$set": set, "$setOnInsert": bson.M{"_id": primitive.NewObjectID(), "created_at": now}},
		options.Update().SetUpsert(true),
	)
	return err
}

func (s *Store) upsertV3Workout(ctx context.Context, deviceID, deviceDate string, tzOffsetMin int, workout WearableWorkout) error {
	if strings.TrimSpace(workout.StartUTC) == "" {
		return wearablePayloadError("workout.startUtc is required")
	}
	start, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(workout.StartUTC))
	if err != nil {
		return wearablePayloadError("invalid workout.startUtc")
	}
	start = start.UTC()
	var end *time.Time
	if strings.TrimSpace(workout.EndUTC) != "" {
		parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(workout.EndUTC))
		if err != nil {
			return wearablePayloadError("invalid workout.endUtc")
		}
		value := parsed.UTC()
		end = &value
	} else if workout.DurationS != nil {
		value := start.Add(time.Duration(*workout.DurationS) * time.Second)
		end = &value
	}
	now := time.Now().UTC()
	set := bson.M{
		"device_id":     deviceID,
		"device_date":   deviceDate,
		"start_utc":     start,
		"tz_offset_min": tzOffsetMin,
		"source":        firstNonEmpty(strings.TrimSpace(workout.Source), wearableV3Source),
		"synced_at":     now,
	}
	if end != nil {
		set["end_utc"] = *end
	}
	if workout.SportTypeID != nil {
		set["sport_type_id"] = *workout.SportTypeID
	}
	if value := strings.TrimSpace(workout.SportTypeName); value != "" {
		set["sport_type_name"] = value
	}
	if workout.DurationS != nil {
		set["duration_s"] = *workout.DurationS
	}
	if workout.DistanceM != nil {
		set["distance_m"] = *workout.DistanceM
	}
	if workout.Calories != nil {
		set["calories"] = *workout.Calories
	}
	if workout.AvgHR != nil {
		set["avg_hr"] = *workout.AvgHR
	}
	if workout.MaxHR != nil {
		set["max_hr"] = *workout.MaxHR
	}
	if workout.MinHR != nil {
		set["min_hr"] = *workout.MinHR
	}
	if workout.AvgSpeedCmS != nil {
		set["avg_speed_cm_s"] = *workout.AvgSpeedCmS
	}
	if workout.MaxSpeedCmS != nil {
		set["max_speed_cm_s"] = *workout.MaxSpeedCmS
	}
	if workout.ElevationCm != nil {
		set["elevation_cm"] = *workout.ElevationCm
	}
	if workout.UphillCm != nil {
		set["uphill_cm"] = *workout.UphillCm
	}
	if workout.DownhillCm != nil {
		set["downhill_cm"] = *workout.DownhillCm
	}
	if workout.AvgCadenceSPM != nil {
		set["avg_cadence_spm"] = *workout.AvgCadenceSPM
	}
	if workout.SportCount != nil {
		set["sport_count"] = *workout.SportCount
	}
	if workout.Steps != nil {
		set["steps"] = *workout.Steps
	}
	if value := sanitizeName(workout.Status); value != "" {
		set["status"] = value
	}
	if workout.Locations != nil {
		set["locations"] = workout.Locations
	}
	if workout.Metadata != nil {
		set["metadata"] = workout.Metadata
	}

	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err = s.db.Collection("workouts").UpdateOne(
		cctx,
		bson.M{"device_id": deviceID, "start_utc": start},
		bson.M{"$set": set, "$setOnInsert": bson.M{"_id": primitive.NewObjectID(), "created_at": now}},
		options.Update().SetUpsert(true),
	)
	return err
}

func (s *Store) ingestV3Event(ctx context.Context, deviceID, deviceDate string, tzOffsetMin int, event WearableDeviceEvent, ts time.Time) (int, error) {
	eventType := sanitizeName(event.EventType)
	if eventType == "" {
		return 0, wearablePayloadError("event.eventType is required")
	}
	source := sanitizeName(firstNonEmpty(event.Source, wearableV3Source))
	sessionID := strings.TrimSpace(event.SessionID)
	ts = ts.UTC()
	now := time.Now().UTC()

	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	key := bson.M{
		"device_id": deviceID, "event_type": eventType, "ts_utc": ts, "session_id": sessionID, "source": source,
	}
	keyDoc := cloneBSON(key)
	keyDoc["created_at"] = now
	if _, err := s.db.Collection("device_event_v3_keys").InsertOne(cctx, keyDoc); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return 0, nil
		}
		return 0, err
	}
	doc := bson.M{
		"device_id": deviceID, "device_date": deviceDate, "tz_offset_min": tzOffsetMin,
		"ts_utc": ts, "event_type": eventType, "session_id": sessionID, "source": source,
		"payload": event.Payload, "synced_at": now,
	}
	if _, err := s.db.Collection("device_events").InsertOne(cctx, doc); err != nil {
		_, _ = s.db.Collection("device_event_v3_keys").DeleteOne(context.Background(), key)
		return 0, err
	}
	return 1, nil
}

func (s *Store) ingestV3RawSample(ctx context.Context, deviceID, deviceDate string, tzOffsetMin int, sample WearableRawSensorSample) (int, error) {
	if strings.TrimSpace(sample.Ts) == "" {
		return 0, wearablePayloadError("raw sample ts is required")
	}
	if len(sample.Channels) == 0 {
		return 0, wearablePayloadError("raw sample channels is required")
	}
	ts, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(sample.Ts))
	if err != nil {
		return 0, wearablePayloadError("invalid raw sample ts")
	}
	ts = ts.UTC()
	kind := sanitizeName(firstNonEmpty(sample.Kind, "sensor"))
	sessionID := strings.TrimSpace(sample.SessionID)
	now := time.Now().UTC()

	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	key := bson.M{
		"device_id": deviceID, "session_id": sessionID, "ts_utc": ts, "sample_index": sample.SampleIndex, "kind": kind,
	}
	keyDoc := cloneBSON(key)
	keyDoc["created_at"] = now
	if _, err := s.db.Collection("raw_sensor_sample_keys").InsertOne(cctx, keyDoc); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return 0, nil
		}
		return 0, err
	}
	doc := bson.M{
		"device_id": deviceID, "device_date": deviceDate, "tz_offset_min": tzOffsetMin,
		"ts_utc": ts, "session_id": sessionID, "sample_index": sample.SampleIndex, "kind": kind,
		"channels": sample.Channels, "metadata": sample.Metadata, "source": wearableV3Source, "synced_at": now,
	}
	if _, err := s.db.Collection("raw_sensor_samples").InsertOne(cctx, doc); err != nil {
		_, _ = s.db.Collection("raw_sensor_sample_keys").DeleteOne(context.Background(), key)
		return 0, err
	}
	return 1, nil
}

func (s *Store) upsertV3SyncMetadata(ctx context.Context, deviceID, deviceUID, syncID, date string, counters WearableSyncUpsertCounters) error {
	now := time.Now().UTC()
	coverage := bson.M{
		"daily_activity":   counters.DailyActivity > 0,
		"activity_buckets": counters.ActivityBuckets,
		"measurements":     counters.Measurements,
		"sleep_sessions":   counters.SleepSessions,
		"sleep_segments":   counters.SleepSegments,
		"targets":          counters.Targets,
		"workouts":         counters.Workouts,
		"events":           counters.Events,
		"raw_samples":      counters.RawSamples,
	}
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := s.db.Collection("wearable_syncs").UpdateOne(
		cctx,
		bson.M{"device_uid": deviceUID, "sync_date": date},
		bson.M{
			"$set": bson.M{
				"device_id": deviceID, "device_uid": deviceUID, "sync_date": date,
				"last_sync_id": syncID, "coverage": coverage, "source": wearableV3Source,
				"received_at": now, "updated_at": now,
			},
			"$setOnInsert": bson.M{"_id": primitive.NewObjectID(), "created_at": now},
		},
		options.Update().SetUpsert(true),
	)
	return err
}
