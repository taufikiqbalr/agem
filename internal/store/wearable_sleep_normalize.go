package store

import (
	"fmt"
	"strings"
	"time"
)

func normalizeSleepSegments(sessionStart time.Time, explicitEnd, protocol string, segments []WearableSleepSegment) ([]normalizedSleepSegment, derivedSleepTotals, time.Time, error) {
	out := make([]normalizedSleepSegment, 0, len(segments))
	derived := derivedSleepTotals{}
	cursor := sessionStart.UTC()
	var lastEnd time.Time
	wasAwake := false

	for i, segment := range segments {
		start := cursor
		if strings.TrimSpace(segment.Start) != "" {
			parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(segment.Start))
			if err != nil {
				return nil, derived, time.Time{}, wearablePayloadError(fmt.Sprintf("invalid sleep segment %d start", i))
			}
			start = parsed.UTC()
		}
		var end time.Time
		if strings.TrimSpace(segment.End) != "" {
			parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(segment.End))
			if err != nil {
				return nil, derived, time.Time{}, wearablePayloadError(fmt.Sprintf("invalid sleep segment %d end", i))
			}
			end = parsed.UTC()
		} else if segment.DurationS != nil && *segment.DurationS >= 0 {
			end = start.Add(time.Duration(*segment.DurationS) * time.Second)
		} else {
			return nil, derived, time.Time{}, wearablePayloadError(fmt.Sprintf("sleep segment %d requires end or durationS", i))
		}
		if end.Before(start) {
			return nil, derived, time.Time{}, wearablePayloadError(fmt.Sprintf("sleep segment %d end is before start", i))
		}
		stage := normalizeSleepStage(protocol, segment.StageCode, segment.Stage)
		duration := int(end.Sub(start).Seconds())
		switch stage {
		case "deep":
			derived.deepS += duration
			derived.totalSleepS += duration
			wasAwake = false
		case "light":
			derived.lightS += duration
			derived.totalSleepS += duration
			wasAwake = false
		case "rem":
			derived.remS += duration
			derived.totalSleepS += duration
			wasAwake = false
		case "awake":
			derived.awakeS += duration
			if !wasAwake {
				derived.wakingCount++
			}
			wasAwake = true
		default:
			wasAwake = false
		}
		out = append(out, normalizedSleepSegment{start: start, end: end, stage: stage, stageCode: segment.StageCode})
		cursor = end
		lastEnd = end
	}

	if strings.TrimSpace(explicitEnd) != "" {
		parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(explicitEnd))
		if err != nil {
			return nil, derived, time.Time{}, wearablePayloadError("invalid sleep session end")
		}
		lastEnd = parsed.UTC()
		if lastEnd.Before(sessionStart) {
			return nil, derived, time.Time{}, wearablePayloadError("sleep session end is before start")
		}
	}
	return out, derived, lastEnd, nil
}

func normalizeSleepStage(protocol string, code *int, explicit string) string {
	if value := sanitizeName(explicit); value != "" {
		switch value {
		case "rapid", "rapid_eye_movement", "eye_movement":
			return "rem"
		case "shallow":
			return "light"
		default:
			return value
		}
	}
	if code == nil {
		return "unknown"
	}
	if protocol == "new" || protocol == "new_sleep" || protocol == "new_sleep_protocol" {
		switch *code {
		case 0:
			return "not_sleeping"
		case 1:
			return "off_wrist"
		case 2:
			return "light"
		case 3:
			return "deep"
		case 4:
			return "rem"
		case 5:
			return "awake"
		}
	} else {
		switch *code {
		case 1:
			return "deep"
		case 2:
			return "light"
		case 3:
			return "awake"
		}
	}
	return fmt.Sprintf("unknown_%d", *code)
}
