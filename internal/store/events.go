package store

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type DeviceEventRow struct {
	DeviceID  string
	TsUTC     time.Time
	EventType string
	Source    string
	Payload   map[string]any
}

type DeviceEventResponse struct {
	DeviceID  string         `json:"device_id"`
	TsUTC     string         `json:"ts_utc"`
	EventType string         `json:"event_type"`
	Source    string         `json:"source"`
	Payload   map[string]any `json:"payload,omitempty"`
	SyncedAt  string         `json:"synced_at"`
}

type deviceEventDoc struct {
	DeviceID  string         `bson:"device_id"`
	TsUTC     time.Time      `bson:"ts_utc"`
	EventType string         `bson:"event_type"`
	Source    string         `bson:"source"`
	Payload   map[string]any `bson:"payload,omitempty"`
	SyncedAt  time.Time      `bson:"synced_at"`
}

func (s *Store) IngestDeviceEventsBulk(ctx context.Context, events []DeviceEventRow) (int, error) {
	if len(events) == 0 {
		return 0, nil
	}

	cctx, cancel := ctxTimeout(ctx)
	defer cancel()

	now := time.Now().UTC()
	eventCol := s.db.Collection("device_events")
	keyCol := s.db.Collection("device_event_keys")
	inserted := 0

	for _, event := range events {
		tsUTC := event.TsUTC.UTC()
		src := event.Source
		if src == "" {
			src = "device"
		}

		keyFilter := bson.M{"device_id": event.DeviceID, "event_type": event.EventType, "ts_utc": tsUTC, "source": src}
		if _, err := keyCol.InsertOne(cctx, bson.M{
			"device_id":  event.DeviceID,
			"event_type": event.EventType,
			"ts_utc":     tsUTC,
			"source":     src,
			"created_at": now,
		}); err != nil {
			if mongo.IsDuplicateKeyError(err) {
				continue
			}
			return inserted, err
		}

		doc := deviceEventDoc{
			DeviceID:  event.DeviceID,
			TsUTC:     tsUTC,
			EventType: event.EventType,
			Source:    src,
			Payload:   event.Payload,
			SyncedAt:  now,
		}
		if _, err := eventCol.InsertOne(cctx, doc); err != nil {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 2*time.Second)
			_, _ = keyCol.DeleteOne(cleanupCtx, keyFilter)
			cleanupCancel()
			return inserted, err
		}
		inserted++
	}

	return inserted, nil
}

func (s *Store) ListDeviceEvents(ctx context.Context, deviceID, eventType string, from, to time.Time) ([]DeviceEventResponse, error) {
	cctx, cancel := ctxTimeout(ctx)
	defer cancel()

	filter := bson.M{
		"device_id": deviceID,
		"ts_utc": bson.M{
			"$gte": from.UTC(),
			"$lt":  to.UTC(),
		},
	}
	if eventType != "" {
		filter["event_type"] = eventType
	}

	cursor, err := s.db.Collection("device_events").Find(
		cctx,
		filter,
		options.Find().SetSort(bson.D{{Key: "ts_utc", Value: 1}, {Key: "event_type", Value: 1}}),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(cctx)

	var out []DeviceEventResponse
	for cursor.Next(cctx) {
		var doc deviceEventDoc
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		out = append(out, deviceEventDocToResponse(doc))
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func deviceEventDocToResponse(doc deviceEventDoc) DeviceEventResponse {
	return DeviceEventResponse{
		DeviceID:  doc.DeviceID,
		TsUTC:     doc.TsUTC.Format(time.RFC3339),
		EventType: doc.EventType,
		Source:    doc.Source,
		Payload:   doc.Payload,
		SyncedAt:  doc.SyncedAt.Format(time.RFC3339),
	}
}
