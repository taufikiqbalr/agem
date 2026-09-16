package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Store struct {
	db             *mongo.Database
	userDB         *mongo.Database
	userCollection string
	externalUserDB bool
}

func New(db *mongo.Database) *Store {
	return NewWithUserSource(db, nil, "")
}

func NewWithUserSource(db *mongo.Database, userDB *mongo.Database, userCollection string) *Store {
	if userCollection == "" {
		userCollection = "users"
	}
	st := &Store{
		db:             db,
		userDB:         db,
		userCollection: userCollection,
	}
	if userDB != nil {
		st.userDB = userDB
		st.externalUserDB = true
	}
	return st
}

func (s *Store) DB() *mongo.Database { return s.db }

func (s *Store) usersCollection() *mongo.Collection {
	db := s.db
	if s.userDB != nil {
		db = s.userDB
	}
	return db.Collection(s.userCollection)
}

var ErrNotFound = errors.New("not found")

const (
	secondsPerDay            int64 = 24 * 60 * 60
	rawRetentionSeconds            = 365 * secondsPerDay
	eventRetentionSeconds          = 90 * secondsPerDay
	keyRetentionGraceSeconds       = 7 * secondsPerDay
	rawKeyRetentionSeconds         = rawRetentionSeconds + keyRetentionGraceSeconds
	eventKeyRetentionSeconds       = eventRetentionSeconds + keyRetentionGraceSeconds
)

