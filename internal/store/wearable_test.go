package store

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"
)

const sampleWearableSyncJSON = `{
  "deviceUid": "qring-aa-bb-cc-01",
  "hrv": {
    "current": 42,
    "average": 45,
    "readingsCount": 12,
    "lastUpdated": "2026-07-27T23:44:10.512Z"
  },
  "heartRate": {
    "current": 77,
    "average": 74,
    "min": 58,
    "max": 112,
    "readingsCount": 96,
    "lastUpdated": "2026-07-27T23:44:10.512Z"
  },
  "spo2": {
    "current": 97.0,
    "average": 96,
    "min": 94,
    "max": 99,
    "readingsCount": 24,
    "lastUpdated": "2026-07-27T23:44:10.512Z"
  },
  "temperature": {
    "current": 36.7,
    "average": 36.5,
    "raw": 36.72,
    "readingsCount": 18,
    "lastUpdated": "2026-07-27T23:44:10.512Z"
  },
  "stress": {
    "current": 30,
    "average": 34,
    "min": 12,
    "max": 71,
    "readingsCount": 48,
    "lastUpdated": "2026-07-27T23:44:10.512Z"
  },
  "activity": {
    "steps": 717,
    "calories": 28390.0,
    "distance": 493,
    "activeTime": 29,
    "lastUpdated": "2026-07-27T23:44:10.512Z"
  },
  "sleep": {
    "totalMinutes": 431,
    "deepMinutes": 92,
    "lightMinutes": 268,
    "remMinutes": 61,
    "awakeMinutes": 10,
    "score": 78,
    "sleepStart": "2026-07-26T22:41:00.000Z",
    "sleepEnd": "2026-07-27T05:52:00.000Z",
    "lastUpdated": "2026-07-27T23:44:10.512Z",
    "stageData": [2, 2, 3, 3, 1, 1, 1, 2, 4]
  },
  "bloodPressure": {
    "systolic": 118,
    "diastolic": 76,
    "heartRate": 72,
    "measurementTime": "2026-07-27T07:12:00.000Z",
    "lastUpdated": "2026-07-27T23:44:10.512Z"
  },
  "totalActivities": [
    {"value": 717.0, "target": 4000.0},
    {"value": 28.39, "target": 500.0},
    {"value": 0.493, "target": 4.0}
  ]
}`

const sampleWearableSyncWithReadingsJSON = `{
  "deviceUid": "qring-aa-bb-cc-01",
  "tzOffsetMin": 420,
  "hrv": {
    "current": 42,
    "average": 45,
    "readingsCount": 2,
    "lastUpdated": "2026-07-27T23:44:10.512Z",
    "readings": [
      { "ts": "2026-07-27T23:43:10.123Z", "value": 41 },
      { "ts": "2026-07-27T23:44:10.512Z", "value": 42 }
    ]
  },
  "heartRate": {
    "current": 77,
    "average": 74,
    "min": 58,
    "max": 112,
    "readingsCount": 2,
    "lastUpdated": "2026-07-27T23:44:10.512Z",
    "readings": [
      { "ts": "2026-07-27T23:43:10.123Z", "value": 76 },
      { "ts": "2026-07-27T23:44:10.512Z", "value": 77 }
    ]
  },
  "spo2": {
    "current": 97,
    "average": 96,
    "readingsCount": 1,
    "lastUpdated": "2026-07-27T23:44:10.512Z",
    "readings": [
      { "ts": "2026-07-27T23:44:10.512Z", "value": 97 }
    ]
  },
  "temperature": {
    "current": 36.7,
    "average": 36.5,
    "raw": 36.72,
    "readingsCount": 1,
    "lastUpdated": "2026-07-27T23:44:10.512Z",
    "readings": [
      { "ts": "2026-07-27T23:44:10.512Z", "value": 36.7, "raw": 36.72 }
    ]
  },
  "stress": {
    "current": 30,
    "average": 34,
    "readingsCount": 1,
    "lastUpdated": "2026-07-27T23:44:10.512Z",
    "readings": [
      { "ts": "2026-07-27T23:44:10.512Z", "value": 30 }
    ]
  },
  "activity": {
    "steps": 717,
    "calories": 28390,
    "distance": 493,
    "activeTime": 29,
    "lastUpdated": "2026-07-27T23:44:10.512Z",
    "readings": [
      { "ts": "2026-07-27T08:00:00.000Z", "steps": 120, "calories": 8, "distance": 95, "activeTime": 15 }
    ]
  },
  "sleep": {
    "totalMinutes": 431,
    "deepMinutes": 92,
    "lightMinutes": 268,
    "remMinutes": 61,
    "awakeMinutes": 10,
    "score": 78,
    "sleepStart": "2026-07-26T22:41:00.000Z",
    "sleepEnd": "2026-07-27T05:52:00.000Z",
    "lastUpdated": "2026-07-27T23:44:10.512Z",
    "stageData": [2, 2, 3, 3, 1, 1, 1, 2, 4],
    "segments": [
      { "start": "2026-07-26T22:41:00.000Z", "end": "2026-07-26T23:11:00.000Z", "stage": "light" }
    ]
  },
  "bloodPressure": {
    "systolic": 118,
    "diastolic": 76,
    "heartRate": 72,
    "measurementTime": "2026-07-27T07:12:00.000Z",
    "lastUpdated": "2026-07-27T23:44:10.512Z",
    "readings": [
      { "ts": "2026-07-27T07:12:00.000Z", "systolic": 118, "diastolic": 76, "heartRate": 72 }
    ]
  },
  "totalActivities": [
    { "kind": "steps", "value": 717, "target": 4000 },
    { "kind": "calories", "value": 28.39, "target": 500 },
    { "kind": "distance", "value": 0.493, "target": 4 }
  ]
}`

