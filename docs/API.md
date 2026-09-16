# AGEM Backend API

This document is the human-readable API reference for the AGEM wearable backend.
The machine-readable OpenAPI document is in [openapi.yaml](./openapi.yaml).

## Conventions

- Base paths: `/v1` for normalized resources and `/v3` for wearable sync.
- Request and response bodies are JSON.
- Unknown JSON request fields are rejected on `/v1`. The v3 wearable sync
  endpoint ignores unknown future fields and stores only recognized sections.
- UTC timestamps use RFC3339, for example `2026-05-19T08:00:00Z`.
- Device-local dates use `YYYY-MM-DD`.
- Error responses use `{"error":"message"}`.
- Public IDs are strings. MongoDB `_id` values are returned as hex string `id`
  fields.
- User identity is read from Regene `users` when `NASABAH_REGENE` or
  `REGENE_MONGODB_URI` is configured. AGEM stores wearable devices, pairings,
  and readings in `db-kala-agem`.

## Health Check

| Method | Route | Description |
| --- | --- | --- |
| `GET` | `/healthz` | Service health check. |

## Wearable v3

| Method | Route | Description |
| --- | --- | --- |
| `POST` | `/v3/wearable/sync` | Store a QRing-style wearable sync payload. |
| `GET` | `/v3/wearable/range?device_uid=...&from=YYYY-MM-DD&to=YYYY-MM-DD` | List synced daily snapshots in ascending date order. |
| `GET` | `/v3/users/{user_id}/wearable/range?from=YYYY-MM-DD&to=YYYY-MM-DD` | List synced daily snapshots grouped by the user's paired devices. |

`POST /v3/wearable/sync` requires `deviceUid` in the body. `device_uid` is
accepted as an alias. At least one recognized section is required:
`hrv`, `heartRate`, `spo2`, `temperature`, `stress`, `activity`, `sleep`,
`bloodPressure`, or `totalActivities`.
Use ISO/RFC3339 timestamps with millisecond precision when available.
`tzOffsetMin` defaults to `0` and is used for `activity.readings` bucket dates.

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

Range read:

```sh
curl -s "$BASE/v3/wearable/range?device_uid=qring-aa-bb-cc-01&from=2026-07-01&to=2026-07-27"
```

Range read by user:

```sh
curl -s "$BASE/v3/users/66a000000000000000000100/wearable/range?from=2026-07-01&to=2026-07-27"
```

The sync endpoint maps sensor sections into `health_metrics`, maps `activity`
into `daily_activity`, maps `activity.readings` into `steps_15m` including
`activeTime` as `sport_duration_s`, maps `sleep` into `sleep_summary`, maps
`sleep.segments` into `sleep_segments`, stores `totalActivities` in
`total_activities`, and preserves the recognized nested payload in
`wearable_syncs` for range reads. Range responses omit raw `readings` and
`segments` by default; add `include_readings=true` to include them. The
user-level range endpoint returns the same daily snapshot shape under each
paired device.

## Users

| Method | Route | Description |
| --- | --- | --- |
| `POST` | `/v1/users/` | Create a user. |
| `GET` | `/v1/users/{id}` | Get a user by ID from Regene when configured, otherwise local AGEM users. |
| `PATCH` | `/v1/users/{id}` | Update local AGEM fallback user email or phone. |
| `DELETE` | `/v1/users/{id}` | Delete a local AGEM fallback user. |
| `POST` | `/v1/users/{user_id}/devices` | Pair a device to a user. |
| `GET` | `/v1/users/{user_id}/devices` | List user device pairings. |

Create local fallback user:

```sh
curl -s -X POST "$BASE/v1/users/" \
  -H 'Content-Type: application/json' \
  -d '{"email":"patient@example.com","phone":"+628123456789"}'
```

For test Regene-backed calls, `62d10918cf0e3f8821001f7d` is the existing
`tester@regene.id` user.

## Devices

| Method | Route | Description |
| --- | --- | --- |
| `POST` | `/v1/devices/` | Create a wearable device. |
| `GET` | `/v1/devices/{id}` | Get device by API ID. |
| `GET` | `/v1/devices/by_uid/{device_uid}` | Get device by stable wearable UID or MAC. |
| `PATCH` | `/v1/devices/{id}` | Update device metadata. |
| `DELETE` | `/v1/devices/{id}` | Delete device. |

Create device:

```sh
curl -s -X POST "$BASE/v1/devices/" \
  -H 'Content-Type: application/json' \
  -d '{
    "vendor": "qring",
    "model": "G69",
    "hw_rev": "RTL8762E",
    "fw_rev": "1.0.0",
    "device_uid": "AA:BB:CC:DD:EE:FF"
  }'
```

