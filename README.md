# agem API

Backend API for AGEM wearable device data. 

This README focuses on API usage. The service stores data in MongoDB and keeps
public IDs as strings. Mongo `_id` values are returned as hex string `id`
fields.

Canonical user identity can come from Regene `users`. When `NASABAH_REGENE` or
`REGENE_MONGODB_URI` is configured, AGEM reads users from the Regene database
and keeps wearable devices, pairings, and measurements in `db-kala-agem`.
The local AGEM `users` collection remains as a fallback for local development
and older records.

Additional API artifacts:

| File | Purposes |
| --- | --- |
| [`docs/API.md`](docs/API.md) | Human-readable route reference. |
| [`docs/openapi.yaml`](docs/openapi.yaml) | OpenAPI 3.0.3 specification for Swagger UI, Postman, Insomnia, or client generation. |

## Quick Start

Run locally with a MongoDB URI:

```sh
ADDR=:8080 \
MONGODB_URI='mongodb://user:password@localhost:27017/regene_kalaagem?authSource=admin' \
go run ./cmd/api
```

Set a reusable base URL for examples:

```sh
BASE=http://localhost:8080
```

Health check:

```sh
curl "$BASE/healthz"
```

Response:

```json
{"status":"ok"}
```

## API Conventions

- All request and response bodies are JSON.
- Unknown JSON fields are rejected on `/v1`; `/v3/wearable/sync` ignores
  unknown future fields and stores recognized sections.
- Timestamps use RFC3339, for example `2026-05-19T08:00:00Z`.
- Device-local dates use `YYYY-MM-DD`.
- Error responses use `{"error":"message"}`.
- Empty delete responses use `{"status":"deleted"}`.

## Typical Usage Flow

1. Create a user.
2. Create a wearable device.
3. Pair the device to the user.
4. Ingest wearable data for the device.
5. Read back activity, sensor metrics, sleep, events, and workouts by date or time range.

## Wearable v3 Sync

Use this endpoint for the QRing/frontend daily sync payload. It stores the
recognized nested payload in `wearable_syncs`, fans summary sensor sections and
timestamped readings into `health_metrics`, writes daily totals into
`daily_activity`, maps `activity.readings` into `steps_15m` including
`activeTime` as `sport_duration_s`, writes sleep totals into `sleep_summary`,
maps `sleep.segments` into `sleep_segments`, and stores target progress rows in
`total_activities`.

| Method | Route | Description |
| --- | --- | --- |
| `POST` | `/v3/wearable/sync` | Sync one QRing-style daily wearable snapshot. |
| `GET` | `/v3/wearable/range?device_uid=...&from=YYYY-MM-DD&to=YYYY-MM-DD` | List synced snapshots by inclusive date range. |
| `GET` | `/v3/users/{user_id}/wearable/range?from=YYYY-MM-DD&to=YYYY-MM-DD` | List synced snapshots grouped by the user's paired devices. |

`deviceUid` is required in the JSON body. The backend also accepts
`device_uid` as an alias. If the device UID does not exist yet, the backend
creates a minimal device with `vendor: "QRing"`.
Use ISO/RFC3339 timestamps, including millisecond precision when available.
`tzOffsetMin` is optional and defaults to `0`; it is used for deriving
`device_date` and 15-minute `time_index` from `activity.readings`.

