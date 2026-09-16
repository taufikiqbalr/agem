# AGEM Backend — QRing SDK v3

Go backend for AGEM/QRing wearable devices. The canonical frontend API is **v3**. v3 is aligned with the QRing Android SDK used by the mobile frontend and is designed to preserve the semantics of historical SDK synchronization instead of flattening everything into one daily snapshot.

The service stores wearable data in MongoDB (`regene_kalaagem` by default). User identity can come from the Regene `users` collection while AGEM owns devices, pairings, measurements, activity, sleep, workouts, events, raw samples, and sync metadata.

## Why v3 was redesigned

The QRing SDK exposes multiple independent synchronization paths:

- daily activity totals (`BleStepTotal` / `TodaySportDataRsp`)
- 15-minute activity details (`BleStepDetails`)
- historical heart rate arrays
- historical SpO2 min/max values
- blood pressure
- stress / pressure
- HRV
- continuous and manual temperature
- legacy and new sleep protocols, including lunch/nap sleep
- sedentary state
- workouts / sport records
- battery and charging state
- manual and one-click measurements
- optional raw PPG / accelerometer-related values

Several SDK calls synchronize historical data using day offsets. Therefore v3 requires **explicit device-local dates** and supports **multiple days per request**. The backend no longer derives one `sync_date` from the latest timestamp in the request.

## Stack

- Go 1.24+
- `github.com/go-chi/chi/v5`
- MongoDB Go driver
- MongoDB regular + time-series collections

## Run locally

```bash
ADDR=:8080 \
MONGODB_URI='mongodb://user:password@localhost:27017/regene_kalaagem?authSource=admin' \
go run ./cmd/api
```

Health check:

```bash
curl http://localhost:8080/healthz
```

```json
{"status":"ok"}
```

## Canonical architecture

```text
QRing / G69
    │ Bluetooth LE
    ▼
Android frontend + QRing SDK
    │
    │ POST /v3/wearable/sync
    ▼
AGEM Go API
    │
    ├── devices                 current device metadata/state/capabilities
    ├── user_devices            user ↔ device pairing history
    ├── daily_activity          device-local daily activity totals
    ├── steps_15m               15-minute activity buckets (time series)
    ├── health_metrics          normalized measurements (time series)
    ├── sleep_sessions          main sleep + nap/lunch sleep
    ├── sleep_segments          sleep stage timeline (time series)
    ├── sleep_summary           daily aggregate derived from sessions
    ├── workouts                training/sport sessions
    ├── device_events           sedentary/charging/device events (time series)
    ├── raw_sensor_samples      optional short-retention raw sensor samples
    ├── total_activities        daily target/progress values
    └── wearable_syncs          per-day sync metadata/coverage
```

`wearable_syncs` is **not the authoritative sensor store**. v3 range responses are assembled from the normalized collections above. This prevents partial sync requests from overwriting previously synchronized domains.

---

# v3 endpoints

## Device / pairing

| Method | Route | Purpose |
|---|---|---|
| `GET` | `/v3/devices/by_uid/{device_uid}` | Get registered device metadata/state. |
| `POST` | `/v3/users/{user_id}/devices` | Pair a registered device to a user by stable device UID. |
| `GET` | `/v3/users/{user_id}/devices` | List active device pairings. |
| `PATCH` | `/v3/users/{user_id}/devices/{device_uid}` | Update pairing nickname / primary flag. |
| `DELETE` | `/v3/users/{user_id}/devices/{device_uid}` | Soft-unpair; history is preserved. |

## Wearable synchronization / readback

| Method | Route | Purpose |
|---|---|---|
| `POST` | `/v3/wearable/sync` | Canonical QRing SDK ingest endpoint. |
| `GET` | `/v3/wearable/range?device_uid=...&from=YYYY-MM-DD&to=YYYY-MM-DD` | Read normalized data by device UID. |
| `GET` | `/v3/devices/{device_id}/wearable/range?from=...&to=...` | Read normalized data by Mongo device ID. |
| `GET` | `/v3/users/{user_id}/wearable/range?from=...&to=...` | Read all active devices for a user. |

Add `include_raw=true` only when diagnostic raw sensor samples are required. Raw samples are intentionally excluded by default.