func (s *Store) EnsureIndexes(ctx context.Context) error {
	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	for _, name := range []string{
		"users",
		"devices",
		"user_devices",
		"daily_activity",
		"sleep_summary",
		"wearable_syncs",
		"total_activities",
		"workouts",
		"steps_15m_keys",
		"sleep_segments_keys",
		"health_metric_keys",
		"device_event_keys",
	} {
		if err := s.ensureRegularCollection(cctx, name); err != nil {
			return err
		}
	}

	if err := s.ensureTimeSeriesCollection(cctx, "steps_15m", "ts_utc", "device_id", "minutes", rawRetentionSeconds); err != nil {
		return err
	}
	if err := s.ensureTimeSeriesCollection(cctx, "sleep_segments", "start_utc", "device_id", "minutes", rawRetentionSeconds); err != nil {
		return err
	}
	if err := s.ensureTimeSeriesCollection(cctx, "health_metrics", "ts_utc", "device_id", "seconds", rawRetentionSeconds); err != nil {
		return err
	}
	if err := s.ensureTimeSeriesCollection(cctx, "device_events", "ts_utc", "device_id", "seconds", eventRetentionSeconds); err != nil {
		return err
	}

	indexes := map[string][]mongo.IndexModel{
		"devices": {
			{
				Keys:    bson.D{{Key: "device_uid", Value: 1}},
				Options: options.Index().SetUnique(true).SetName("uniq_device_uid"),
			},
		},
		"user_devices": {
			{
				Keys:    bson.D{{Key: "user_id", Value: 1}, {Key: "is_primary", Value: -1}, {Key: "paired_at", Value: -1}},
				Options: options.Index().SetName("idx_user_devices_user_primary_paired"),
			},
		},
		"steps_15m": {
			{
				Keys:    bson.D{{Key: "device_id", Value: 1}, {Key: "device_date", Value: 1}, {Key: "time_index", Value: 1}},
				Options: options.Index().SetName("idx_steps_device_date_time"),
			},
		},
		"steps_15m_keys": {
			{
				Keys:    bson.D{{Key: "device_id", Value: 1}, {Key: "ts_utc", Value: 1}},
				Options: options.Index().SetUnique(true).SetName("uniq_steps_device_ts"),
			},
			{
				Keys:    bson.D{{Key: "created_at", Value: 1}},
				Options: ttlIndexOptions("ttl_steps_15m_keys_created_at", rawKeyRetentionSeconds),
			},
		},
		"daily_activity": {
			{
				Keys:    bson.D{{Key: "device_id", Value: 1}, {Key: "device_date", Value: 1}},
				Options: options.Index().SetUnique(true).SetName("uniq_daily_activity_device_date"),
			},
		},
		"wearable_syncs": {
			{
				Keys:    bson.D{{Key: "device_uid", Value: 1}, {Key: "sync_date", Value: 1}},
				Options: options.Index().SetUnique(true).SetName("uniq_wearable_sync_device_uid_date"),
			},
			{
				Keys:    bson.D{{Key: "device_id", Value: 1}, {Key: "sync_date", Value: 1}},
				Options: options.Index().SetName("idx_wearable_sync_device_date"),
			},
		},
		"total_activities": {
			{
				Keys:    bson.D{{Key: "device_id", Value: 1}, {Key: "device_date", Value: 1}, {Key: "kind", Value: 1}},
				Options: options.Index().SetUnique(true).SetName("uniq_total_activities_device_date_kind"),
			},
			{
				Keys:    bson.D{{Key: "device_uid", Value: 1}, {Key: "device_date", Value: 1}},
				Options: options.Index().SetName("idx_total_activities_device_uid_date"),
			},
		},
		"sleep_summary": {
			{
				Keys:    bson.D{{Key: "device_id", Value: 1}, {Key: "sleep_date", Value: 1}},
				Options: options.Index().SetUnique(true).SetName("uniq_sleep_summary_device_date"),
			},
		},
		"sleep_segments": {
			{
				Keys:    bson.D{{Key: "device_id", Value: 1}, {Key: "sleep_date", Value: 1}, {Key: "start_utc", Value: 1}},
				Options: options.Index().SetName("idx_sleep_segments_device_date_start"),
			},
		},
		"sleep_segments_keys": {
			{
				Keys:    bson.D{{Key: "device_id", Value: 1}, {Key: "start_utc", Value: 1}},
				Options: options.Index().SetUnique(true).SetName("uniq_sleep_segments_device_start"),
			},
			{
				Keys:    bson.D{{Key: "created_at", Value: 1}},
				Options: ttlIndexOptions("ttl_sleep_segments_keys_created_at", rawKeyRetentionSeconds),
			},
		},
		"workouts": {
			{
				Keys:    bson.D{{Key: "device_id", Value: 1}, {Key: "start_utc", Value: 1}},
				Options: options.Index().SetUnique(true).SetName("uniq_workouts_device_start"),
			},
		},
		"health_metrics": {
			{
				Keys:    bson.D{{Key: "device_id", Value: 1}, {Key: "metric", Value: 1}, {Key: "ts_utc", Value: 1}},
				Options: options.Index().SetName("idx_health_metrics_device_metric_ts"),
			},
		},
		"health_metric_keys": {
			{
				Keys:    bson.D{{Key: "device_id", Value: 1}, {Key: "metric", Value: 1}, {Key: "ts_utc", Value: 1}, {Key: "source", Value: 1}},
				Options: options.Index().SetUnique(true).SetName("uniq_health_metric_device_metric_ts_source"),
			},
			{
				Keys:    bson.D{{Key: "created_at", Value: 1}},
				Options: ttlIndexOptions("ttl_health_metric_keys_created_at", rawKeyRetentionSeconds),
			},
		},
		"device_events": {
			{
				Keys:    bson.D{{Key: "device_id", Value: 1}, {Key: "event_type", Value: 1}, {Key: "ts_utc", Value: 1}},
				Options: options.Index().SetName("idx_device_events_device_type_ts"),
			},
		},
		"device_event_keys": {
			{
				Keys:    bson.D{{Key: "device_id", Value: 1}, {Key: "event_type", Value: 1}, {Key: "ts_utc", Value: 1}, {Key: "source", Value: 1}},
				Options: options.Index().SetUnique(true).SetName("uniq_device_event_device_type_ts_source"),
			},
			{
				Keys:    bson.D{{Key: "created_at", Value: 1}},
				Options: ttlIndexOptions("ttl_device_event_keys_created_at", eventKeyRetentionSeconds),
			},
		},
	}

	for collection, models := range indexes {
		if _, err := s.db.Collection(collection).Indexes().CreateMany(cctx, models); err != nil {
			if isIndexExistsConflict(err) {
				continue
			}
			return err
		}
	}
	return nil
}