```sh
curl -s -X POST "$BASE/v3/wearable/sync" \
  -H 'Content-Type: application/json' \
  -d '{
    "deviceUid": "qring-aa-bb-cc-01",
    "tzOffsetMin": 420,
    "hrv": {"current": 42, "average": 45, "readingsCount": 2, "lastUpdated": "2026-07-27T23:44:10.512Z", "readings": [{"ts": "2026-07-27T23:43:10.123Z", "value": 41}, {"ts": "2026-07-27T23:44:10.512Z", "value": 42}]},
    "heartRate": {"current": 77, "average": 74, "min": 58, "max": 112, "readingsCount": 2, "lastUpdated": "2026-07-27T23:44:10.512Z", "readings": [{"ts": "2026-07-27T23:43:10.123Z", "value": 76}, {"ts": "2026-07-27T23:44:10.512Z", "value": 77}]},
    "spo2": {"current": 97, "average": 96, "readingsCount": 1, "lastUpdated": "2026-07-27T23:44:10.512Z", "readings": [{"ts": "2026-07-27T23:44:10.512Z", "value": 97}]},
    "temperature": {"current": 36.7, "average": 36.5, "raw": 36.72, "readingsCount": 1, "lastUpdated": "2026-07-27T23:44:10.512Z", "readings": [{"ts": "2026-07-27T23:44:10.512Z", "value": 36.7, "raw": 36.72}]},
    "stress": {"current": 30, "average": 34, "readingsCount": 1, "lastUpdated": "2026-07-27T23:44:10.512Z", "readings": [{"ts": "2026-07-27T23:44:10.512Z", "value": 30}]},
    "activity": {"steps": 717, "calories": 28390, "distance": 493, "activeTime": 29, "lastUpdated": "2026-07-27T23:44:10.512Z", "readings": [{"ts": "2026-07-27T08:00:00.000Z", "steps": 120, "calories": 8, "distance": 95, "activeTime": 15}]},
    "sleep": {"totalMinutes": 431, "deepMinutes": 92, "lightMinutes": 268, "remMinutes": 61, "awakeMinutes": 10, "score": 78, "sleepStart": "2026-07-26T22:41:00.000Z", "sleepEnd": "2026-07-27T05:52:00.000Z", "lastUpdated": "2026-07-27T23:44:10.512Z", "stageData": [2, 2, 3, 3, 1, 1, 1, 2, 4], "segments": [{"start": "2026-07-26T22:41:00.000Z", "end": "2026-07-26T23:11:00.000Z", "stage": "light"}]},
    "bloodPressure": {"systolic": 118, "diastolic": 76, "heartRate": 72, "measurementTime": "2026-07-27T07:12:00.000Z", "lastUpdated": "2026-07-27T23:44:10.512Z", "readings": [{"ts": "2026-07-27T07:12:00.000Z", "systolic": 118, "diastolic": 76, "heartRate": 72}]},
    "totalActivities": [
      {"kind": "steps", "value": 717, "target": 4000},
      {"kind": "calories", "value": 28.39, "target": 500},
      {"kind": "distance", "value": 0.493, "target": 4}
    ]
  }'
```

Response:

```json
{
  "status": "synced",
  "device_id": "66a000000000000000000001",
  "device_uid": "qring-aa-bb-cc-01",
  "sync_date": "2026-07-27",
  "upserted": {
    "snapshot": 1,
    "health_metrics": 14,
    "daily_activity": 1,
    "steps_15m": 1,
    "sleep_summary": 1,
    "sleep_segments": 1,
    "total_activities": 3
  }
}
```

Read back snapshots:

```sh
curl -s "$BASE/v3/wearable/range?device_uid=qring-aa-bb-cc-01&from=2026-07-01&to=2026-07-27"
```

Read back snapshots for every device paired to a user:

```sh
curl -s "$BASE/v3/users/66a000000000000000000100/wearable/range?from=2026-07-01&to=2026-07-27"
```

Add `include_readings=true` to return the raw `readings` and `segments` arrays
inside each daily snapshot. This works for both range endpoints:

```sh
curl -s "$BASE/v3/wearable/range?device_uid=qring-aa-bb-cc-01&from=2026-07-01&to=2026-07-27&include_readings=true"
```

## Users

| Method | Route | Description |
| --- | --- | --- |
| `POST` | `/v1/users/` | Create user. |
| `GET` | `/v1/users/{id}` | Get user by ID from Regene when configured, otherwise local AGEM users. |
| `PATCH` | `/v1/users/{id}` | Update local AGEM fallback user. |
| `DELETE` | `/v1/users/{id}` | Delete local AGEM fallback user. |

