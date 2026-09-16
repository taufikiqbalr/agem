# AGEM QRing API v3

This document is the route-focused companion to the repository README. The canonical QRing frontend contract is v3.

## Base URL

```text
http://localhost:8080
```

## Routes

| Method | Route | Description |
|---|---|---|
| `GET` | `/healthz` | Service health. |
| `POST` | `/v3/wearable/sync` | Multi-day QRing SDK synchronization. |
| `GET` | `/v3/wearable/range` | Read normalized wearable data by `device_uid`. |
| `GET` | `/v3/devices/by_uid/{device_uid}` | Get current device metadata/state/capabilities. |
| `GET` | `/v3/devices/{device_id}/wearable/range` | Read normalized wearable data by Mongo device ID. |
| `POST` | `/v3/users/{user_id}/devices` | Pair a registered device by UID. |
| `GET` | `/v3/users/{user_id}/devices` | List active pairings. |
| `PATCH` | `/v3/users/{user_id}/devices/{device_uid}` | Update nickname/primary flag. |
| `DELETE` | `/v3/users/{user_id}/devices/{device_uid}` | Soft-unpair. |
| `GET` | `/v3/users/{user_id}/wearable/range` | Read all active devices for the user. |

`/v1` remains available for legacy clients, but new frontend integration should use v3.

## Sync request

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
      "date": "2026-09-16",
      "activity": {
        "totalSteps": 717,
        "runningSteps": 20,
        "caloriesRaw": 28390,
        "caloriesKcal": 28.39,
        "distanceM": 493,
        "sportDurationS": 1740,
        "sleepDurationS": 25860
      },
      "activityBuckets": [
        {
          "timeIndex": 32,
          "walkSteps": 110,
          "runSteps": 10,
          "caloriesRaw": 8000,
          "caloriesKcal": 8,
          "distanceM": 95
        }
      ],
      "measurements": [
        {
          "metric": "heart_rate",
          "minuteOfDay": 480,
          "value": 77,
          "unit": "bpm",
          "measurementMode": "history_sync",
          "sampleIntervalSec": 300
        },
        {
          "metric": "spo2",
          "minuteOfDay": 480,
          "values": {"min": 95, "max": 98},
          "unit": "%",
          "measurementMode": "history_sync",
          "sampleIntervalSec": 3600
        }
      ],
      "sleepSessions": [
        {
          "type": "main",
          "protocol": "new_sleep_protocol",
          "start": "2026-09-15T15:30:00Z",
          "end": "2026-09-15T23:00:00Z",
          "segments": [
            {"durationS": 3600, "stageCode": 3},
            {"durationS": 5400, "stageCode": 2},
            {"durationS": 1200, "stageCode": 4},
            {"durationS": 300, "stageCode": 5}
          ]
        }
      ],
      "targets": [
        {"kind": "steps", "value": 717, "target": 4000, "unit": "steps"}
      ]
    }
  ]
}
```

## Time semantics

- `days[].date` is always device-local `YYYY-MM-DD`.
- `tzOffsetMin` is the UTC offset used to interpret local dates and `minuteOfDay` / `timeIndex`.
- Measurement/event time can be supplied as RFC3339 `ts` or `minuteOfDay`.
- `activityBuckets[].timeIndex` is `0..95`, each bucket representing 15 minutes.
- All actual timestamps are stored in UTC.
- Sleep sessions may cross midnight.

## Measurements

Normalized measurement shape:

```json
{
  "metric": "temperature",
  "ts": "2026-09-16T08:30:00Z",
  "value": 36.7,
  "values": {"side": 35.2, "side1": 35.1},
  "rawValue": 367,
  "unit": "celsius",
  "measurementMode": "manual",
  "sessionId": "measure-001",
  "sampleIntervalSec": 60,
  "errorCode": 0,
  "derived": false,
  "metadata": {}
}
```

Use `measurementMode` to separate automatic/history/manual/one-click/realtime/derived values. Use `sessionId` to correlate results produced by one measurement operation.

## Sleep stage codes

New sleep protocol:

```text
0 not_sleeping
1 off_wrist
2 light
3 deep
4 rem
5 awake
```

Legacy protocol:

```text
1 deep
2 light
3 awake
```

## Read range

```bash
curl "$BASE/v3/wearable/range?device_uid=AA%3ABB%3ACC%3ADD%3AEE%3AFF&from=2026-09-01&to=2026-09-16"
```

Set `include_raw=true` only when raw diagnostic samples are needed.

## Pairing

```bash
curl -X POST "$BASE/v3/users/$USER_ID/devices" \
  -H 'Content-Type: application/json' \
  -d '{"deviceUid":"AA:BB:CC:DD:EE:FF","nickname":"G69","isPrimary":true}'
```

A device can have only one active owner. A user can have only one active primary device. Unpairing preserves history.

## Status codes

| Status | Meaning |
|---:|---|
| 200 | successful read/sync/update |
| 201 | pairing created/upserted |
| 400 | invalid payload/date/time/field |
| 404 | user/device/pairing not found |
| 409 | device already has another active owner |
| 413 | sync body exceeds 8 MiB |
| 500 | internal error (details are not leaked to the client) |