func TestWearableSyncRequestValidationHelpers(t *testing.T) {
	req := WearableSyncRequest{DeviceUIDAlias: " qring-alias ", Activity: &WearableActivitySection{}}
	if got := req.CanonicalDeviceUID(); got != "qring-alias" {
		t.Fatalf("CanonicalDeviceUID() = %q, want qring-alias", got)
	}
	if !req.HasRecognizedData() {
		t.Fatal("HasRecognizedData() = false, want true")
	}

	req = WearableSyncRequest{DeviceUID: "qring-empty"}
	if req.HasRecognizedData() {
		t.Fatal("HasRecognizedData() = true for request without wearable sections")
	}
}

func TestSyncWearableRejectsMissingDeviceUID(t *testing.T) {
	var st Store
	_, err := st.SyncWearable(context.Background(), WearableSyncRequest{
		Activity: &WearableActivitySection{Steps: float64Ptr(1)},
	})
	if !errors.Is(err, ErrInvalidWearablePayload) {
		t.Fatalf("SyncWearable() error = %v, want ErrInvalidWearablePayload", err)
	}
}

func TestSyncWearableRejectsNoRecognizedSections(t *testing.T) {
	var st Store
	_, err := st.SyncWearable(context.Background(), WearableSyncRequest{DeviceUID: "qring-empty"})
	if !errors.Is(err, ErrInvalidWearablePayload) {
		t.Fatalf("SyncWearable() error = %v, want ErrInvalidWearablePayload", err)
	}
}

func TestWearableSyncDateDerivedFromLastUpdated(t *testing.T) {
	req := mustSampleWearableSyncRequest(t)
	got, err := req.DeriveSyncTime(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("DeriveSyncTime() error = %v", err)
	}
	if got.Format(wearableDateLayout) != "2026-07-27" {
		t.Fatalf("sync date = %s, want 2026-07-27", got.Format(wearableDateLayout))
	}
}