Create local fallback user:

```sh
curl -s -X POST "$BASE/v1/users/" \
  -H 'Content-Type: application/json' \
  -d '{"email":"patient@example.com","phone":"+628123456789"}'
```

Response:

```json
{
  "id": "665d6f4a7f4a6d0e8f3c0001",
  "email": "patient@example.com",
  "phone": "+628123456789",
  "created_at": "2026-05-19T08:00:00Z",
  "updated_at": "2026-05-19T08:00:00Z"
}
```

Update user:

```sh
curl -s -X PATCH "$BASE/v1/users/$USER_ID" \
  -H 'Content-Type: application/json' \
  -d '{"phone":"+628987654321"}'
```

## Devices

| Method | Route | Description |
| --- | --- | --- |
| `POST` | `/v1/devices/` | Create device. |
| `GET` | `/v1/devices/{id}` | Get device by ID. |
| `GET` | `/v1/devices/by_uid/{device_uid}` | Get device by stable device UID. |
| `PATCH` | `/v1/devices/{id}` | Update device metadata. |
| `DELETE` | `/v1/devices/{id}` | Delete device. |

Create device:

```sh
curl -s -X POST "$BASE/v1/devices/" \
  -H 'Content-Type: application/json' \
  -d '{
    "vendor": "agem",
    "model": "agem-band",
    "hw_rev": "1.0",
    "fw_rev": "1.0.0",
    "serial_number": "SN-AGEM-001",
    "device_uid": "AGEM-MAC-001"
  }'
```

Required fields:

| Field | Notes |
| --- | --- |
| `vendor` | Required. |
| `device_uid` | Required and unique. Use the stable wearable identifier, such as remote ID or MAC address. |

Update device:

```sh
curl -s -X PATCH "$BASE/v1/devices/$DEVICE_ID" \
  -H 'Content-Type: application/json' \
  -d '{"fw_rev":"1.0.1","last_seen_at":"2026-05-19T08:15:00Z"}'
```

## User Device Pairing

| Method | Route | Description |
| --- | --- | --- |
| `POST` | `/v1/users/{user_id}/devices` | Pair a device to a user. |
| `GET` | `/v1/users/{user_id}/devices` | List paired devices for a user. |
| `PATCH` | `/v1/user_devices/{id}` | Update pairing metadata. |
| `DELETE` | `/v1/user_devices/{id}` | Delete pairing record. |

`user_id` must exist in Regene `users` when `NASABAH_REGENE` is configured.
For test, `62d10918cf0e3f8821001f7d` is the `tester@regene.id` user.

Pair device to user:

```sh
curl -s -X POST "$BASE/v1/users/$USER_ID/devices" \
  -H 'Content-Type: application/json' \
  -d '{
    "device_id": "'$DEVICE_ID'",
    "nickname": "daily band",
    "is_primary": true
  }'
```

Update pairing:

```sh
curl -s -X PATCH "$BASE/v1/user_devices/$PAIRING_ID" \
  -H 'Content-Type: application/json' \
  -d '{"nickname":"left wrist","is_primary":true}'
```

Unpair without deleting the record:

```sh
curl -s -X PATCH "$BASE/v1/user_devices/$PAIRING_ID" \
  -H 'Content-Type: application/json' \
  -d '{"unpaired_at":"2026-05-19T09:00:00Z"}'
```

## Steps

| Method | Route | Description |
| --- | --- | --- |
| `POST` | `/v1/devices/{device_id}/steps15m` | Idempotent ingest for 15-minute step points. |
| `GET` | `/v1/devices/{device_id}/steps15m?date=YYYY-MM-DD` | List 15-minute points for one device-local date. |
| `GET` | `/v1/devices/{device_id}/steps/daily?from=RFC3339&to=RFC3339` | Aggregate daily step totals from the `steps_15m` time series collection. |

Ingest 15-minute points:

```sh
curl -s -X POST "$BASE/v1/devices/$DEVICE_ID/steps15m" \
  -H 'Content-Type: application/json' \
  -d '{
    "points": [
      {
        "ts_utc": "2026-05-19T00:00:00Z",
        "device_date": "2026-05-19",
        "time_index": 0,
        "tz_offset_min": 420,
        "walk_steps": 120,
        "run_steps": 0,
        "calories": 4,
        "distance_m": 80,
        "sport_duration_s": 600,
        "source": "device"
      }
    ]
  }'
```

Response:

```json
{"upserted":1}
```

`upserted` is the number of new measurements inserted into the time series
collection. Duplicate points with the same `device_id + ts_utc` are skipped by
the backend key collection.

List points by date:

```sh
curl -s "$BASE/v1/devices/$DEVICE_ID/steps15m?date=2026-05-19"
```

Get daily totals:

```sh
curl -s "$BASE/v1/devices/$DEVICE_ID/steps/daily?from=2026-05-19T00:00:00Z&to=2026-05-20T00:00:00Z"
```

Daily total response fields:

| Field | Description |
| --- | --- |
| `day_utc` | UTC day bucket. |
| `total_steps` | `walk_steps + run_steps`. |
| `running_steps` | Sum of `run_steps`. |
| `calories` | Sum of calories. |
| `distance_m` | Sum of distance. |
| `last_synced_at` | Latest sync time in the bucket. |

## Daily Activity

Use this for the qring SDK daily total object (`BleStepTotal` /
`TodaySportDataRsp`), including fields that are not present in 15-minute step
details.

| Method | Route | Description |
| --- | --- | --- |
| `POST` | `/v1/devices/{device_id}/activity/daily` | Upsert one device-local daily activity total. |
| `GET` | `/v1/devices/{device_id}/activity/daily?from=YYYY-MM-DD&to=YYYY-MM-DD` | List daily totals by device-local date range. |

```sh
curl -s -X POST "$BASE/v1/devices/$DEVICE_ID/activity/daily" \
  -H 'Content-Type: application/json' \
  -d '{
    "device_date": "2026-05-19",
    "tz_offset_min": 420,
    "total_steps": 6200,
    "running_steps": 900,
    "calories": 240,
    "walk_distance_m": 4300,
    "sport_duration_s": 1800,
    "sleep_duration_s": 27000,
    "source": "device"
  }'
```

## Health Metrics

Use this generic time-series route for qring sensor data that is not already
modeled by steps, sleep, or workouts: heart rate, SpO2, blood pressure, HRV,
stress/pressure, temperature, RRI, raw PPG samples, and similar measurements.

| Method | Route | Description |
| --- | --- | --- |
| `POST` | `/v1/devices/{device_id}/health/metrics` | Idempotent ingest for health sensor points. |
| `GET` | `/v1/devices/{device_id}/health/metrics?from=RFC3339&to=RFC3339&metric=heart_rate` | List health sensor points. `metric` is optional. |

Scalar point example:

```sh
curl -s -X POST "$BASE/v1/devices/$DEVICE_ID/health/metrics" \
  -H 'Content-Type: application/json' \
  -d '{
    "points": [
      {
        "ts_utc": "2026-05-19T08:00:00Z",
        "metric": "heart_rate",
        "value": 78,
        "unit": "bpm",
        "source": "device_auto"
      }
    ]
  }'
```

Multi-value point example for blood pressure or raw SDK packets:

```json
{
  "ts_utc": "2026-05-19T08:00:00Z",
  "metric": "blood_pressure",
  "values": {"sbp": 118, "dbp": 76},
  "unit": "mmHg",
  "source": "device_manual",
  "metadata": {"sdk_class": "StopHeartRateRsp"}
}
```

Recommended metric names:

```text
heart_rate, blood_oxygen, blood_pressure, hrv, stress, temperature,
rri, raw_ppg, battery_level
```

Duplicate points with the same `device_id + metric + ts_utc + source` are
skipped by the backend key collection.

## Device Events

