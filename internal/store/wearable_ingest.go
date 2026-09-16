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

func (s *Store) upsertWearableDevice(ctx context.Context, deviceUID string, meta WearableDeviceMetadata, state *WearableDeviceState) (*wearableV3DeviceDoc, time.Time, error) {
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	now := time.Now().UTC()
	lastSeen := now
	if state != nil && strings.TrimSpace(state.LastSeenAt) != "" {
		parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(state.LastSeenAt))
		if err != nil {
			return nil, time.Time{}, wearablePayloadError("invalid deviceState.lastSeenAt")
		}
		lastSeen = parsed.UTC()
	}

	set := bson.M{"updated_at": now, "last_seen_at": lastSeen}
	vendor := strings.TrimSpace(meta.Vendor)
	if vendor != "" {
		set["vendor"] = vendor
	}
	if model := strings.TrimSpace(meta.Model); model != "" {
		set["model"] = model
	}
	if value := strings.TrimSpace(meta.HWRev); value != "" {
		set["hw_rev"] = value
	}
	if value := strings.TrimSpace(meta.FWRev); value != "" {
		set["fw_rev"] = value
	}
	if value := strings.TrimSpace(meta.SerialNumber); value != "" {
		set["serial_number"] = value
	}
	if value := strings.TrimSpace(meta.SDKVersion); value != "" {
		set["sdk_version"] = value
	}
	if meta.Capabilities != nil {
		set["capabilities"] = meta.Capabilities
	}
	if state != nil {
		if state.BatteryPercent != nil {
			set["battery_percent"] = *state.BatteryPercent
		}
		if state.Charging != nil {
			set["charging"] = *state.Charging
		}
		set["state_updated_at"] = lastSeen
	}

	// A field may not appear in both $set and $setOnInsert in the same MongoDB
	// update. Put the default vendor in $setOnInsert only when the caller did not
	// explicitly supply a vendor in $set.
	setOnInsert := bson.M{
		"_id":           primitive.NewObjectID(),
		"device_uid":    deviceUID,
		"first_seen_at": lastSeen,
		"created_at":    now,
	}
	if vendor == "" {
		setOnInsert["vendor"] = "QRing"
	}

	var doc wearableV3DeviceDoc
	err := s.db.Collection("devices").FindOneAndUpdate(
		cctx,
		bson.M{"device_uid": deviceUID},
		bson.M{"$set": set, "$setOnInsert": setOnInsert},
		options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After),
	).Decode(&doc)
	if err != nil {
		return nil, time.Time{}, err
	}
	return &doc, lastSeen, nil
}

func (s *Store) upsertV3DailyActivity(ctx context.Context, deviceID, deviceDate string, tzOffsetMin int, activity WearableActivitySummary) error {
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	now := time.Now().UTC()
	set := bson.M{
		"device_id":     deviceID,
		"device_date":   deviceDate,
		"tz_offset_min": tzOffsetMin,
		"source":        firstNonEmpty(strings.TrimSpace(activity.Source), wearableV3Source),
		"synced_at":     now,
	}
	if activity.TotalSteps != nil {
		set["total_steps"] = *activity.TotalSteps
	}
	if activity.RunningSteps != nil {
		set["running_steps"] = *activity.RunningSteps
	}
	if activity.CaloriesRaw != nil {
		set["calories"] = *activity.CaloriesRaw // legacy field kept as SDK raw value
		set["calories_raw"] = *activity.CaloriesRaw
	}
	if activity.CaloriesKcal != nil {
		set["calories_kcal"] = *activity.CaloriesKcal
	}
	if activity.DistanceM != nil {
		set["walk_distance_m"] = *activity.DistanceM
	}
	if activity.SportDurationS != nil {
		set["sport_duration_s"] = *activity.SportDurationS
	}
	if activity.SleepDurationS != nil {
		set["sleep_duration_s"] = *activity.SleepDurationS
	}

	_, err := s.db.Collection("daily_activity").UpdateOne(
		cctx,
		bson.M{"device_id": deviceID, "device_date": deviceDate},
		bson.M{"$set": set, "$setOnInsert": bson.M{"_id": primitive.NewObjectID(), "created_at": now}},
		options.Update().SetUpsert(true),
	)
	return err
}