The older `/v1` collection-oriented endpoints remain in the router for legacy clients, but new QRing frontend work should use v3.

---

# Device registration model

A dedicated registration call is not required. The first successful `/v3/wearable/sync` upserts the device by stable `deviceUid`.

Recommended first sync:

```json
{
  "deviceUid": "AA:BB:CC:DD:EE:FF",
  "tzOffsetMin": 420,
  "device": {
    "vendor": "QRing",
    "model": "G69",
    "hwRev": "1.0",
    "fwRev": "1.2.3",
    "serialNumber": "G69-001",
    "sdkVersion": "2025-08-26",
    "capabilities": {
      "heartRate": true,
      "spo2": true,
      "temperature": true,
      "hrv": true,
      "newSleepProtocol": true,
      "oneClickMeasurement": true,
      "bloodPressure": false
    }
  },
  "deviceState": {
    "batteryPercent": 82,
    "charging": false,
    "lastSeenAt": "2026-09-16T08:30:00Z"
  }
}
```

### Device capabilities

The SDK exposes capability flags through `SetTimeRsp` and `DeviceSupportFunctionRsp`. Store capability values reported by the actual device rather than assuming all devices using the SDK support all functions.

For example, a G69 product profile can report HR, SpO2, temperature, HRV, all-day sleep, sedentary reminder, and sport features while another QRing model may expose blood pressure or other capabilities.

`capabilities` is an open boolean map so new SDK feature flags can be added without a database migration.

### Current device state

`deviceState` updates the current `devices` document:

- `battery_percent`
- `charging`
- `last_seen_at`
- `state_updated_at`

Battery is also persisted as a `health_metrics` point (`metric=battery_level`). Charging state is also persisted as a `device_events` point (`event_type=charging_state`) so current state and history are both available.

---

# Pairing

Pairing uses the stable `deviceUid`; the frontend does not need to know the internal MongoDB device ID.

```bash
curl -X POST "$BASE/v3/users/$USER_ID/devices" \
  -H 'Content-Type: application/json' \
  -d '{
    "deviceUid": "AA:BB:CC:DD:EE:FF",
    "nickname": "My G69",
    "isPrimary": true
  }'
```

Rules enforced by database indexes and the v3 store:

1. A user cannot have duplicate active pairings to the same device.
2. A physical device can have only one active owner at a time.
3. A user can have only one active primary wearable.
4. Unpairing is soft: `unpaired_at` is set and the historical pairing record is retained.
5. `user_id` must resolve in the configured Regene `users` collection, with local AGEM `users` as fallback when applicable.

Update pairing:

```bash
curl -X PATCH "$BASE/v3/users/$USER_ID/devices/AA:BB:CC:DD:EE:FF" \
  -H 'Content-Type: application/json' \
  -d '{"nickname":"Left wrist","isPrimary":true}'
```

Unpair:

```bash
curl -X DELETE "$BASE/v3/users/$USER_ID/devices/AA:BB:CC:DD:EE:FF"
```

---

# `/v3/wearable/sync`

## Request-level fields

| Field | Type | Required | Description |
|---|---|---:|---|
| `deviceUid` | string | yes | Stable QRing identifier (typically the stable BLE/device identifier chosen by the frontend). |
| `device_uid` | string | alias | Compatibility alias for `deviceUid`. |
| `tzOffsetMin` | integer | yes | Device-local UTC offset in minutes, e.g. Jakarta `420`. Range `-840..840`. |
| `device` | object | no | Device metadata and capabilities. |
| `deviceState` | object | no | Current battery/charging/last-seen state. |
| `days` | array | no | Explicit device-local days containing synchronized SDK data. |

At least one of `device`, `deviceState`, or `days` must contain data.

Request bodies are limited to **8 MiB**. Raw samples should be batched instead of sending an unbounded diagnostic stream.

Unknown JSON fields are rejected. This is intentional: SDK mapping mistakes should fail visibly rather than silently losing wearable data.

## Multi-day rule

Each synchronized historical SDK day is explicit:

```json
{
  "days": [
    {"date":"2026-09-14"},
    {"date":"2026-09-15"},
    {"date":"2026-09-16"}
  ]
}
```

The backend does **not** derive a day from the latest measurement timestamp. This matches QRing APIs that synchronize `offset=0..6` / historical day ranges.