## Pairings

| Method | Route | Description |
| --- | --- | --- |
| `PATCH` | `/v1/user_devices/{id}` | Update pairing nickname, primary flag, or unpaired time. |
| `DELETE` | `/v1/user_devices/{id}` | Delete pairing record. |

Pair device:

```sh
curl -s -X POST "$BASE/v1/users/$USER_ID/devices" \
  -H 'Content-Type: application/json' \
  -d '{"device_id":"'$DEVICE_ID'","nickname":"daily ring","is_primary":true}'
```

## Steps And Daily Activity

| Method | Route | Description |
| --- | --- | --- |
| `POST` | `/v1/devices/{device_id}/steps15m` | Idempotent ingest for 15-minute step points. |
| `GET` | `/v1/devices/{device_id}/steps15m?date=YYYY-MM-DD` | List 15-minute points by device-local date. |
| `GET` | `/v1/devices/{device_id}/steps/daily?from=RFC3339&to=RFC3339` | Aggregate daily step totals from 15-minute points. |
| `POST` | `/v1/devices/{device_id}/activity/daily` | Upsert qring daily total data. |
| `GET` | `/v1/devices/{device_id}/activity/daily?from=YYYY-MM-DD&to=YYYY-MM-DD` | List qring daily totals. |

Use `steps15m` for `BleStepDetails`. Use `activity/daily` for qring
`BleStepTotal`, `TodaySportDataRsp`, or `TotalSportDataRsp`.

## Health Metrics

| Method | Route | Description |
| --- | --- | --- |
| `POST` | `/v1/devices/{device_id}/health/metrics` | Idempotent ingest for qring sensor readings. |
| `GET` | `/v1/devices/{device_id}/health/metrics?from=RFC3339&to=RFC3339&metric=heart_rate` | List sensor readings. `metric` is optional. |

Recommended metric names:

```text
heart_rate, blood_oxygen, blood_pressure, hrv, stress, temperature,
rri, raw_ppg, battery_level
```

Example:

```sh
curl -s -X POST "$BASE/v1/devices/$DEVICE_ID/health/metrics" \
  -H 'Content-Type: application/json' \
  -d '{
    "points": [
      {
        "ts_utc": "2026-05-19T08:00:00Z",
        "metric": "blood_pressure",
        "values": {"sbp": 118, "dbp": 76},
        "unit": "mmHg",
        "source": "device_manual"
      }
    ]
  }'
```

## Device Events

| Method | Route | Description |
| --- | --- | --- |
| `POST` | `/v1/devices/{device_id}/events` | Idempotent ingest for device events and raw SDK notifications. |
| `GET` | `/v1/devices/{device_id}/events?from=RFC3339&to=RFC3339&event_type=battery` | List device events. `event_type` is optional. |

Use this for `DeviceNotifyRsp`, battery notifications, touch events, sedentary
events, or vendor-specific payloads.

## Sleep

| Method | Route | Description |
| --- | --- | --- |
| `POST` | `/v1/devices/{device_id}/sleep/summary` | Upsert nightly sleep summary. |
| `GET` | `/v1/devices/{device_id}/sleep/summary?date=YYYY-MM-DD` | Get sleep summary by sleep date. |
| `POST` | `/v1/devices/{device_id}/sleep/segments` | Idempotent ingest for sleep stage segments. |
| `GET` | `/v1/devices/{device_id}/sleep/segments?date=YYYY-MM-DD` | List sleep segments by sleep date. |

Stage values by convention:

```text
deep, light, rem, awake, off_wrist, unknown
```

## Workouts

| Method | Route | Description |
| --- | --- | --- |
| `POST` | `/v1/devices/{device_id}/workouts` | Create or upsert workout by `device_id + start_utc`. |
| `GET` | `/v1/devices/{device_id}/workouts?from=RFC3339&to=RFC3339` | List workouts. |
| `PATCH` | `/v1/devices/{device_id}/workouts` | Update workout identified by `start_utc` in body. |
| `DELETE` | `/v1/devices/{device_id}/workouts?start_utc=RFC3339` | Delete workout by start time. |

The workout schema includes qring `SportPlusEntity` fields such as speed,
elevation, uphill/downhill, cadence, sport count, steps, and detail heart-rate
samples in `locations`.

## OpenAPI

Import [openapi.yaml](./openapi.yaml) into Swagger UI, Postman, Insomnia, or a
client generator.