func (s *Store) ingestV3ActivityBucket(ctx context.Context, deviceID, deviceDate string, tzOffsetMin int, bucket WearableActivityBucket) (int, error) {
	if bucket.TimeIndex == nil || *bucket.TimeIndex < 0 || *bucket.TimeIndex > 95 {
		return 0, wearablePayloadError("timeIndex must be 0..95")
	}
	start, err := parseDeviceDayStart(deviceDate, tzOffsetMin)
	if err != nil {
		return 0, err
	}
	tsUTC := start.Add(time.Duration(*bucket.TimeIndex*15) * time.Minute)
	now := time.Now().UTC()

	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	keyFilter := bson.M{"device_id": deviceID, "ts_utc": tsUTC}
	if _, err := s.db.Collection("steps_15m_keys").InsertOne(cctx, bson.M{"device_id": deviceID, "ts_utc": tsUTC, "created_at": now}); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return 0, nil
		}
		return 0, err
	}

	doc := bson.M{
		"device_id":        deviceID,
		"ts_utc":           tsUTC,
		"device_date":      deviceDate,
		"time_index":       *bucket.TimeIndex,
		"tz_offset_min":    tzOffsetMin,
		"walk_steps":       bucket.WalkSteps,
		"run_steps":        bucket.RunSteps,
		"calories":         bucket.CaloriesRaw,
		"calories_raw":     bucket.CaloriesRaw,
		"distance_m":       bucket.DistanceM,
		"sport_duration_s": bucket.SportDurationS,
		"source":           firstNonEmpty(strings.TrimSpace(bucket.Source), wearableV3Source),
		"synced_at":        now,
	}
	if bucket.CaloriesKcal != nil {
		doc["calories_kcal"] = *bucket.CaloriesKcal
	}
	if _, err := s.db.Collection("steps_15m").InsertOne(cctx, doc); err != nil {
		_, _ = s.db.Collection("steps_15m_keys").DeleteOne(context.Background(), keyFilter)
		return 0, err
	}
	return 1, nil
}

func (s *Store) ingestV3Measurement(ctx context.Context, deviceID, deviceDate string, tzOffsetMin int, measurement WearableMeasurement, ts time.Time) (int, error) {
	metric := sanitizeName(measurement.Metric)
	if metric == "" {
		return 0, wearablePayloadError("measurement.metric is required")
	}
	if measurement.Value == nil && len(measurement.Values) == 0 && measurement.RawValue == nil && len(measurement.RawValues) == 0 {
		return 0, wearablePayloadError("measurement requires value, values, rawValue, or rawValues")
	}
	mode := sanitizeName(firstNonEmpty(measurement.MeasurementMode, "history_sync"))
	sessionID := strings.TrimSpace(measurement.SessionID)
	source := sanitizeName(firstNonEmpty(measurement.Source, wearableV3Source))
	ts = ts.UTC()
	now := time.Now().UTC()

	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	keyDoc := bson.M{
		"device_id":        deviceID,
		"metric":           metric,
		"ts_utc":           ts,
		"measurement_mode": mode,
		"session_id":       sessionID,
		"source":           source,
		"created_at":       now,
	}
	if _, err := s.db.Collection("health_metric_v3_keys").InsertOne(cctx, keyDoc); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return 0, nil
		}
		return 0, err
	}

	doc := bson.M{
		"device_id":        deviceID,
		"device_date":      deviceDate,
		"tz_offset_min":    tzOffsetMin,
		"ts_utc":           ts,
		"metric":           metric,
		"unit":             strings.TrimSpace(measurement.Unit),
		"measurement_mode": mode,
		"session_id":       sessionID,
		"source":           source,
		"synced_at":        now,
	}
	if measurement.Value != nil {
		doc["value"] = *measurement.Value
	}
	if len(measurement.Values) > 0 {
		doc["values"] = measurement.Values
	}
	if measurement.RawValue != nil {
		doc["raw_value"] = *measurement.RawValue
	}
	if len(measurement.RawValues) > 0 {
		doc["raw_values"] = measurement.RawValues
	}
	if measurement.SampleIntervalS != nil {
		doc["sample_interval_s"] = *measurement.SampleIntervalS
	}
	if measurement.ErrorCode != nil {
		doc["error_code"] = *measurement.ErrorCode
	}
	if measurement.Derived != nil {
		doc["derived"] = *measurement.Derived
	}
	if value := strings.TrimSpace(measurement.Algorithm); value != "" {
		doc["algorithm"] = value
	}
	if measurement.Metadata != nil {
		doc["metadata"] = measurement.Metadata
	}
	if _, err := s.db.Collection("health_metrics").InsertOne(cctx, doc); err != nil {
		_, _ = s.db.Collection("health_metric_v3_keys").DeleteOne(context.Background(), bson.M{
			"device_id": deviceID, "metric": metric, "ts_utc": ts, "measurement_mode": mode, "session_id": sessionID, "source": source,
		})
		return 0, err
	}
	return 1, nil
}