Duplicate `date` values inside one request are rejected.

---

# Activity mapping

## Daily activity

The QRing `BleStepTotal` / daily response exposes fields such as total steps, running steps, calorie value, walking distance, sport duration, and sleep duration.

v3 payload:

```json
{
  "date": "2026-09-16",
  "activity": {
    "totalSteps": 717,
    "runningSteps": 20,
    "caloriesRaw": 28390,
    "caloriesKcal": 28.39,
    "distanceM": 493,
    "sportDurationS": 1740,
    "sleepDurationS": 25860
  }
}
```

### Important unit rule

The backend never silently multiplies or divides calorie values. The SDK exposes raw calorie values and some SDK target APIs use scaled values. Therefore:

- `caloriesRaw`: preserve the exact integer returned by the device/SDK.
- `caloriesKcal`: optional normalized kcal computed by the frontend only when the SDK/model semantics are known.

Likewise, `sportDurationS` and `sleepDurationS` are explicitly **seconds**. The old ambiguous `activeTime` field is no longer part of v3.

## 15-minute activity buckets

`BleStepDetails` is naturally modeled using the SDK `timeIndex` (`0..95`):

```json
{
  "activityBuckets": [
    {
      "timeIndex": 32,
      "walkSteps": 110,
      "runSteps": 10,
      "caloriesRaw": 8000,
      "caloriesKcal": 8.0,
      "distanceM": 95
    }
  ]
}
```

The backend calculates `ts_utc` from:

```text
day.date + timeIndex * 15 minutes + tzOffsetMin
```

This preserves device-local day semantics and avoids UTC-midnight grouping errors.

---

# Measurements

All scalar/multi-value health measurements use one normalized structure and are written to `health_metrics`.

```json
{
  "metric": "heart_rate",
  "ts": "2026-09-16T08:00:00Z",
  "value": 77,
  "unit": "bpm",
  "measurementMode": "history_sync",
  "sampleIntervalSec": 300,
  "source": "qring_sdk_v3"
}
```

A measurement can identify its time in either form:

- RFC3339/RFC3339Nano `ts`, or
- device-local `minuteOfDay` (`0..1439`) relative to `day.date`.

### Measurement fields

| Field | Purpose |
|---|---|
| `metric` | Canonical metric name. |
| `value` | Main normalized scalar value. |
| `values` | Multi-value normalized reading. |
| `rawValue` | Original SDK numeric value when scaling/conversion exists. |
| `rawValues` | Original multi-channel values. |
| `unit` | `bpm`, `%`, `mmHg`, `celsius`, `ms`, `score`, etc. |
| `measurementMode` | Provenance such as `automatic`, `history_sync`, `manual`, `one_click`, `realtime`, `derived`. |
| `sessionId` | Correlates measurements produced by one manual/one-click session. |
| `sampleIntervalSec` | Original sampling interval when known. |
| `errorCode` | SDK measurement error/status code. |
| `derived` | Whether value was calculated rather than directly measured. |
| `algorithm` | Calculation algorithm name when derived. |
| `metadata` | SDK/model-specific details that should not become schema columns. |

Recommended metric names:

```text
heart_rate
spo2
blood_pressure
hrv
stress
temperature
rri
battery_level
```

Additional metrics are allowed; `metric` is normalized to lowercase snake case.

## Heart rate history

For `ReadHeartRateRsp`, create individual points at the SDK sampling interval (commonly 5 minutes) and set:

```json
{
  "metric": "heart_rate",
  "minuteOfDay": 480,
  "value": 77,
  "unit": "bpm",
  "measurementMode": "history_sync",
  "sampleIntervalSec": 300
}
```

## SpO2 hourly min/max

`BloodOxygenEntity` provides per-hour minimum and maximum values. Preserve both:

```json
{
  "metric": "spo2",
  "minuteOfDay": 480,
  "values": {"min":95,"max":98},
  "unit": "%",
  "measurementMode": "history_sync",
  "sampleIntervalSec": 3600
}
```

## Stress and HRV raw scaling

When the SDK provides a raw array value that the app divides/scales for display, store both raw and normalized values:

```json
{
  "metric": "stress",
  "minuteOfDay": 600,
  "value": 34.2,
  "rawValue": 342,
  "unit": "score",
  "sampleIntervalSec": 1800,
  "measurementMode": "history_sync"
}
```