func (s *Store) ensureRegularCollection(ctx context.Context, name string) error {
	specs, err := s.db.ListCollectionSpecifications(ctx, bson.M{"name": name})
	if err != nil {
		return err
	}
	if len(specs) == 0 {
		if err := s.db.CreateCollection(ctx, name); err != nil && !isNamespaceExists(err) {
			return err
		}
		return nil
	}

	var rawOptions struct {
		TimeSeries      *timeSeriesSpec `bson:"timeseries"`
		TimeSeriesCamel *timeSeriesSpec `bson:"timeSeries"`
	}
	if err := bson.Unmarshal(specs[0].Options, &rawOptions); err != nil {
		return err
	}
	if rawOptions.TimeSeries != nil || rawOptions.TimeSeriesCamel != nil {
		return fmt.Errorf("%s exists as a MongoDB time series collection; expected regular collection", name)
	}
	return nil
}

func (s *Store) ensureTimeSeriesCollection(ctx context.Context, name, timeField, metaField, granularity string, expireAfterSeconds int64) error {
	specs, err := s.db.ListCollectionSpecifications(ctx, bson.M{"name": name})
	if err != nil {
		return err
	}
	if len(specs) == 0 {
		opts := options.CreateCollection().SetTimeSeriesOptions(
			options.TimeSeries().
				SetTimeField(timeField).
				SetMetaField(metaField).
				SetGranularity(granularity),
		)
		if expireAfterSeconds > 0 {
			opts.SetExpireAfterSeconds(expireAfterSeconds)
		}
		if err := s.db.CreateCollection(ctx, name, opts); err != nil && !isNamespaceExists(err) {
			return err
		}
		return nil
	}

	var rawOptions struct {
		TimeSeries      *timeSeriesSpec `bson:"timeseries"`
		TimeSeriesCamel *timeSeriesSpec `bson:"timeSeries"`
	}
	if err := bson.Unmarshal(specs[0].Options, &rawOptions); err != nil {
		return err
	}
	timeSeries := rawOptions.TimeSeries
	if timeSeries == nil {
		timeSeries = rawOptions.TimeSeriesCamel
	}
	if timeSeries == nil {
		return fmt.Errorf("%s exists as a regular collection; recreate or migrate it as a MongoDB time series collection", name)
	}
	if timeSeries.TimeField != timeField || timeSeries.MetaField != metaField {
		return fmt.Errorf("%s time series options mismatch: expected timeField=%s metaField=%s, got timeField=%s metaField=%s", name, timeField, metaField, timeSeries.TimeField, timeSeries.MetaField)
	}
	if granularity != "" && timeSeries.Granularity != "" && timeSeries.Granularity != granularity {
		return fmt.Errorf("%s time series granularity mismatch: expected %s, got %s", name, granularity, timeSeries.Granularity)
	}
	return nil
}

type timeSeriesSpec struct {
	TimeField   string `bson:"timeField"`
	MetaField   string `bson:"metaField"`
	Granularity string `bson:"granularity"`
}

func ttlIndexOptions(name string, expireAfterSeconds int64) *options.IndexOptions {
	return options.Index().
		SetName(name).
		SetExpireAfterSeconds(int32(expireAfterSeconds))
}

func isNamespaceExists(err error) bool {
	var cmdErr mongo.CommandError
	return errors.As(err, &cmdErr) && cmdErr.Code == 48
}

func isIndexExistsConflict(err error) bool {
	messageHasExistingIndex := func(message string) bool {
		message = strings.ToLower(message)
		return strings.Contains(message, "already exists") || strings.Contains(message, "equivalent index")
	}

	var cmdErr mongo.CommandError
	if errors.As(err, &cmdErr) && (cmdErr.Code == 85 || cmdErr.Code == 86) {
		return messageHasExistingIndex(cmdErr.Message)
	}

	var writeErr mongo.WriteException
	if errors.As(err, &writeErr) {
		for _, writeError := range writeErr.WriteErrors {
			if (writeError.Code == 85 || writeError.Code == 86) && messageHasExistingIndex(writeError.Message) {
				return true
			}
		}
	}
	return false
}

func ctxTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, 5*time.Second)
}

func objectIDFromString(id string) (primitive.ObjectID, error) {
	oid, err := primitive.ObjectIDFromHex(strings.TrimSpace(id))
	if err != nil {
		return primitive.NilObjectID, ErrNotFound
	}
	return oid, nil
}

func objectIDString(id primitive.ObjectID) string {
	if id.IsZero() {
		return ""
	}
	return id.Hex()
}

func isNotFound(err error) bool {
	return errors.Is(err, mongo.ErrNoDocuments) || errors.Is(err, ErrNotFound)
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