Use this route for qring SDK notifications and raw events that are not direct
health measurements, such as battery notifications, touch events, sedentary
events, and vendor payloads from `DeviceNotifyRsp`.

| Method | Route | Description |
| --- | --- | --- |
| `POST` | `/v1/devices/{device_id}/events` | Idempotent ingest for device events. |
| `GET` | `/v1/devices/{device_id}/events?from=RFC3339&to=RFC3339&event_type=battery` | List device events. `event_type` is optional. |

```sh
curl -s -X POST "$BASE/v1/devices/$DEVICE_ID/events" \
  -H 'Content-Type: application/json' \
  -d '{
    "events": [
      {
        "ts_utc": "2026-05-19T08:00:00Z",
        "event_type": "battery",
        "source": "device_notify",
        "payload": {"battery_level": 82, "charging": false}
      }
    ]
  }'
```

## Sleep

| Method | Route | Description |
| --- | --- | --- |
| `POST` | `/v1/devices/{device_id}/sleep/summary` | Upsert nightly sleep summary. |
| `GET` | `/v1/devices/{device_id}/sleep/summary?date=YYYY-MM-DD` | Get sleep summary by sleep date. |
| `POST` | `/v1/devices/{device_id}/sleep/segments` | Idempotent ingest for sleep stage segments. |
| `GET` | `/v1/devices/{device_id}/sleep/segments?date=YYYY-MM-DD` | List sleep segments from the `sleep_segments` time series collection by sleep date. |

Upsert sleep summary:

```sh
curl -s -X POST "$BASE/v1/devices/$DEVICE_ID/sleep/summary" \
  -H 'Content-Type: application/json' \
  -d '{
    "sleep_date": "2026-05-18",
    "tz_offset_min": 420,
    "sleep_start_utc": "2026-05-18T15:00:00Z",
    "wake_utc": "2026-05-18T23:00:00Z",
    "total_sleep_s": 28800,
    "deep_s": 5400,
    "light_s": 16200,
    "awake_s": 1200,
    "rem_s": 6000,
    "source": "device"
  }'
```

Optional fields:

| Field | Notes |
| --- | --- |
| `sleep_start_utc` | Optional RFC3339. |
| `wake_utc` | Optional RFC3339. |

Get sleep summary:

```sh
curl -s "$BASE/v1/devices/$DEVICE_ID/sleep/summary?date=2026-05-18"
```

Ingest sleep segments:

```sh
curl -s -X POST "$BASE/v1/devices/$DEVICE_ID/sleep/segments" \
  -H 'Content-Type: application/json' \
  -d '{
    "segments": [
      {
        "start_utc": "2026-05-18T15:00:00Z",
        "end_utc": "2026-05-18T15:30:00Z",
        "sleep_date": "2026-05-18",
        "tz_offset_min": 420,
        "stage": "light",
        "source": "device"
      }
    ]
  }'
```

Supported stage values by convention:

```text
deep, light, rem, awake, off_wrist, unknown
```

List sleep segments:

```sh
curl -s "$BASE/v1/devices/$DEVICE_ID/sleep/segments?date=2026-05-18"
```

Duplicate segments with the same `device_id + start_utc` are skipped by the
backend key collection.

## Workouts

| Method | Route | Description |
| --- | --- | --- |
| `POST` | `/v1/devices/{device_id}/workouts` | Create or upsert workout by `device_id + start_utc`. |
| `GET` | `/v1/devices/{device_id}/workouts?from=RFC3339&to=RFC3339` | List workouts in a time range. |
| `PATCH` | `/v1/devices/{device_id}/workouts` | Update workout identified by `start_utc` in body. |
| `DELETE` | `/v1/devices/{device_id}/workouts?start_utc=RFC3339` | Delete workout by `start_utc`. |

Create workout:

