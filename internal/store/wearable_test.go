package store

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

func TestWearableSyncRequestCanonicalDeviceUID(t *testing.T) {
	req := WearableSyncRequest{DeviceUIDAlias: " qring-alias "}
	if got := req.CanonicalDeviceUID(); got != "qring-alias" {
		t.Fatalf("CanonicalDeviceUID() = %q, want qring-alias", got)
	}
	req.DeviceUID = " primary "
	if got := req.CanonicalDeviceUID(); got != "primary" {
		t.Fatalf("CanonicalDeviceUID() = %q, want primary", got)
	}
}

func TestParseDeviceDayStartUsesDeviceTimezone(t *testing.T) {
	got, err := parseDeviceDayStart("2026-09-17", 420)
	if err != nil {
		t.Fatal(err)
	}
	want := "2026-09-16T17:00:00Z"
	if got.Format(time.RFC3339) != want {
		t.Fatalf("day start = %s, want %s", got.Format(time.RFC3339), want)
	}
}

func TestResolveDayTimestampFromMinuteOfDay(t *testing.T) {
	minute := 75 // 01:15 device local
	got, err := resolveDayTimestamp("2026-09-17", 420, "", &minute)
	if err != nil {
		t.Fatal(err)
	}
	want := "2026-09-16T18:15:00Z"
	if got.Format(time.RFC3339) != want {
		t.Fatalf("timestamp = %s, want %s", got.Format(time.RFC3339), want)
	}
}

func TestDeviceDateForUTCCrossesMidnight(t *testing.T) {
	ts := time.Date(2026, 9, 16, 18, 0, 0, 0, time.UTC)
	if got := deviceDateForUTC(ts, 420); got != "2026-09-17" {
		t.Fatalf("device date = %s, want 2026-09-17", got)
	}
}

func TestNormalizeNewSleepProtocolSegments(t *testing.T) {
	start := time.Date(2026, 9, 16, 15, 0, 0, 0, time.UTC)
	deepCode, remCode, awakeCode := 3, 4, 5
	deepDuration, remDuration, awakeDuration := 3600, 1800, 300
	segments := []WearableSleepSegment{
		{DurationS: &deepDuration, StageCode: &deepCode},
		{DurationS: &remDuration, StageCode: &remCode},
		{DurationS: &awakeDuration, StageCode: &awakeCode},
	}
	got, totals, end, err := normalizeSleepSegments(start, "", "new_sleep_protocol", segments)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("segments = %d, want 3", len(got))
	}
	if got[0].stage != "deep" || got[1].stage != "rem" || got[2].stage != "awake" {
		t.Fatalf("stages = %#v", []string{got[0].stage, got[1].stage, got[2].stage})
	}
	if totals.totalSleepS != 5400 || totals.deepS != 3600 || totals.remS != 1800 || totals.awakeS != 300 || totals.wakingCount != 1 {
		t.Fatalf("totals = %#v", totals)
	}
	if end.Sub(start) != 5700*time.Second {
		t.Fatalf("session duration = %s, want 5700s", end.Sub(start))
	}
}

func TestNormalizeLegacySleepCodes(t *testing.T) {
	cases := map[int]string{1: "deep", 2: "light", 3: "awake"}
	for code, want := range cases {
		c := code
		if got := normalizeSleepStage("legacy", &c, ""); got != want {
			t.Fatalf("code %d = %s, want %s", code, got, want)
		}
	}
}

func TestExplicitSleepStageWinsOverCode(t *testing.T) {
	code := 1
	if got := normalizeSleepStage("new_sleep_protocol", &code, "REM"); got != "rem" {
		t.Fatalf("stage = %s, want rem", got)
	}
}

func TestSnakeToCamel(t *testing.T) {
	cases := map[string]string{
		"device_date":      "deviceDate",
		"measurement_mode": "measurementMode",
		"tz_offset_min":    "tzOffsetMin",
		"alreadyCamel":     "alreadyCamel",
	}
	for input, want := range cases {
		if got := snakeToCamel(input); got != want {
			t.Fatalf("snakeToCamel(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizeBSONDocumentPreservesMetricValueKeys(t *testing.T) {
	doc := bson.M{
		"device_date": "2026-09-17",
		"values":      map[string]any{"heart_rate": 72, "side_temperature": 35.1},
	}
	got := normalizeBSONDocument(doc)
	values, ok := got["values"].(map[string]any)
	if !ok {
		t.Fatalf("values type = %T", got["values"])
	}
	want := map[string]any{"heart_rate": 72, "side_temperature": 35.1}
	if !reflect.DeepEqual(values, want) {
		t.Fatalf("values = %#v, want %#v", values, want)
	}
}

func TestResolveDayTimestampRejectsWrongDeviceDate(t *testing.T) {
	_, err := resolveDayTimestamp("2026-09-17", 420, "2026-09-16T16:59:59Z", nil)
	if err == nil {
		t.Fatal("expected timestamp/date mismatch error")
	}
}

func TestSyncWearableRejectsInvalidBatteryBeforeDatabaseWrite(t *testing.T) {
	battery := 101
	var st Store
	_, err := st.SyncWearable(context.Background(), WearableSyncRequest{
		DeviceUID:   "qring-test",
		TzOffsetMin: 420,
		DeviceState: &WearableDeviceState{BatteryPercent: &battery},
	})
	if !errors.Is(err, ErrInvalidWearablePayload) {
		t.Fatalf("SyncWearable() error = %v, want ErrInvalidWearablePayload", err)
	}
}