## Temperature channels

The current AAR exposes main and side temperature channels. Preserve them using `values` / `rawValues`:

```json
{
  "metric": "temperature",
  "ts": "2026-09-16T08:30:00Z",
  "value": 36.7,
  "values": {
    "side": 35.2,
    "side1": 35.1
  },
  "unit": "celsius",
  "measurementMode": "manual"
}
```

## Blood pressure provenance

Do not mix device-measured and SDK-derived BP without provenance:

```json
{
  "metric": "blood_pressure",
  "ts": "2026-09-16T08:40:00Z",
  "values": {
    "systolic": 118,
    "diastolic": 76,
    "heart_rate": 72
  },
  "unit": "mmHg",
  "measurementMode": "manual",
  "derived": false
}
```

If the frontend intentionally sends a value calculated by an SDK algorithm:

```json
{
  "metric": "blood_pressure",
  "ts": "2026-09-16T08:40:00Z",
  "values": {"systolic":118,"diastolic":76},
  "unit": "mmHg",
  "measurementMode": "derived",
  "derived": true,
  "algorithm": "CalcBloodPressureByHeart"
}
```

---

# Manual and one-click measurements

Use `measurementMode` and a shared `sessionId` to correlate values returned from one operation:

```json
{
  "measurements": [
    {
      "metric":"heart_rate",
      "ts":"2026-09-16T09:00:00Z",
      "value":76,
      "unit":"bpm",
      "measurementMode":"one_click",
      "sessionId":"m-20260916-001",
      "errorCode":0
    },
    {
      "metric":"hrv",
      "ts":"2026-09-16T09:00:00Z",
      "value":42,
      "unit":"ms",
      "measurementMode":"one_click",
      "sessionId":"m-20260916-001",
      "errorCode":0
    }
  ]
}
```

The v3 idempotency key includes `measurement_mode`, `session_id`, and `source`, so an automatic and a manual measurement can coexist at the same timestamp.

---

# Sleep

v3 models **sessions**, not only one nightly row. This supports all-day sleep and the SDK new sleep protocol with lunch/nap sleep.

```json
{
  "sleepSessions": [
    {
      "sessionId": "sleep-main-20260916",
      "type": "main",
      "protocol": "new_sleep_protocol",
      "start": "2026-09-15T15:30:00Z",
      "end": "2026-09-15T23:00:00Z",
      "wakingCount": 2,
      "segments": [
        {"durationS":1800,"stageCode":2},
        {"durationS":3600,"stageCode":3},
        {"durationS":1200,"stageCode":4},
        {"durationS":300,"stageCode":5}
      ]
    },
    {
      "sessionId": "sleep-nap-20260916",
      "type": "nap",
      "protocol": "new_sleep_protocol",
      "start": "2026-09-16T05:00:00Z",
      "end": "2026-09-16T05:35:00Z",
      "segments": [
        {"durationS":2100,"stageCode":2}
      ]
    }
  ]
}
```

Segments may use explicit `start`/`end` timestamps or sequential `durationS`. When timestamps are omitted, segments are chained from the session start.

### New sleep protocol stage codes

| Code | Normalized stage |
|---:|---|
| 0 | `not_sleeping` |
| 1 | `off_wrist` |
| 2 | `light` |
| 3 | `deep` |
| 4 | `rem` |
| 5 | `awake` |

### Legacy sleep codes

| Code | Normalized stage |
|---:|---|
| 1 | `deep` |
| 2 | `light` |
| 3 | `awake` |

If `stage` is explicitly provided, it wins over numeric-code inference.

The backend stores each session in `sleep_sessions`, each timeline point in `sleep_segments`, and recalculates `sleep_summary` for that device/date from **all sessions**. This prevents a nap from overwriting the main sleep session.

---

# Targets / goals

QRing target settings include steps, calorie, distance, sport duration, and sleep duration. Use explicit units:

```json
{
  "targets": [
    {"kind":"steps","value":717,"target":4000,"unit":"steps"},
    {"kind":"calories","value":28.39,"target":500,"unit":"kcal"},
    {"kind":"distance","value":0.493,"target":4,"unit":"km"},
    {"kind":"sport_duration","value":29,"target":60,"unit":"minute"},
    {"kind":"sleep_duration","value":431,"target":480,"unit":"minute"}
  ]
}
```

