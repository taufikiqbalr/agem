package store

import (
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"strings"
	"time"
)

func normalizeBSONDocument(doc bson.M) map[string]any {
	out := make(map[string]any, len(doc))
	for key, value := range doc {
		if key == "_id" {
			continue
		}
		camel := snakeToCamel(key)
		preserveKeys := key == "metadata" || key == "values" || key == "raw_values" || key == "payload" || key == "channels" || key == "capabilities"
		out[camel] = normalizeBSONValue(value, preserveKeys)
	}
	return out
}

func normalizeBSONValue(value any, preserveKeys bool) any {
	switch v := value.(type) {
	case time.Time:
		return v.UTC().Format(time.RFC3339Nano)
	case primitive.DateTime:
		return v.Time().UTC().Format(time.RFC3339Nano)
	case primitive.ObjectID:
		return v.Hex()
	case bson.M:
		out := make(map[string]any, len(v))
		for key, item := range v {
			newKey := key
			if !preserveKeys {
				newKey = snakeToCamel(key)
			}
			childPreserve := preserveKeys || key == "metadata" || key == "values" || key == "raw_values" || key == "payload" || key == "channels" || key == "capabilities"
			out[newKey] = normalizeBSONValue(item, childPreserve)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			newKey := key
			if !preserveKeys {
				newKey = snakeToCamel(key)
			}
			out[newKey] = normalizeBSONValue(item, preserveKeys)
		}
		return out
	case primitive.A:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = normalizeBSONValue(item, preserveKeys)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = normalizeBSONValue(item, preserveKeys)
		}
		return out
	default:
		return value
	}
}

func appendDocsToDays(docs []bson.M, getDay func(string) map[string]any, field string) {
	for _, doc := range docs {
		date := asString(doc["device_date"])
		if date == "" {
			date = asString(doc["sleep_date"])
		}
		if date == "" {
			continue
		}
		appendDayItem(getDay(date), field, normalizeBSONDocument(doc))
	}
}

func appendDayItem(day map[string]any, field string, value any) {
	items, _ := day[field].([]any)
	items = append(items, value)
	day[field] = items
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func snakeToCamel(value string) string {
	parts := strings.Split(value, "_")
	if len(parts) == 1 {
		return value
	}
	var b strings.Builder
	b.WriteString(parts[0])
	for _, part := range parts[1:] {
		if part == "" {
			continue
		}
		b.WriteString(strings.ToUpper(part[:1]))
		b.WriteString(part[1:])
	}
	return b.String()
}

func sanitizeName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "-", "_")
	value = strings.ReplaceAll(value, " ", "_")
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func valueOrDefault(value *int, fallback int) int {
	if value != nil {
		return *value
	}
	return fallback
}

func cloneBSON(in bson.M) bson.M {
	out := make(bson.M, len(in)+1)
	for key, value := range in {
		out[key] = value
	}
	return out
}

func addWearableCounters(dst *WearableSyncUpsertCounters, src WearableSyncUpsertCounters) {
	dst.DeviceState += src.DeviceState
	dst.DailyActivity += src.DailyActivity
	dst.ActivityBuckets += src.ActivityBuckets
	dst.Measurements += src.Measurements
	dst.SleepSessions += src.SleepSessions
	dst.SleepSegments += src.SleepSegments
	dst.SleepSummaries += src.SleepSummaries
	dst.Targets += src.Targets
	dst.Workouts += src.Workouts
	dst.Events += src.Events
	dst.RawSamples += src.RawSamples
	dst.SyncMetadata += src.SyncMetadata
}

func wearablePayloadError(message string) error {
	return fmt.Errorf("%w: %s", ErrInvalidWearablePayload, message)
}

func isEmptyWearableDeviceMetadata(meta WearableDeviceMetadata) bool {
	return strings.TrimSpace(meta.Vendor) == "" &&
		strings.TrimSpace(meta.Model) == "" &&
		strings.TrimSpace(meta.HWRev) == "" &&
		strings.TrimSpace(meta.FWRev) == "" &&
		strings.TrimSpace(meta.SerialNumber) == "" &&
		strings.TrimSpace(meta.SDKVersion) == "" &&
		meta.Capabilities == nil
}