func TestBuildWearableFanoutMapsSamplePayload(t *testing.T) {
	req := mustSampleWearableSyncRequest(t)
	syncTime, err := req.DeriveSyncTime(time.Now().UTC())
	if err != nil {
		t.Fatalf("DeriveSyncTime() error = %v", err)
	}
	fanout, err := buildWearableFanout("device-id", req.CanonicalDeviceUID(), req, "2026-07-27", syncTime)
	if err != nil {
		t.Fatalf("buildWearableFanout() error = %v", err)
	}

	if len(fanout.healthMetrics) != 6 {
		t.Fatalf("health metrics count = %d, want 6", len(fanout.healthMetrics))
	}
	metrics := map[string]HealthMetricRow{}
	for _, metric := range fanout.healthMetrics {
		metrics[metric.Metric] = metric
	}

	assertMetric(t, metrics["hrv"], "ms", 42, map[string]float64{"average": 45, "readings_count": 12})
	assertMetric(t, metrics["heart_rate"], "bpm", 77, map[string]float64{"average": 74, "min": 58, "max": 112, "readings_count": 96})
	assertMetric(t, metrics["spo2"], "%", 97, map[string]float64{"average": 96, "min": 94, "max": 99, "readings_count": 24})
	assertMetric(t, metrics["temperature"], "celsius", 36.7, map[string]float64{"average": 36.5, "raw": 36.72, "readings_count": 18})
	assertMetric(t, metrics["stress"], "score", 30, map[string]float64{"average": 34, "min": 12, "max": 71, "readings_count": 48})

	bp := metrics["blood_pressure"]
	if bp.Unit != "mmHg" {
		t.Fatalf("blood pressure unit = %q, want mmHg", bp.Unit)
	}
	if bp.TsUTC.Format(time.RFC3339) != "2026-07-27T07:12:00Z" {
		t.Fatalf("blood pressure ts = %s, want measurementTime", bp.TsUTC.Format(time.RFC3339))
	}
	if !reflect.DeepEqual(bp.Values, map[string]float64{"systolic": 118, "diastolic": 76, "heart_rate": 72}) {
		t.Fatalf("blood pressure values = %#v", bp.Values)
	}

	if fanout.dailyActivity == nil {
		t.Fatal("dailyActivity = nil")
	}
	if got := *fanout.dailyActivity; got.TotalSteps != 717 || got.Calories != 28390 || got.WalkDistanceM != 493 || got.SportDurationS != 1740 {
		t.Fatalf("daily activity mapping = %#v", got)
	}

	if fanout.sleepSummary == nil {
		t.Fatal("sleepSummary = nil")
	}
	sleep := fanout.sleepSummary
	if sleep.TotalSleepS != 25860 || sleep.DeepS != 5520 || sleep.LightS != 16080 || sleep.RemS != 3660 || sleep.AwakeS != 600 {
		t.Fatalf("sleep seconds mapping = %#v", sleep)
	}
	if sleep.Score == nil || *sleep.Score != 78 {
		t.Fatalf("sleep score = %#v, want 78", sleep.Score)
	}
	if !reflect.DeepEqual(sleep.StageData, []int{2, 2, 3, 3, 1, 1, 1, 2, 4}) {
		t.Fatalf("stage data = %#v", sleep.StageData)
	}

	if len(fanout.totalActivities) != 3 {
		t.Fatalf("total activities count = %d, want 3", len(fanout.totalActivities))
	}
	kinds := []string{fanout.totalActivities[0].Kind, fanout.totalActivities[1].Kind, fanout.totalActivities[2].Kind}
	if !reflect.DeepEqual(kinds, []string{"steps", "calories", "distance"}) {
		t.Fatalf("total activity kinds = %#v", kinds)
	}
}

func TestParseWearableTimeAcceptsMilliseconds(t *testing.T) {
	got, err := parseWearableTime("2026-07-27T23:43:10.123Z", "sample.ts")
	if err != nil {
		t.Fatalf("parseWearableTime() error = %v", err)
	}
	if got.Format(time.RFC3339Nano) != "2026-07-27T23:43:10.123Z" {
		t.Fatalf("parsed time = %s", got.Format(time.RFC3339Nano))
	}
}