---

# Workouts

`SportPlusEntity` maps directly into v3 workout fields:

```json
{
  "workouts": [
    {
      "startUtc": "2026-09-16T00:15:00Z",
      "endUtc": "2026-09-16T00:55:00Z",
      "sportTypeId": 7,
      "sportTypeName": "running",
      "durationS": 2400,
      "distanceM": 5200,
      "calories": 310.5,
      "avgHr": 142,
      "minHr": 91,
      "maxHr": 174,
      "avgSpeedCmS": 217,
      "maxSpeedCmS": 330,
      "elevationCm": 2300,
      "uphillCm": 12000,
      "downhillCm": 11000,
      "avgCadenceSpm": 166,
      "sportCount": 1,
      "steps": 6400,
      "locations": [
        {"rateReal":141},
        {"rateReal":145}
      ]
    }
  ]
}
```

`workouts` remains a regular collection because sessions can be corrected/upserted.

---

# Device events / sedentary data

Non-measurement SDK state changes belong in `device_events`.

Example sedentary synchronization:

```json
{
  "events": [
    {
      "minuteOfDay": 600,
      "eventType": "sedentary",
      "payload": {
        "state": 1,
        "durationS": 1800
      }
    }
  ]
}
```

Suggested state mapping from the SDK sedentary detail:

```text
0 = static
1 = sedentary_triggered
2 = movement
```

Other suitable event types include:

```text
charging_state
touch
gesture
sport_status
device_notify
not_wearing
```

---

# Optional raw sensor samples

Raw PPG/accelerometer diagnostic values are intentionally separated from `health_metrics` because their volume can be much higher.

```json
{
  "rawSamples": [
    {
      "ts": "2026-09-16T09:00:00.125Z",
      "sessionId": "raw-001",
      "sampleIndex": 0,
      "kind": "ppg_accelerometer",
      "channels": {
        "greenLightPpgL": 123,
        "greenLightPpgH": 2,
        "redLightPpgL": 112,
        "redLightPpgH": 1,
        "infraredPpgL": 98,
        "infraredPpgH": 1,
        "xL": 1,
        "xH": 0,
        "yL": 2,
        "yH": 0,
        "zL": 3,
        "zH": 0
      }
    }
  ]
}
```

The backend intentionally stores `channels` as a flexible numeric map. It does not invent a bit-combination formula for SDK L/H channel fields; combine them in the frontend only when the SDK/vendor definition is known.

Default raw-sensor retention is **30 days**.

---

# Complete multi-day example

```json
{
  "deviceUid": "AA:BB:CC:DD:EE:FF",
  "tzOffsetMin": 420,
  "device": {
    "vendor": "QRing",
    "model": "G69",
    "hwRev": "1.0",
    "fwRev": "1.2.3",
    "sdkVersion": "2025-08-26",
    "capabilities": {
      "heartRate": true,
      "spo2": true,
      "temperature": true,
      "hrv": true,
      "newSleepProtocol": true
    }
  },
  "deviceState": {
    "batteryPercent": 82,
    "charging": false,
    "lastSeenAt": "2026-09-16T09:10:00Z"
  },
  "days": [
    {
      "date": "2026-09-15",
      "activity": {
        "totalSteps": 8200,
        "runningSteps": 650,
        "caloriesRaw": 365000,
        "caloriesKcal": 365.0,
        "distanceM": 6100,
        "sportDurationS": 3150,
        "sleepDurationS": 25440
      },
      "activityBuckets": [
        {"timeIndex":32,"walkSteps":110,"runSteps":10,"caloriesRaw":8000,"caloriesKcal":8,"distanceM":95}
      ],
      "measurements": [
        {"metric":"heart_rate","minuteOfDay":480,"value":77,"unit":"bpm","measurementMode":"history_sync","sampleIntervalSec":300},
        {"metric":"spo2","minuteOfDay":480,"values":{"min":95,"max":98},"unit":"%","measurementMode":"history_sync","sampleIntervalSec":3600},
        {"metric":"hrv","minuteOfDay":600,"value":42,"rawValue":420,"unit":"ms","measurementMode":"history_sync","sampleIntervalSec":1800}
      ],
      "targets": [
        {"kind":"steps","value":8200,"target":10000,"unit":"steps"}
      ]
    },
    {
      "date": "2026-09-16",
      "activity": {
        "totalSteps": 717,
        "runningSteps": 20,
        "caloriesRaw": 28390,
        "caloriesKcal": 28.39,
        "distanceM": 493,
        "sportDurationS": 1740
      },
      "measurements": [
        {"metric":"heart_rate","ts":"2026-09-16T08:00:00Z","value":77,"unit":"bpm","measurementMode":"automatic"},
        {"metric":"temperature","ts":"2026-09-16T08:30:00Z","value":36.7,"values":{"side":35.2,"side1":35.1},"unit":"celsius","measurementMode":"manual"}
      ],
      "sleepSessions": [
        {
          "type":"main",
          "protocol":"new_sleep_protocol",
          "start":"2026-09-15T15:30:00Z",
          "end":"2026-09-15T23:00:00Z",
          "segments":[
            {"durationS":3600,"stageCode":3},
            {"durationS":5400,"stageCode":2},
            {"durationS":1200,"stageCode":4},
            {"durationS":300,"stageCode":5}
          ]
        }
      ]
    }
  ]
}
```