```sh
curl -s -X POST "$BASE/v1/devices/$DEVICE_ID/workouts" \
  -H 'Content-Type: application/json' \
  -d '{
    "start_utc": "2026-05-19T01:00:00Z",
    "end_utc": "2026-05-19T01:30:00Z",
    "tz_offset_min": 420,
    "sport_type_id": 1,
    "duration_s": 1800,
    "distance_m": 2500,
    "calories": 150.5,
    "avg_hr": 112,
    "max_hr": 145,
    "min_hr": 80,
    "avg_speed_cm_s": 140,
    "max_speed_cm_s": 220,
    "elevation_cm": 5000,
    "uphill_cm": 1200,
    "downhill_cm": 800,
    "avg_cadence_spm": 92,
    "sport_count": 1,
    "steps": 3400,
    "locations": [{"rate_real": 115}],
    "source": "device"
  }'
```

List workouts:

```sh
curl -s "$BASE/v1/devices/$DEVICE_ID/workouts?from=2026-05-19T00:00:00Z&to=2026-05-20T00:00:00Z"
```

Patch workout:

```sh
curl -s -X PATCH "$BASE/v1/devices/$DEVICE_ID/workouts" \
  -H 'Content-Type: application/json' \
  -d '{
    "start_utc": "2026-05-19T01:00:00Z",
    "duration_s": 1900,
    "calories": 160,
    "source": "device"
  }'
```

Delete workout:

```sh
curl -s -X DELETE "$BASE/v1/devices/$DEVICE_ID/workouts?start_utc=2026-05-19T01:00:00Z"
```

## Upsert Keys

Mongo collections and indexes are checked on startup. `steps_15m` and
`sleep_segments` must be MongoDB time series collections.

| Data | Collection type | Upsert or uniqueness key |
| --- | --- | --- |
| Devices | Regular | `device_uid` |
| Steps | Time series plus `steps_15m_keys` | `device_id + ts_utc` |
| Daily activity | Regular | `device_id + device_date` |
| Health metrics | Time series plus `health_metric_keys` | `device_id + metric + ts_utc + source` |
| Device events | Time series plus `device_event_keys` | `device_id + event_type + ts_utc + source` |
| Sleep summary | Regular | `device_id + sleep_date` |
| Sleep segments | Time series plus `sleep_segments_keys` | `device_id + start_utc` |
| Wearable sync snapshots | Regular | `device_uid + sync_date` |
| Total activity targets | Regular | `device_id + device_date + kind` |
| Workouts | Regular | `device_id + start_utc` |

## Retention

Raw time-series measurements use MongoDB retention. Fresh collections created
by the backend use these defaults:

| Data | Collections | Retention |
| --- | --- | --- |
| Raw wearable measurements | `steps_15m`, `sleep_segments`, `health_metrics` | 365 days |
| Raw device events | `device_events` | 90 days |
| Idempotency keys | `*_keys` collections | matching raw retention plus 7 days |
| Daily summaries and snapshots | `daily_activity`, `sleep_summary`, `total_activities`, `wearable_syncs` | no TTL |
| Raw arrays inside snapshots | `wearable_syncs.payload.*.readings`, `wearable_syncs.payload.sleep.segments` | pruned after 180 days |

Existing MongoDB time-series retention is applied by the `db-kala-agem`
deployment helper, because AGEM connects with the application `readWrite` user.

## Minimal Environment Reference

For API testing, the only required runtime values are the listen address and a
Mongo connection:

```sh
ADDR=:8080
MONGODB_URI=mongodb://user:password@host:57215/regene_kalaagem?authSource=admin
```

Docker secret files may contain either a raw Mongo URI or:

```text
MONGODB_URI=mongodb://user:password@host:57215/regene_kalaagem?authSource=admin
```

The backend also accepts the legacy `/etc/app2` secret-file format:

```text
hosts=host:57215
user=mongo_user
pass=mongo_password
db=regene_kalaagem
authSource=admin
```

Optional Regene user source:

```text
NASABAH_REGENE=/run/secrets/nasabah_regene_t
```

The Regene secret can use the same `/etc/app2` style, for example:

```text
hosts=talas51.regene.xyz:57018
user=regene_reader
pass=secret
db=regeneNode
authSource=admin
```
