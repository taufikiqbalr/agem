package store

import (
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"sort"
	"time"
)

func (s *Store) GetWearableRangeByUIDV3(ctx context.Context, deviceUID, fromDate, toDate string, includeRaw bool) (*WearableRangeResponse, error) {
	device, err := s.getWearableDeviceDocByUID(ctx, deviceUID)
	if err != nil {
		return nil, err
	}
	return s.getWearableRangeByDeviceV3(ctx, *device, fromDate, toDate, includeRaw)
}

func (s *Store) GetWearableRangeByDeviceIDV3(ctx context.Context, deviceID, fromDate, toDate string, includeRaw bool) (*WearableRangeResponse, error) {
	device, err := s.getWearableDeviceDocByIDString(ctx, deviceID)
	if err != nil {
		return nil, err
	}
	return s.getWearableRangeByDeviceV3(ctx, *device, fromDate, toDate, includeRaw)
}

func (s *Store) GetUserWearableRangeV3(ctx context.Context, userID, fromDate, toDate string, includeRaw bool) (*UserWearableRangeResponse, error) {
	pairs, err := s.ListWearableDevicesForUserV3(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := &UserWearableRangeResponse{UserID: userID, From: fromDate, To: toDate, Devices: make([]UserWearableRangeDevice, 0, len(pairs))}
	for _, pair := range pairs {
		rangeResp, err := s.GetWearableRangeByDeviceIDV3(ctx, pair.DeviceID, fromDate, toDate, includeRaw)
		if err != nil {
			return nil, err
		}
		out.Devices = append(out.Devices, UserWearableRangeDevice{Pairing: pair, Range: *rangeResp})
	}
	return out, nil
}

func (s *Store) getWearableRangeByDeviceV3(ctx context.Context, device wearableV3DeviceDoc, fromDate, toDate string, includeRaw bool) (*WearableRangeResponse, error) {
	if _, err := time.Parse(wearableDateLayout, fromDate); err != nil {
		return nil, wearablePayloadError("invalid from date")
	}
	if _, err := time.Parse(wearableDateLayout, toDate); err != nil {
		return nil, wearablePayloadError("invalid to date")
	}
	if fromDate > toDate {
		return nil, wearablePayloadError("from must be before or equal to to")
	}
	deviceID := device.ID.Hex()
	days := map[string]map[string]any{}
	getDay := func(date string) map[string]any {
		day, ok := days[date]
		if !ok {
			day = map[string]any{"date": date}
			days[date] = day
		}
		return day
	}

	activityDocs, err := s.findV3DocsByDate(ctx, "daily_activity", deviceID, fromDate, toDate, bson.D{{Key: "device_date", Value: 1}})
	if err != nil {
		return nil, err
	}
	for _, doc := range activityDocs {
		getDay(asString(doc["device_date"]))["activity"] = normalizeBSONDocument(doc)
	}
	bucketDocs, err := s.findV3DocsByDate(ctx, "steps_15m", deviceID, fromDate, toDate, bson.D{{Key: "device_date", Value: 1}, {Key: "time_index", Value: 1}})
	if err != nil {
		return nil, err
	}
	appendDocsToDays(bucketDocs, getDay, "activityBuckets")
	measurementDocs, err := s.findV3DocsByDate(ctx, "health_metrics", deviceID, fromDate, toDate, bson.D{{Key: "device_date", Value: 1}, {Key: "ts_utc", Value: 1}, {Key: "metric", Value: 1}})
	if err != nil {
		return nil, err
	}
	appendDocsToDays(measurementDocs, getDay, "measurements")
	summaryDocs, err := s.findV3DocsByDate(ctx, "sleep_summary", deviceID, fromDate, toDate, bson.D{{Key: "sleep_date", Value: 1}})
	if err != nil {
		return nil, err
	}
	for _, doc := range summaryDocs {
		date := asString(doc["sleep_date"])
		getDay(date)["sleepSummary"] = normalizeBSONDocument(doc)
	}
	segmentDocs, err := s.findV3DocsByDate(ctx, "sleep_segments", deviceID, fromDate, toDate, bson.D{{Key: "sleep_date", Value: 1}, {Key: "start_utc", Value: 1}})
	if err != nil {
		return nil, err
	}
	segmentsBySession := map[string][]any{}
	for _, doc := range segmentDocs {
		sid := asString(doc["session_id"])
		if sid == "" {
			continue
		}
		segmentsBySession[sid] = append(segmentsBySession[sid], normalizeBSONDocument(doc))
	}
	sessionDocs, err := s.findV3DocsByDate(ctx, "sleep_sessions", deviceID, fromDate, toDate, bson.D{{Key: "sleep_date", Value: 1}, {Key: "start_utc", Value: 1}})
	if err != nil {
		return nil, err
	}
	for _, doc := range sessionDocs {
		date := asString(doc["sleep_date"])
		normalized := normalizeBSONDocument(doc)
		sid := asString(doc["session_id"])
		if segments := segmentsBySession[sid]; len(segments) > 0 {
			normalized["segments"] = segments
		}
		appendDayItem(getDay(date), "sleepSessions", normalized)
	}
	targetDocs, err := s.findV3DocsByDate(ctx, "total_activities", deviceID, fromDate, toDate, bson.D{{Key: "device_date", Value: 1}, {Key: "index", Value: 1}})
	if err != nil {
		return nil, err
	}
	appendDocsToDays(targetDocs, getDay, "targets")
	workoutDocs, err := s.findV3DocsByDate(ctx, "workouts", deviceID, fromDate, toDate, bson.D{{Key: "device_date", Value: 1}, {Key: "start_utc", Value: 1}})
	if err != nil {
		return nil, err
	}
	appendDocsToDays(workoutDocs, getDay, "workouts")
	eventDocs, err := s.findV3DocsByDate(ctx, "device_events", deviceID, fromDate, toDate, bson.D{{Key: "device_date", Value: 1}, {Key: "ts_utc", Value: 1}})
	if err != nil {
		return nil, err
	}
	appendDocsToDays(eventDocs, getDay, "events")
	if includeRaw {
		rawDocs, err := s.findV3DocsByDate(ctx, "raw_sensor_samples", deviceID, fromDate, toDate, bson.D{{Key: "device_date", Value: 1}, {Key: "ts_utc", Value: 1}, {Key: "sample_index", Value: 1}})
		if err != nil {
			return nil, err
		}
		appendDocsToDays(rawDocs, getDay, "rawSamples")
	}

	keys := make([]string, 0, len(days))
	for key := range days {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	dayList := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		dayList = append(dayList, days[key])
	}
	return &WearableRangeResponse{Device: wearableV3DeviceDocToView(device), From: fromDate, To: toDate, Days: dayList}, nil
}

func (s *Store) findV3DocsByDate(ctx context.Context, collection, deviceID, fromDate, toDate string, sortSpec bson.D) ([]bson.M, error) {
	dateField := "device_date"
	if collection == "sleep_summary" || collection == "sleep_sessions" || collection == "sleep_segments" {
		dateField = "sleep_date"
	}
	cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cursor, err := s.db.Collection(collection).Find(
		cctx,
		bson.M{"device_id": deviceID, dateField: bson.M{"$gte": fromDate, "$lte": toDate}},
		options.Find().SetSort(sortSpec),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(cctx)
	out := make([]bson.M, 0)
	for cursor.Next(cctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		out = append(out, doc)
	}
	return out, cursor.Err()
}