Typical response:

```json
{
  "status": "synced",
  "syncId": "68ca00000000000000000001",
  "device": {
    "id": "68ca00000000000000000002",
    "deviceUid": "AA:BB:CC:DD:EE:FF",
    "vendor": "QRing",
    "model": "G69",
    "batteryPercent": 82,
    "charging": false
  },
  "days": [
    {
      "date": "2026-09-15",
      "upserted": {
        "dailyActivity": 1,
        "activityBuckets": 1,
        "measurements": 3,
        "targets": 1,
        "syncMetadata": 1
      }
    }
  ],
  "upserted": {
    "deviceState": 1,
    "dailyActivity": 2,
    "activityBuckets": 1,
    "measurements": 6,
    "sleepSessions": 1,
    "sleepSegments": 4,
    "sleepSummaries": 1,
    "targets": 1,
    "events": 1,
    "syncMetadata": 2
  }
}
```

Duplicate immutable points return an inserted count of `0` rather than creating duplicates.

---

# Readback

By stable device UID:

```bash
curl "$BASE/v3/wearable/range?device_uid=AA%3ABB%3ACC%3ADD%3AEE%3AFF&from=2026-09-01&to=2026-09-16"
```

By user:

```bash
curl "$BASE/v3/users/$USER_ID/wearable/range?from=2026-09-01&to=2026-09-16"
```

The response is grouped by explicit device-local `date` and contains whichever normalized domains exist for that date:

```json
{
  "device": {},
  "from": "2026-09-01",
  "to": "2026-09-16",
  "days": [
    {
      "date": "2026-09-16",
      "activity": {},
      "activityBuckets": [],
      "measurements": [],
      "sleepSummary": {},
      "sleepSessions": [],
      "targets": [],
      "workouts": [],
      "events": []
    }
  ]
}
```

Dates and timestamps in API responses are camelCase/RFC3339. Arbitrary payload/value/channel keys are preserved rather than rewritten.

---

# Data retention

Defaults are implemented by `db-kala-agem`:

| Data | Collection | Default retention |
|---|---|---:|
| Activity buckets | `steps_15m` | 365 days |
| Health measurements | `health_metrics` | 365 days |
| Sleep stage timeline | `sleep_segments` | 365 days |
| Device events | `device_events` | 90 days |
| Raw PPG/accelerometer samples | `raw_sensor_samples` | 30 days |
| Idempotency keys | corresponding `*_keys` | measurement retention + 7 days |
| Daily summaries | `daily_activity`, `sleep_summary` | no TTL |
| Sleep sessions | `sleep_sessions` | no TTL |
| Workouts | `workouts` | no TTL |
| Targets | `total_activities` | no TTL |
| Device/pairing identity | `devices`, `user_devices` | no TTL |

Raw high-frequency data must not be used as the only source for long-term UI history; normalized measurements/summaries have longer retention.

---

# Idempotency

MongoDB time-series collections cannot use the same unique-index semantics as regular collections. v3 therefore uses regular key collections:

```text
steps_15m_keys
health_metric_v3_keys
sleep_segment_v3_keys
device_event_v3_keys
raw_sensor_sample_keys
```

Key semantics:

- activity bucket: `device_id + ts_utc`
- measurement: `device_id + metric + ts_utc + measurement_mode + session_id + source`
- sleep segment: `device_id + session_id + start_utc`
- device event: `device_id + event_type + ts_utc + session_id + source`
- raw sample: `device_id + session_id + ts_utc + sample_index + kind`

Daily activity, sleep sessions, workouts, targets, and sync metadata are mutable regular collections and use upserts.

---

# Time and date rules

1. Persist actual instants as UTC BSON `Date`.
2. Persist device-local grouping date as `YYYY-MM-DD`.
3. Persist the UTC offset used during sync as `tz_offset_min`.
4. Activity `timeIndex` is interpreted in the device-local day, not UTC.
5. Daily queries group using `device_date` / `sleep_date`, not `$dateTrunc(... UTC)`.
6. Sleep sessions are allowed to cross midnight.

Example Jakarta (`tzOffsetMin=420`):

```text
local 2026-09-17 00:00 WIB
= UTC 2026-09-16 17:00Z
```

The stored device date remains `2026-09-17`.

---

# SDK-to-v3 mapping checklist

| QRing SDK model/data | v3 payload | MongoDB |
|---|---|---|
| `BleStepTotal` | `day.activity` | `daily_activity` |
| `BleStepDetails` | `day.activityBuckets[]` | `steps_15m` |
| `ReadHeartRateRsp` | `day.measurements[]` (`heart_rate`) | `health_metrics` |
| `BloodOxygenEntity` | `measurements[]` (`spo2`, min/max values) | `health_metrics` |
| `BpDataEntity` / `BlePressure` | `measurements[]` (`blood_pressure`) | `health_metrics` |
| `PressureRsp` | `measurements[]` (`stress`) | `health_metrics` |
| `HRVRsp` | `measurements[]` (`hrv`) | `health_metrics` |
| `TemperatureEntity` / `TemperatureOnceEntity` | `measurements[]` (`temperature`, side channels) | `health_metrics` |
| manual / one-click response | `measurements[]` + shared `sessionId` | `health_metrics` |
| `SleepDisplay` | `sleepSessions[]` (`protocol=legacy`) | `sleep_sessions`, `sleep_segments` |
| `SleepNewProtoResp` | `sleepSessions[]` (`main` + `nap`) | `sleep_sessions`, `sleep_segments` |
| `LongSitResp` | `events[]` (`sedentary`) | `device_events` |
| `SportPlusEntity` | `workouts[]` | `workouts` |
| `BatteryRsp` | `deviceState` | `devices` + battery metric/event |
| `SetTimeRsp` / `DeviceSupportFunctionRsp` | `device.capabilities` | `devices.capabilities` |
| raw PPG/accelerometer values | `rawSamples[]` | `raw_sensor_samples` |

Female-cycle support and AI health-report presentation are product features, but the supplied SDK material does not expose enough canonical payload fields to create a safe dedicated server schema. They should be added only when the frontend has an explicit source payload contract; do not invent medical/cycle fields from capability flags alone.

---

# Validation / error semantics

- `400`: invalid wearable payload/date/timestamp/required fields.
- `404`: user, device, or active pairing not found.
- `409`: device already has another active owner.
- `413`: v3 request exceeds 8 MiB.
- `500`: internal error. Internal MongoDB details are intentionally not returned to clients.

---

# Database initialization

AGEM starts with:

```go
st.EnsureIndexes(ctx)
st.EnsureSDKV3Schema(ctx)
```

The companion `db-kala-agem` repository also creates the same v3 schema and is the deployment-level source for retention/housekeeping settings.

If an existing deployment contains duplicate active pairings, creation of the new partial unique ownership indexes will fail. Clean duplicate active `user_devices` records before deploying the v3 schema.

---

# Notes on authentication

This repository currently focuses on wearable/domain storage and does not add application authentication middleware. In production, protect v3 behind the platform IAM/API gateway or add authenticated user/device middleware before exposing these routes publicly. Pairing and health-data reads must be authorized to the caller, not only validated by path IDs.
