package store

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"
)

func (s *Store) EnsureSDKV3Schema(ctx context.Context) error {
	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	for _, name := range []string{
		"sleep_sessions",
		"health_metric_v3_keys",
		"sleep_segment_v3_keys",
		"device_event_v3_keys",
		"raw_sensor_sample_keys",
	} {
		if err := s.ensureRegularCollection(cctx, name); err != nil {
			return err
		}
	}
	if err := s.ensureTimeSeriesCollection(cctx, "raw_sensor_samples", "ts_utc", "device_id", "seconds", rawSensorRetentionSeconds); err != nil {
		return err
	}

	// MongoDB partial indexes support equality and $exists:true, but not
	// $exists:false. Backfill an explicit active flag before creating the
	// ownership indexes so both legacy and v3 pairing rows are constrained.
	pairings := s.db.Collection("user_devices")
	if _, err := pairings.UpdateMany(cctx, bson.M{"active": bson.M{"$exists": false}, "unpaired_at": bson.M{"$exists": false}}, bson.M{"$set": bson.M{"active": true}}); err != nil {
		return fmt.Errorf("backfill active pairings: %w", err)
	}
	if _, err := pairings.UpdateMany(cctx, bson.M{"active": bson.M{"$exists": false}, "unpaired_at": bson.M{"$exists": true}}, bson.M{"$set": bson.M{"active": false, "is_primary": false}}); err != nil {
		return fmt.Errorf("backfill inactive pairings: %w", err)
	}

	activePairFilter := bson.M{"active": true}
	primaryPairFilter := bson.M{"active": true, "is_primary": true}

	indexes := map[string][]mongo.IndexModel{
		"devices": {
			{Keys: bson.D{{Key: "last_seen_at", Value: -1}}, Options: options.Index().SetName("idx_devices_last_seen_v3")},
		},
		"user_devices": {
			{Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "device_id", Value: 1}}, Options: options.Index().SetUnique(true).SetName("uniq_user_active_device_v3").SetPartialFilterExpression(activePairFilter)},
			{Keys: bson.D{{Key: "device_id", Value: 1}}, Options: options.Index().SetUnique(true).SetName("uniq_device_active_owner_v3").SetPartialFilterExpression(activePairFilter)},
			{Keys: bson.D{{Key: "user_id", Value: 1}}, Options: options.Index().SetUnique(true).SetName("uniq_user_primary_active_v3").SetPartialFilterExpression(primaryPairFilter)},
		},
		"health_metrics": {
			{Keys: bson.D{{Key: "device_id", Value: 1}, {Key: "device_date", Value: 1}, {Key: "metric", Value: 1}, {Key: "ts_utc", Value: 1}}, Options: options.Index().SetName("idx_health_metrics_v3_device_date_metric_ts")},
		},
		"health_metric_v3_keys": {
			{Keys: bson.D{{Key: "device_id", Value: 1}, {Key: "metric", Value: 1}, {Key: "ts_utc", Value: 1}, {Key: "measurement_mode", Value: 1}, {Key: "session_id", Value: 1}, {Key: "source", Value: 1}}, Options: options.Index().SetUnique(true).SetName("uniq_health_metric_v3")},
			{Keys: bson.D{{Key: "created_at", Value: 1}}, Options: ttlIndexOptions("ttl_health_metric_v3_keys_created_at", rawKeyRetentionSeconds)},
		},
		"sleep_sessions": {
			{Keys: bson.D{{Key: "device_id", Value: 1}, {Key: "session_id", Value: 1}}, Options: options.Index().SetUnique(true).SetName("uniq_sleep_session_v3")},
			{Keys: bson.D{{Key: "device_id", Value: 1}, {Key: "sleep_date", Value: 1}, {Key: "start_utc", Value: 1}}, Options: options.Index().SetName("idx_sleep_session_v3_device_date_start")},
		},
		"sleep_segments": {
			{Keys: bson.D{{Key: "device_id", Value: 1}, {Key: "sleep_date", Value: 1}, {Key: "session_id", Value: 1}, {Key: "start_utc", Value: 1}}, Options: options.Index().SetName("idx_sleep_segment_v3_session_start")},
		},
		"sleep_segment_v3_keys": {
			{Keys: bson.D{{Key: "device_id", Value: 1}, {Key: "session_id", Value: 1}, {Key: "start_utc", Value: 1}}, Options: options.Index().SetUnique(true).SetName("uniq_sleep_segment_v3")},
			{Keys: bson.D{{Key: "created_at", Value: 1}}, Options: ttlIndexOptions("ttl_sleep_segment_v3_keys_created_at", rawKeyRetentionSeconds)},
		},
		"workouts": {
			{Keys: bson.D{{Key: "device_id", Value: 1}, {Key: "device_date", Value: 1}, {Key: "start_utc", Value: 1}}, Options: options.Index().SetName("idx_workouts_v3_device_date_start")},
		},
		"device_events": {
			{Keys: bson.D{{Key: "device_id", Value: 1}, {Key: "device_date", Value: 1}, {Key: "event_type", Value: 1}, {Key: "ts_utc", Value: 1}}, Options: options.Index().SetName("idx_device_events_v3_device_date_type_ts")},
		},
		"device_event_v3_keys": {
			{Keys: bson.D{{Key: "device_id", Value: 1}, {Key: "event_type", Value: 1}, {Key: "ts_utc", Value: 1}, {Key: "session_id", Value: 1}, {Key: "source", Value: 1}}, Options: options.Index().SetUnique(true).SetName("uniq_device_event_v3")},
			{Keys: bson.D{{Key: "created_at", Value: 1}}, Options: ttlIndexOptions("ttl_device_event_v3_keys_created_at", eventKeyRetentionSeconds)},
		},
		"raw_sensor_samples": {
			{Keys: bson.D{{Key: "device_id", Value: 1}, {Key: "device_date", Value: 1}, {Key: "session_id", Value: 1}, {Key: "ts_utc", Value: 1}}, Options: options.Index().SetName("idx_raw_sensor_v3_device_date_session_ts")},
		},
		"raw_sensor_sample_keys": {
			{Keys: bson.D{{Key: "device_id", Value: 1}, {Key: "session_id", Value: 1}, {Key: "ts_utc", Value: 1}, {Key: "sample_index", Value: 1}, {Key: "kind", Value: 1}}, Options: options.Index().SetUnique(true).SetName("uniq_raw_sensor_sample_v3")},
			{Keys: bson.D{{Key: "created_at", Value: 1}}, Options: ttlIndexOptions("ttl_raw_sensor_sample_keys_created_at", rawSensorKeyRetentionSecond)},
		},
	}

	for collection, models := range indexes {
		if _, err := s.db.Collection(collection).Indexes().CreateMany(cctx, models); err != nil {
			if isIndexExistsConflict(err) {
				continue
			}
			return fmt.Errorf("v3 indexes %s: %w", collection, err)
		}
	}
	return nil
}
