package store

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"strings"
	"time"
)

func (p WearableSyncRequest) CanonicalDeviceUID() string {
	if uid := strings.TrimSpace(p.DeviceUID); uid != "" {
		return uid
	}
	return strings.TrimSpace(p.DeviceUIDAlias)
}

func (s *Store) SyncWearable(ctx context.Context, req WearableSyncRequest) (*WearableSyncResponse, error) {
	deviceUID := req.CanonicalDeviceUID()
	if deviceUID == "" {
		return nil, wearablePayloadError("deviceUid is required")
	}
	if req.TzOffsetMin == nil {
		return nil, wearablePayloadError("tzOffsetMin is required")
	}
	tzOffsetMin := *req.TzOffsetMin
	if tzOffsetMin < -14*60 || tzOffsetMin > 14*60 {
		return nil, wearablePayloadError("tzOffsetMin must be between -840 and 840")
	}
	if len(req.Days) == 0 && req.DeviceState == nil && isEmptyWearableDeviceMetadata(req.Device) {
		return nil, wearablePayloadError("at least one of device metadata, deviceState, or days is required")
	}
	// Validate state before any device upsert so a rejected request cannot write
	// an invalid current state and then return HTTP 400.
	if req.DeviceState != nil && req.DeviceState.BatteryPercent != nil {
		battery := *req.DeviceState.BatteryPercent
		if battery < 0 || battery > 100 {
			return nil, wearablePayloadError("deviceState.batteryPercent must be 0..100")
		}
	}

	seenDates := make(map[string]struct{}, len(req.Days))
	for _, day := range req.Days {
		if _, err := parseDeviceDayStart(day.Date, tzOffsetMin); err != nil {
			return nil, err
		}
		if _, exists := seenDates[day.Date]; exists {
			return nil, wearablePayloadError("duplicate day date " + day.Date)
		}
		seenDates[day.Date] = struct{}{}
	}

	syncID := primitive.NewObjectID().Hex()
	device, stateTime, err := s.upsertWearableDevice(ctx, deviceUID, req.Device, req.DeviceState)
	if err != nil {
		return nil, err
	}

	response := &WearableSyncResponse{
		Status:   "synced",
		SyncID:   syncID,
		Device:   wearableV3DeviceDocToView(*device),
		Days:     make([]WearableDaySyncResult, 0, len(req.Days)),
		Upserted: WearableSyncUpsertCounters{},
	}

	if req.DeviceState != nil {
		response.Upserted.DeviceState = 1
		if req.DeviceState.BatteryPercent != nil {
			battery := *req.DeviceState.BatteryPercent
			value := float64(battery)
			dayDate := deviceDateForUTC(stateTime, tzOffsetMin)
			inserted, err := s.ingestV3Measurement(ctx, device.ID.Hex(), dayDate, tzOffsetMin, WearableMeasurement{
				Ts:              stateTime.Format(time.RFC3339Nano),
				Metric:          "battery_level",
				Value:           &value,
				Unit:            "%",
				MeasurementMode: "realtime",
				Source:          "device_state",
			}, stateTime)
			if err != nil {
				return nil, err
			}
			response.Upserted.Measurements += inserted
		}
		if req.DeviceState.Charging != nil {
			dayDate := deviceDateForUTC(stateTime, tzOffsetMin)
			inserted, err := s.ingestV3Event(ctx, device.ID.Hex(), dayDate, tzOffsetMin, WearableDeviceEvent{
				Ts:        stateTime.Format(time.RFC3339Nano),
				EventType: "charging_state",
				Source:    "device_state",
				Payload:   map[string]any{"charging": *req.DeviceState.Charging},
			}, stateTime)
			if err != nil {
				return nil, err
			}
			response.Upserted.Events += inserted
		}
	}

	for _, day := range req.Days {
		result := WearableDaySyncResult{Date: day.Date}

		if day.Activity != nil {
			if err := s.upsertV3DailyActivity(ctx, device.ID.Hex(), day.Date, tzOffsetMin, *day.Activity); err != nil {
				return nil, err
			}
			result.Upserted.DailyActivity = 1
		}

		for i, bucket := range day.ActivityBuckets {
			inserted, err := s.ingestV3ActivityBucket(ctx, device.ID.Hex(), day.Date, tzOffsetMin, bucket)
			if err != nil {
				return nil, fmt.Errorf("day %s activityBuckets[%d]: %w", day.Date, i, err)
			}
			result.Upserted.ActivityBuckets += inserted
		}

		for i, measurement := range day.Measurements {
			ts, err := resolveDayTimestamp(day.Date, tzOffsetMin, measurement.Ts, measurement.MinuteOfDay)
			if err != nil {
				return nil, fmt.Errorf("day %s measurements[%d]: %w", day.Date, i, err)
			}
			inserted, err := s.ingestV3Measurement(ctx, device.ID.Hex(), day.Date, tzOffsetMin, measurement, ts)
			if err != nil {
				return nil, fmt.Errorf("day %s measurements[%d]: %w", day.Date, i, err)
			}
			result.Upserted.Measurements += inserted
		}

		for i, session := range day.SleepSessions {
			sessionCount, segmentCount, err := s.upsertV3SleepSession(ctx, device.ID.Hex(), day.Date, tzOffsetMin, session)
			if err != nil {
				return nil, fmt.Errorf("day %s sleepSessions[%d]: %w", day.Date, i, err)
			}
			result.Upserted.SleepSessions += sessionCount
			result.Upserted.SleepSegments += segmentCount
		}
		if len(day.SleepSessions) > 0 {
			if err := s.refreshV3SleepSummary(ctx, device.ID.Hex(), day.Date, tzOffsetMin); err != nil {
				return nil, err
			}
			result.Upserted.SleepSummaries = 1
		}

		for i, target := range day.Targets {
			if err := s.upsertV3Target(ctx, device.ID.Hex(), deviceUID, day.Date, target, i); err != nil {
				return nil, fmt.Errorf("day %s targets[%d]: %w", day.Date, i, err)
			}
			result.Upserted.Targets++
		}

		for i, workout := range day.Workouts {
			if err := s.upsertV3Workout(ctx, device.ID.Hex(), day.Date, tzOffsetMin, workout); err != nil {
				return nil, fmt.Errorf("day %s workouts[%d]: %w", day.Date, i, err)
			}
			result.Upserted.Workouts++
		}

		for i, event := range day.Events {
			ts, err := resolveDayTimestamp(day.Date, tzOffsetMin, event.Ts, event.MinuteOfDay)
			if err != nil {
				return nil, fmt.Errorf("day %s events[%d]: %w", day.Date, i, err)
			}
			inserted, err := s.ingestV3Event(ctx, device.ID.Hex(), day.Date, tzOffsetMin, event, ts)
			if err != nil {
				return nil, fmt.Errorf("day %s events[%d]: %w", day.Date, i, err)
			}
			result.Upserted.Events += inserted
		}

		for i, sample := range day.RawSamples {
			inserted, err := s.ingestV3RawSample(ctx, device.ID.Hex(), day.Date, tzOffsetMin, sample)
			if err != nil {
				return nil, fmt.Errorf("day %s rawSamples[%d]: %w", day.Date, i, err)
			}
			result.Upserted.RawSamples += inserted
		}

		if err := s.upsertV3SyncMetadata(ctx, device.ID.Hex(), deviceUID, syncID, day.Date, tzOffsetMin, result.Upserted); err != nil {
			return nil, err
		}
		result.Upserted.SyncMetadata = 1
		response.Days = append(response.Days, result)
		addWearableCounters(&response.Upserted, result.Upserted)
	}

	return response, nil
}