func TestBuildWearableFanoutMapsTimestampedReadings(t *testing.T) {
	req := mustWearableSyncRequest(t, sampleWearableSyncWithReadingsJSON)
	syncTime, err := req.DeriveSyncTime(time.Now().UTC())
	if err != nil {
		t.Fatalf("DeriveSyncTime() error = %v", err)
	}
	fanout, err := buildWearableFanout("device-id", req.CanonicalDeviceUID(), req, "2026-07-27", syncTime)
	if err != nil {
		t.Fatalf("buildWearableFanout() error = %v", err)
	}

	if len(fanout.healthMetrics) != 14 {
		t.Fatalf("health metrics count = %d, want 14", len(fanout.healthMetrics))
	}
	var hrvReading *HealthMetricRow
	var temperatureReading *HealthMetricRow
	var bpReading *HealthMetricRow
	for i := range fanout.healthMetrics {
		row := &fanout.healthMetrics[i]
		if row.Source != wearableMetricReadingSource {
			continue
		}
		switch row.Metric {
		case "hrv":
			if row.TsUTC.Format(time.RFC3339Nano) == "2026-07-27T23:43:10.123Z" {
				hrvReading = row
			}
		case "temperature":
			temperatureReading = row
		case "blood_pressure":
			bpReading = row
		}
	}
	if hrvReading == nil || hrvReading.Value == nil || *hrvReading.Value != 41 {
		t.Fatalf("missing first HRV reading: %#v", hrvReading)
	}
	if temperatureReading == nil || temperatureReading.Value == nil || *temperatureReading.Value != 36.7 || temperatureReading.Values["raw"] != 36.72 {
		t.Fatalf("temperature reading mapping = %#v", temperatureReading)
	}
	if bpReading == nil || !reflect.DeepEqual(bpReading.Values, map[string]float64{"systolic": 118, "diastolic": 76, "heart_rate": 72}) {
		t.Fatalf("blood pressure reading mapping = %#v", bpReading)
	}

	if len(fanout.steps15m) != 1 {
		t.Fatalf("steps15m count = %d, want 1", len(fanout.steps15m))
	}
	step := fanout.steps15m[0]
	if step.DeviceDate != "2026-07-27" || step.TimeIndex != 60 || step.TzOffsetMin != 420 {
		t.Fatalf("step date/index/tz mapping = %#v", step)
	}
	if step.WalkSteps != 120 || step.Calories != 8 || step.DistanceM != 95 || step.SportDurationS != 900 || step.Source != wearableActivityReadingSource {
		t.Fatalf("step values mapping = %#v", step)
	}

	if len(fanout.sleepSegments) != 1 {
		t.Fatalf("sleep segment count = %d, want 1", len(fanout.sleepSegments))
	}
	segment := fanout.sleepSegments[0]
	if segment.SleepDate != "2026-07-27" || segment.TzOffsetMin != 420 || segment.Stage != "light" || segment.Source != wearableSleepSegmentSource {
		t.Fatalf("sleep segment mapping = %#v", segment)
	}

	kinds := []string{fanout.totalActivities[0].Kind, fanout.totalActivities[1].Kind, fanout.totalActivities[2].Kind}
	if !reflect.DeepEqual(kinds, []string{"steps", "calories", "distance"}) {
		t.Fatalf("explicit total activity kinds = %#v", kinds)
	}
}

func TestWearableRangeItemStripsReadingsByDefault(t *testing.T) {
	req := mustWearableSyncRequest(t, sampleWearableSyncWithReadingsJSON)
	doc := wearableSyncDoc{
		DeviceUID: "qring-aa-bb-cc-01",
		SyncDate:  "2026-07-27",
		Payload:   req.SnapshotPayload(),
	}

	summary := doc.toRangeItem(false)
	if len(summary.HRV.Readings) != 0 || len(summary.Activity.Readings) != 0 || len(summary.Sleep.Segments) != 0 || len(summary.BloodPressure.Readings) != 0 {
		t.Fatalf("default range item kept readings: %#v", summary)
	}

	full := doc.toRangeItem(true)
	if len(full.HRV.Readings) != 2 || len(full.Activity.Readings) != 1 || len(full.Sleep.Segments) != 1 || len(full.BloodPressure.Readings) != 1 {
		t.Fatalf("include_readings range item stripped readings: %#v", full)
	}
}

func TestTotalActivityKindUnknownIndex(t *testing.T) {
	if got := totalActivityKind(3, ""); got != "unknown_3" {
		t.Fatalf("totalActivityKind(3) = %q, want unknown_3", got)
	}
	if got := totalActivityKind(0, "Active Calories"); got != "active_calories" {
		t.Fatalf("totalActivityKind explicit = %q, want active_calories", got)
	}
}

func mustSampleWearableSyncRequest(t *testing.T) WearableSyncRequest {
	t.Helper()
	return mustWearableSyncRequest(t, sampleWearableSyncJSON)
}

func mustWearableSyncRequest(t *testing.T, raw string) WearableSyncRequest {
	t.Helper()

	var req WearableSyncRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		t.Fatalf("sample json unmarshal error = %v", err)
	}
	return req
}

func assertMetric(t *testing.T, got HealthMetricRow, unit string, value float64, values map[string]float64) {
	t.Helper()

	if got.Unit != unit {
		t.Fatalf("%s unit = %q, want %q", got.Metric, got.Unit, unit)
	}
	if got.Value == nil || *got.Value != value {
		t.Fatalf("%s value = %#v, want %v", got.Metric, got.Value, value)
	}
	if !reflect.DeepEqual(got.Values, values) {
		t.Fatalf("%s values = %#v, want %#v", got.Metric, got.Values, values)
	}
	if got.Source != wearableSyncSource {
		t.Fatalf("%s source = %q, want %q", got.Metric, got.Source, wearableSyncSource)
	}
}

func float64Ptr(v float64) *float64 {
	return &v
}
