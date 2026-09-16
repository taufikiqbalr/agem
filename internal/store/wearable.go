package store

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	wearableDateLayout            = "2006-01-02"
	wearableSyncSource            = "wearable_sync"
	wearableMetricReadingSource   = "wearable_sync_reading"
	wearableActivityReadingSource = "wearable_sync_activity_reading"
	wearableSleepSegmentSource    = "wearable_sync_sleep_segment"
)

var ErrInvalidWearablePayload = errors.New("invalid wearable payload")

type WearableSyncRequest struct {
	DeviceUID       string                         `json:"deviceUid,omitempty" bson:"-"`
	DeviceUIDAlias  string                         `json:"device_uid,omitempty" bson:"-"`
	TzOffsetMin     *int                           `json:"tzOffsetMin,omitempty" bson:"tzOffsetMin,omitempty"`
	HRV             *WearableMetricSection         `json:"hrv,omitempty" bson:"hrv,omitempty"`
	HeartRate       *WearableMetricSection         `json:"heartRate,omitempty" bson:"heartRate,omitempty"`
	SpO2            *WearableMetricSection         `json:"spo2,omitempty" bson:"spo2,omitempty"`
	Temperature     *WearableMetricSection         `json:"temperature,omitempty" bson:"temperature,omitempty"`
	Stress          *WearableMetricSection         `json:"stress,omitempty" bson:"stress,omitempty"`
	Activity        *WearableActivitySection       `json:"activity,omitempty" bson:"activity,omitempty"`
	Sleep           *WearableSleepSection          `json:"sleep,omitempty" bson:"sleep,omitempty"`
	BloodPressure   *WearableBloodPressureSection  `json:"bloodPressure,omitempty" bson:"bloodPressure,omitempty"`
	TotalActivities []WearableTotalActivitySection `json:"totalActivities,omitempty" bson:"totalActivities,omitempty"`
}

type WearableSnapshotPayload struct {
	TzOffsetMin     *int                           `json:"tzOffsetMin,omitempty" bson:"tzOffsetMin,omitempty"`
	HRV             *WearableMetricSection         `json:"hrv,omitempty" bson:"hrv,omitempty"`
	HeartRate       *WearableMetricSection         `json:"heartRate,omitempty" bson:"heartRate,omitempty"`
	SpO2            *WearableMetricSection         `json:"spo2,omitempty" bson:"spo2,omitempty"`
	Temperature     *WearableMetricSection         `json:"temperature,omitempty" bson:"temperature,omitempty"`
	Stress          *WearableMetricSection         `json:"stress,omitempty" bson:"stress,omitempty"`
	Activity        *WearableActivitySection       `json:"activity,omitempty" bson:"activity,omitempty"`
	Sleep           *WearableSleepSection          `json:"sleep,omitempty" bson:"sleep,omitempty"`
	BloodPressure   *WearableBloodPressureSection  `json:"bloodPressure,omitempty" bson:"bloodPressure,omitempty"`
	TotalActivities []WearableTotalActivitySection `json:"totalActivities,omitempty" bson:"totalActivities,omitempty"`
}

type WearableMetricSection struct {
	Current       *float64                `json:"current,omitempty" bson:"current,omitempty" example:"77"`
	Average       *float64                `json:"average,omitempty" bson:"average,omitempty" example:"74"`
	Min           *float64                `json:"min,omitempty" bson:"min,omitempty" example:"58"`
	Max           *float64                `json:"max,omitempty" bson:"max,omitempty" example:"112"`
	Raw           *float64                `json:"raw,omitempty" bson:"raw,omitempty" example:"36.72"`
	ReadingsCount *float64                `json:"readingsCount,omitempty" bson:"readingsCount,omitempty" example:"96"`
	LastUpdated   string                  `json:"lastUpdated,omitempty" bson:"lastUpdated,omitempty" example:"2026-07-27T23:44:10.512Z"`
	Readings      []WearableMetricReading `json:"readings,omitempty" bson:"readings,omitempty"`
}

type WearableActivitySection struct {
	Steps       *float64                  `json:"steps,omitempty" bson:"steps,omitempty" example:"717"`
	Calories    *float64                  `json:"calories,omitempty" bson:"calories,omitempty" example:"28390"`
	Distance    *float64                  `json:"distance,omitempty" bson:"distance,omitempty" example:"493"`
	ActiveTime  *float64                  `json:"activeTime,omitempty" bson:"activeTime,omitempty" example:"29"`
	LastUpdated string                    `json:"lastUpdated,omitempty" bson:"lastUpdated,omitempty" example:"2026-07-27T23:44:10.512Z"`
	Readings    []WearableActivityReading `json:"readings,omitempty" bson:"readings,omitempty"`
}

type WearableSleepSection struct {
	TotalMinutes *float64               `json:"totalMinutes,omitempty" bson:"totalMinutes,omitempty" example:"431"`
	DeepMinutes  *float64               `json:"deepMinutes,omitempty" bson:"deepMinutes,omitempty" example:"92"`
	LightMinutes *float64               `json:"lightMinutes,omitempty" bson:"lightMinutes,omitempty" example:"268"`
	RemMinutes   *float64               `json:"remMinutes,omitempty" bson:"remMinutes,omitempty" example:"61"`
	AwakeMinutes *float64               `json:"awakeMinutes,omitempty" bson:"awakeMinutes,omitempty" example:"10"`
	Score        *float64               `json:"score,omitempty" bson:"score,omitempty" example:"78"`
	SleepStart   string                 `json:"sleepStart,omitempty" bson:"sleepStart,omitempty" example:"2026-07-26T22:41:00.000Z"`
	SleepEnd     string                 `json:"sleepEnd,omitempty" bson:"sleepEnd,omitempty" example:"2026-07-27T05:52:00.000Z"`
	LastUpdated  string                 `json:"lastUpdated,omitempty" bson:"lastUpdated,omitempty" example:"2026-07-27T23:44:10.512Z"`
	StageData    []int                  `json:"stageData,omitempty" bson:"stageData,omitempty"`
	Segments     []WearableSleepSegment `json:"segments,omitempty" bson:"segments,omitempty"`
}

type WearableBloodPressureSection struct {
	Systolic        *float64                       `json:"systolic,omitempty" bson:"systolic,omitempty" example:"118"`
	Diastolic       *float64                       `json:"diastolic,omitempty" bson:"diastolic,omitempty" example:"76"`
	HeartRate       *float64                       `json:"heartRate,omitempty" bson:"heartRate,omitempty" example:"72"`
	MeasurementTime string                         `json:"measurementTime,omitempty" bson:"measurementTime,omitempty" example:"2026-07-27T07:12:00.000Z"`
	LastUpdated     string                         `json:"lastUpdated,omitempty" bson:"lastUpdated,omitempty" example:"2026-07-27T23:44:10.512Z"`
	Readings        []WearableBloodPressureReading `json:"readings,omitempty" bson:"readings,omitempty"`
}

type WearableTotalActivitySection struct {
	Kind   string   `json:"kind,omitempty" bson:"kind,omitempty" example:"steps"`
	Value  *float64 `json:"value,omitempty" bson:"value,omitempty" example:"717"`
	Target *float64 `json:"target,omitempty" bson:"target,omitempty" example:"4000"`
}

type WearableMetricReading struct {
	Ts    string   `json:"ts,omitempty" bson:"ts,omitempty" example:"2026-07-27T23:44:10.512Z"`
	Value *float64 `json:"value,omitempty" bson:"value,omitempty" example:"77"`
	Raw   *float64 `json:"raw,omitempty" bson:"raw,omitempty" example:"36.72"`
}

type WearableActivityReading struct {
	Ts         string   `json:"ts,omitempty" bson:"ts,omitempty" example:"2026-07-27T08:00:00.000Z"`
	Steps      *float64 `json:"steps,omitempty" bson:"steps,omitempty" example:"120"`
	Calories   *float64 `json:"calories,omitempty" bson:"calories,omitempty" example:"8"`
	Distance   *float64 `json:"distance,omitempty" bson:"distance,omitempty" example:"95"`
	ActiveTime *float64 `json:"activeTime,omitempty" bson:"activeTime,omitempty" example:"15"`
}

type WearableSleepSegment struct {
	Start string `json:"start,omitempty" bson:"start,omitempty" example:"2026-07-26T22:41:00.000Z"`
	End   string `json:"end,omitempty" bson:"end,omitempty" example:"2026-07-26T23:11:00.000Z"`
	Stage string `json:"stage,omitempty" bson:"stage,omitempty" example:"light"`
}

type WearableBloodPressureReading struct {
	Ts        string   `json:"ts,omitempty" bson:"ts,omitempty" example:"2026-07-27T07:12:00.000Z"`
	Systolic  *float64 `json:"systolic,omitempty" bson:"systolic,omitempty" example:"118"`
	Diastolic *float64 `json:"diastolic,omitempty" bson:"diastolic,omitempty" example:"76"`
	HeartRate *float64 `json:"heartRate,omitempty" bson:"heartRate,omitempty" example:"72"`
}

type WearableSyncResponse struct {
	Status    string                     `json:"status" example:"synced"`
	DeviceID  string                     `json:"device_id" example:"66a000000000000000000001"`
	DeviceUID string                     `json:"device_uid" example:"qring-aa-bb-cc-01"`
	SyncDate  string                     `json:"sync_date" example:"2026-07-27"`
	Upserted  WearableSyncUpsertCounters `json:"upserted"`
}

type WearableSyncUpsertCounters struct {
	Snapshot        int `json:"snapshot" example:"1"`
	HealthMetrics   int `json:"health_metrics" example:"6"`
	DailyActivity   int `json:"daily_activity" example:"1"`
	Steps15m        int `json:"steps_15m,omitempty" example:"1"`
	SleepSummary    int `json:"sleep_summary" example:"1"`
	SleepSegments   int `json:"sleep_segments,omitempty" example:"1"`
	TotalActivities int `json:"total_activities" example:"3"`
}

type WearableRangeResponse struct {
	DeviceUID string              `json:"deviceUid" example:"qring-aa-bb-cc-01"`
	From      string              `json:"from" example:"2026-07-01"`
	To        string              `json:"to" example:"2026-07-27"`
	Items     []WearableRangeItem `json:"items"`
}

type UserWearableRangeResponse struct {
	UserID  string                    `json:"user_id" example:"66a000000000000000000100"`
	From    string                    `json:"from" example:"2026-07-01"`
	To      string                    `json:"to" example:"2026-07-27"`
	Devices []UserWearableRangeDevice `json:"devices"`
}

type UserWearableRangeDevice struct {
	UserDeviceID string              `json:"user_device_id" example:"66a000000000000000000200"`
	DeviceID     string              `json:"device_id" example:"66a000000000000000000001"`
	DeviceUID    string              `json:"deviceUid" example:"qring-aa-bb-cc-01"`
	Nickname     *string             `json:"nickname,omitempty" example:"daily ring"`
	IsPrimary    bool                `json:"is_primary" example:"true"`
	PairedAt     string              `json:"paired_at" example:"2026-07-01T00:00:00Z"`
	UnpairedAt   *string             `json:"unpaired_at,omitempty" example:"2026-07-28T00:00:00Z"`
	Items        []WearableRangeItem `json:"items"`
}

type WearableRangeItem struct {
	Date            string                         `json:"date" example:"2026-07-27"`
	DeviceUID       string                         `json:"deviceUid" example:"qring-aa-bb-cc-01"`
	TzOffsetMin     *int                           `json:"tzOffsetMin,omitempty" example:"420"`
	HRV             WearableMetricSection          `json:"hrv"`
	HeartRate       WearableMetricSection          `json:"heartRate"`
	SpO2            WearableMetricSection          `json:"spo2"`
	Temperature     WearableMetricSection          `json:"temperature"`
	Stress          WearableMetricSection          `json:"stress"`
	Activity        WearableActivitySection        `json:"activity"`
	Sleep           WearableSleepSection           `json:"sleep"`
	BloodPressure   WearableBloodPressureSection   `json:"bloodPressure"`
	TotalActivities []WearableTotalActivitySection `json:"totalActivities"`
}

type TotalActivityRow struct {
	DeviceID   string
	DeviceUID  string
	DeviceDate string
	Kind       string
	Index      int
	Value      *float64
	Target     *float64
	Source     string
}

type totalActivityDoc struct {
	ID         primitive.ObjectID `bson:"_id,omitempty"`
	DeviceID   string             `bson:"device_id"`
	DeviceUID  string             `bson:"device_uid"`
	DeviceDate string             `bson:"device_date"`
	Kind       string             `bson:"kind"`
	Index      int                `bson:"index"`
	Value      *float64           `bson:"value,omitempty"`
	Target     *float64           `bson:"target,omitempty"`
	Source     string             `bson:"source"`
	SyncedAt   time.Time          `bson:"synced_at"`
}

type wearableSyncDoc struct {
	ID         primitive.ObjectID      `bson:"_id,omitempty"`
	DeviceID   string                  `bson:"device_id"`
	DeviceUID  string                  `bson:"device_uid"`
	SyncDate   string                  `bson:"sync_date"`
	Payload    WearableSnapshotPayload `bson:"payload"`
	Source     string                  `bson:"source"`
	ReceivedAt time.Time               `bson:"received_at"`
	CreatedAt  time.Time               `bson:"created_at"`
	UpdatedAt  time.Time               `bson:"updated_at"`
}

type wearableFanout struct {
	healthMetrics   []HealthMetricRow
	steps15m        []Step15mRow
	dailyActivity   *DailyActivityRow
	sleepSummary    *SleepSummaryRow
	sleepSegments   []SleepSegmentRow
	totalActivities []TotalActivityRow
}

func (s *Store) SyncWearable(ctx context.Context, req WearableSyncRequest) (*WearableSyncResponse, error) {
	deviceUID := req.CanonicalDeviceUID()
	if deviceUID == "" {
		return nil, wearablePayloadError("deviceUid is required")
	}
	if !req.HasRecognizedData() {
		return nil, wearablePayloadError("at least one wearable data section is required")
	}

	syncTime, err := req.DeriveSyncTime(time.Now().UTC())
	if err != nil {
		return nil, err
	}
	syncDate := syncTime.Format(wearableDateLayout)

	device, err := s.EnsureDeviceByUID(ctx, deviceUID)
	if err != nil {
		return nil, err
	}

	fanout, err := buildWearableFanout(device.ID, deviceUID, req, syncDate, syncTime)
	if err != nil {
		return nil, err
	}

	counters := WearableSyncUpsertCounters{Snapshot: 1}

	if len(fanout.healthMetrics) > 0 {
		n, err := s.IngestHealthMetricsBulk(ctx, fanout.healthMetrics)
		if err != nil {
			return nil, err
		}
		counters.HealthMetrics = n
	}
	if len(fanout.steps15m) > 0 {
		n, err := s.UpsertSteps15mBulk(ctx, fanout.steps15m)
		if err != nil {
			return nil, err
		}
		counters.Steps15m = n
	}
	if fanout.dailyActivity != nil {
		if _, err := s.UpsertDailyActivity(ctx, *fanout.dailyActivity); err != nil {
			return nil, err
		}
		counters.DailyActivity = 1
	}
	if fanout.sleepSummary != nil {
		if _, err := s.UpsertSleepSummary(ctx, *fanout.sleepSummary); err != nil {
			return nil, err
		}
		counters.SleepSummary = 1
	}
	if len(fanout.sleepSegments) > 0 {
		n, err := s.UpsertSleepSegmentsBulk(ctx, fanout.sleepSegments)
		if err != nil {
			return nil, err
		}
		counters.SleepSegments = n
	}
	if len(fanout.totalActivities) > 0 {
		n, err := s.UpsertTotalActivities(ctx, fanout.totalActivities)
		if err != nil {
			return nil, err
		}
		counters.TotalActivities = n
	}
	if err := s.UpsertWearableSnapshot(ctx, device.ID, deviceUID, syncDate, req.SnapshotPayload()); err != nil {
		return nil, err
	}

	return &WearableSyncResponse{
		Status:    "synced",
		DeviceID:  device.ID,
		DeviceUID: deviceUID,
		SyncDate:  syncDate,
		Upserted:  counters,
	}, nil
}

func (s *Store) EnsureDeviceByUID(ctx context.Context, uid string) (*DeviceRow, error) {
	uid = strings.TrimSpace(uid)
	if uid == "" {
		return nil, wearablePayloadError("deviceUid is required")
	}

	device, err := s.GetDeviceByUID(ctx, uid)
	if err == nil {
		return device, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	device, err = s.CreateDevice(ctx, CreateDeviceParams{
		Vendor:    "QRing",
		DeviceUID: uid,
	})
	if err == nil {
		return device, nil
	}
	if mongo.IsDuplicateKeyError(err) {
		return s.GetDeviceByUID(ctx, uid)
	}
	return nil, err
}

func (s *Store) UpsertWearableSnapshot(ctx context.Context, deviceID, deviceUID, syncDate string, payload WearableSnapshotPayload) error {
	cctx, cancel := ctxTimeout(ctx)
	defer cancel()

	now := time.Now().UTC()
	docID := primitive.NewObjectID()
	_, err := s.db.Collection("wearable_syncs").UpdateOne(
		cctx,
		bson.M{"device_uid": deviceUID, "sync_date": syncDate},
		bson.M{
			"$set": bson.M{
				"device_id":   deviceID,
				"device_uid":  deviceUID,
				"sync_date":   syncDate,
				"payload":     payload,
				"source":      wearableSyncSource,
				"received_at": now,
				"updated_at":  now,
			},
			"$setOnInsert": bson.M{
				"_id":        docID,
				"created_at": now,
			},
		},
		options.Update().SetUpsert(true),
	)
	return err
}

func (s *Store) ListWearableSnapshots(ctx context.Context, deviceUID, fromDate, toDate string, includeReadings bool) ([]WearableRangeItem, error) {
	cctx, cancel := ctxTimeout(ctx)
	defer cancel()

	cursor, err := s.db.Collection("wearable_syncs").Find(
		cctx,
		bson.M{
			"device_uid": deviceUID,
			"sync_date": bson.M{
				"$gte": fromDate,
				"$lte": toDate,
			},
		},
		options.Find().SetSort(bson.D{{Key: "sync_date", Value: 1}}),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(cctx)

	out := make([]WearableRangeItem, 0)
	for cursor.Next(cctx) {
		var doc wearableSyncDoc
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		out = append(out, doc.toRangeItem(includeReadings))
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) ListWearableSnapshotsByUser(ctx context.Context, userID, fromDate, toDate string, includeReadings bool) (*UserWearableRangeResponse, error) {
	userDevices, err := s.ListUserDevices(ctx, userID)
	if err != nil {
		return nil, err
	}

	response := &UserWearableRangeResponse{
		UserID:  userID,
		From:    fromDate,
		To:      toDate,
		Devices: make([]UserWearableRangeDevice, 0, len(userDevices)),
	}

	seenDevices := make(map[string]struct{}, len(userDevices))
	for _, userDevice := range userDevices {
		if _, seen := seenDevices[userDevice.DeviceID]; seen {
			continue
		}
		seenDevices[userDevice.DeviceID] = struct{}{}

		device, err := s.GetDevice(ctx, userDevice.DeviceID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				continue
			}
			return nil, err
		}

		items, err := s.ListWearableSnapshots(ctx, device.DeviceUID, fromDate, toDate, includeReadings)
		if err != nil {
			return nil, err
		}

		response.Devices = append(response.Devices, UserWearableRangeDevice{
			UserDeviceID: userDevice.ID,
			DeviceID:     userDevice.DeviceID,
			DeviceUID:    device.DeviceUID,
			Nickname:     userDevice.Nickname,
			IsPrimary:    userDevice.IsPrimary,
			PairedAt:     userDevice.PairedAt,
			UnpairedAt:   userDevice.UnpairedAt,
			Items:        items,
		})
	}

	return response, nil
}

func (s *Store) UpsertTotalActivities(ctx context.Context, rows []TotalActivityRow) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}

	cctx, cancel := ctxTimeout(ctx)
	defer cancel()

	now := time.Now().UTC()
	count := 0
	for _, row := range rows {
		src := row.Source
		if src == "" {
			src = wearableSyncSource
		}
		set := bson.M{
			"device_id":   row.DeviceID,
			"device_uid":  row.DeviceUID,
			"device_date": row.DeviceDate,
			"kind":        row.Kind,
			"index":       row.Index,
			"value":       row.Value,
			"target":      row.Target,
			"source":      src,
			"synced_at":   now,
		}
		_, err := s.db.Collection("total_activities").UpdateOne(
			cctx,
			bson.M{"device_id": row.DeviceID, "device_date": row.DeviceDate, "kind": row.Kind},
			bson.M{
				"$set": set,
				"$setOnInsert": bson.M{
					"_id":        primitive.NewObjectID(),
					"created_at": now,
				},
			},
			options.Update().SetUpsert(true),
		)
		if err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func (p WearableSyncRequest) CanonicalDeviceUID() string {
	if uid := strings.TrimSpace(p.DeviceUID); uid != "" {
		return uid
	}
	return strings.TrimSpace(p.DeviceUIDAlias)
}

func (p WearableSyncRequest) TzOffsetMinutes() int {
	if p.TzOffsetMin == nil {
		return 0
	}
	return *p.TzOffsetMin
}

func (p WearableSyncRequest) HasRecognizedData() bool {
	return p.HRV != nil ||
		p.HeartRate != nil ||
		p.SpO2 != nil ||
		p.Temperature != nil ||
		p.Stress != nil ||
		p.Activity != nil ||
		p.Sleep != nil ||
		p.BloodPressure != nil ||
		len(p.TotalActivities) > 0
}

func (p WearableSyncRequest) SnapshotPayload() WearableSnapshotPayload {
	return WearableSnapshotPayload{
		TzOffsetMin:     p.TzOffsetMin,
		HRV:             p.HRV,
		HeartRate:       p.HeartRate,
		SpO2:            p.SpO2,
		Temperature:     p.Temperature,
		Stress:          p.Stress,
		Activity:        p.Activity,
		Sleep:           p.Sleep,
		BloodPressure:   p.BloodPressure,
		TotalActivities: p.TotalActivities,
	}
}

type wearableTimeCandidate struct {
	value string
	field string
}

func (p WearableSyncRequest) DeriveSyncTime(now time.Time) (time.Time, error) {
	var latest *time.Time
	add := func(value, field string) error {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil
		}
		t, err := parseWearableTime(value, field)
		if err != nil {
			return err
		}
		t = t.UTC()
		if latest == nil || t.After(*latest) {
			latest = &t
		}
		return nil
	}

	for _, candidate := range []struct {
		value string
		field string
	}{
		{metricLastUpdated(p.HRV), "hrv.lastUpdated"},
		{metricLastUpdated(p.HeartRate), "heartRate.lastUpdated"},
		{metricLastUpdated(p.SpO2), "spo2.lastUpdated"},
		{metricLastUpdated(p.Temperature), "temperature.lastUpdated"},
		{metricLastUpdated(p.Stress), "stress.lastUpdated"},
		{activityLastUpdated(p.Activity), "activity.lastUpdated"},
		{sleepLastUpdated(p.Sleep), "sleep.lastUpdated"},
		{bloodPressureLastUpdated(p.BloodPressure), "bloodPressure.lastUpdated"},
		{bloodPressureMeasurementTime(p.BloodPressure), "bloodPressure.measurementTime"},
		{sleepEnd(p.Sleep), "sleep.sleepEnd"},
		{sleepStart(p.Sleep), "sleep.sleepStart"},
	} {
		if err := add(candidate.value, candidate.field); err != nil {
			return time.Time{}, err
		}
	}

	for _, candidate := range p.readingTimeCandidates() {
		if err := add(candidate.value, candidate.field); err != nil {
			return time.Time{}, err
		}
	}

	if latest == nil {
		fallback := now.UTC()
		latest = &fallback
	}
	return *latest, nil
}

func (p WearableSyncRequest) readingTimeCandidates() []wearableTimeCandidate {
	var out []wearableTimeCandidate
	addMetric := func(path string, section *WearableMetricSection) {
		if section == nil {
			return
		}
		for i, reading := range section.Readings {
			out = append(out, wearableTimeCandidate{
				value: reading.Ts,
				field: fmt.Sprintf("%s[%d].ts", path, i),
			})
		}
	}
	addMetric("hrv.readings", p.HRV)
	addMetric("heartRate.readings", p.HeartRate)
	addMetric("spo2.readings", p.SpO2)
	addMetric("temperature.readings", p.Temperature)
	addMetric("stress.readings", p.Stress)

	if p.Activity != nil {
		for i, reading := range p.Activity.Readings {
			out = append(out, wearableTimeCandidate{
				value: reading.Ts,
				field: fmt.Sprintf("activity.readings[%d].ts", i),
			})
		}
	}
	if p.Sleep != nil {
		for i, segment := range p.Sleep.Segments {
			out = append(out, wearableTimeCandidate{
				value: segment.Start,
				field: fmt.Sprintf("sleep.segments[%d].start", i),
			})
			out = append(out, wearableTimeCandidate{
				value: segment.End,
				field: fmt.Sprintf("sleep.segments[%d].end", i),
			})
		}
	}
	if p.BloodPressure != nil {
		for i, reading := range p.BloodPressure.Readings {
			out = append(out, wearableTimeCandidate{
				value: reading.Ts,
				field: fmt.Sprintf("bloodPressure.readings[%d].ts", i),
			})
		}
	}
	return out
}

func buildWearableFanout(deviceID, deviceUID string, req WearableSyncRequest, syncDate string, fallbackTime time.Time) (wearableFanout, error) {
	var fanout wearableFanout
	tzOffsetMin := req.TzOffsetMinutes()

	if err := addHealthMetric(&fanout.healthMetrics, deviceID, deviceUID, "hrv", "ms", req.HRV, fallbackTime, "hrv.lastUpdated"); err != nil {
		return fanout, err
	}
	if err := addMetricReadings(&fanout.healthMetrics, deviceID, deviceUID, "hrv", "ms", req.HRV, "hrv.readings"); err != nil {
		return fanout, err
	}
	if err := addHealthMetric(&fanout.healthMetrics, deviceID, deviceUID, "heart_rate", "bpm", req.HeartRate, fallbackTime, "heartRate.lastUpdated"); err != nil {
		return fanout, err
	}
	if err := addMetricReadings(&fanout.healthMetrics, deviceID, deviceUID, "heart_rate", "bpm", req.HeartRate, "heartRate.readings"); err != nil {
		return fanout, err
	}
	if err := addHealthMetric(&fanout.healthMetrics, deviceID, deviceUID, "spo2", "%", req.SpO2, fallbackTime, "spo2.lastUpdated"); err != nil {
		return fanout, err
	}
	if err := addMetricReadings(&fanout.healthMetrics, deviceID, deviceUID, "spo2", "%", req.SpO2, "spo2.readings"); err != nil {
		return fanout, err
	}
	if err := addHealthMetric(&fanout.healthMetrics, deviceID, deviceUID, "temperature", "celsius", req.Temperature, fallbackTime, "temperature.lastUpdated"); err != nil {
		return fanout, err
	}
	if err := addMetricReadings(&fanout.healthMetrics, deviceID, deviceUID, "temperature", "celsius", req.Temperature, "temperature.readings"); err != nil {
		return fanout, err
	}
	if err := addHealthMetric(&fanout.healthMetrics, deviceID, deviceUID, "stress", "score", req.Stress, fallbackTime, "stress.lastUpdated"); err != nil {
		return fanout, err
	}
	if err := addMetricReadings(&fanout.healthMetrics, deviceID, deviceUID, "stress", "score", req.Stress, "stress.readings"); err != nil {
		return fanout, err
	}
	if req.BloodPressure != nil && req.BloodPressure.hasValues() {
		ts, err := bloodPressureMetricTime(req.BloodPressure, fallbackTime)
		if err != nil {
			return fanout, err
		}
		fanout.healthMetrics = append(fanout.healthMetrics, HealthMetricRow{
			DeviceID: deviceID,
			TsUTC:    ts,
			Metric:   "blood_pressure",
			Values:   req.BloodPressure.values(),
			Unit:     "mmHg",
			Source:   wearableSyncSource,
			Metadata: map[string]any{"device_uid": deviceUID},
		})
	}
	if err := addBloodPressureReadings(&fanout.healthMetrics, deviceID, deviceUID, req.BloodPressure); err != nil {
		return fanout, err
	}

	if req.Activity != nil && req.Activity.hasValues() {
		fanout.dailyActivity = &DailyActivityRow{
			DeviceID:       deviceID,
			DeviceDate:     syncDate,
			TzOffsetMin:    tzOffsetMin,
			TotalSteps:     roundNumber(req.Activity.Steps),
			Calories:       roundNumber(req.Activity.Calories),
			WalkDistanceM:  roundNumber(req.Activity.Distance),
			SportDurationS: roundMinutesToSeconds(req.Activity.ActiveTime),
			Source:         wearableSyncSource,
		}
	}
	if req.Activity != nil && len(req.Activity.Readings) > 0 {
		rows, err := activityReadingsToSteps15m(deviceID, req.Activity.Readings, tzOffsetMin)
		if err != nil {
			return fanout, err
		}
		fanout.steps15m = append(fanout.steps15m, rows...)
	}

	if req.Sleep != nil && req.Sleep.hasValues() {
		row, err := sleepSummaryFromWearable(deviceID, syncDate, req.Sleep)
		if err != nil {
			return fanout, err
		}
		row.TzOffsetMin = tzOffsetMin
		fanout.sleepSummary = row
	}
	if req.Sleep != nil && len(req.Sleep.Segments) > 0 {
		rows, err := sleepSegmentsFromWearable(deviceID, syncDate, tzOffsetMin, req.Sleep.Segments)
		if err != nil {
			return fanout, err
		}
		fanout.sleepSegments = append(fanout.sleepSegments, rows...)
	}

	if len(req.TotalActivities) > 0 {
		fanout.totalActivities = make([]TotalActivityRow, 0, len(req.TotalActivities))
		for index, item := range req.TotalActivities {
			fanout.totalActivities = append(fanout.totalActivities, TotalActivityRow{
				DeviceID:   deviceID,
				DeviceUID:  deviceUID,
				DeviceDate: syncDate,
				Kind:       totalActivityKind(index, item.Kind),
				Index:      index,
				Value:      item.Value,
				Target:     item.Target,
				Source:     wearableSyncSource,
			})
		}
	}

	return fanout, nil
}

func addHealthMetric(rows *[]HealthMetricRow, deviceID, deviceUID, metric, unit string, section *WearableMetricSection, fallbackTime time.Time, timeField string) error {
	if section == nil || !section.hasValues() {
		return nil
	}
	ts, err := section.metricTime(fallbackTime, timeField)
	if err != nil {
		return err
	}
	*rows = append(*rows, HealthMetricRow{
		DeviceID: deviceID,
		TsUTC:    ts,
		Metric:   metric,
		Value:    section.Current,
		Values:   section.values(),
		Unit:     unit,
		Source:   wearableSyncSource,
		Metadata: map[string]any{"device_uid": deviceUID},
	})
	return nil
}

func addMetricReadings(rows *[]HealthMetricRow, deviceID, deviceUID, metric, unit string, section *WearableMetricSection, path string) error {
	if section == nil || len(section.Readings) == 0 {
		return nil
	}
	for i, reading := range section.Readings {
		if !reading.hasValues() {
			continue
		}
		ts, err := parseRequiredWearableTime(reading.Ts, fmt.Sprintf("%s[%d].ts", path, i))
		if err != nil {
			return err
		}
		values := map[string]float64{}
		addFloatValue(values, "raw", reading.Raw)
		if len(values) == 0 {
			values = nil
		}
		*rows = append(*rows, HealthMetricRow{
			DeviceID: deviceID,
			TsUTC:    ts,
			Metric:   metric,
			Value:    reading.Value,
			Values:   values,
			Unit:     unit,
			Source:   wearableMetricReadingSource,
			Metadata: map[string]any{"device_uid": deviceUID, "reading_index": i},
		})
	}
	return nil
}

func addBloodPressureReadings(rows *[]HealthMetricRow, deviceID, deviceUID string, section *WearableBloodPressureSection) error {
	if section == nil || len(section.Readings) == 0 {
		return nil
	}
	for i, reading := range section.Readings {
		if !reading.hasValues() {
			continue
		}
		ts, err := parseRequiredWearableTime(reading.Ts, fmt.Sprintf("bloodPressure.readings[%d].ts", i))
		if err != nil {
			return err
		}
		*rows = append(*rows, HealthMetricRow{
			DeviceID: deviceID,
			TsUTC:    ts,
			Metric:   "blood_pressure",
			Values:   reading.values(),
			Unit:     "mmHg",
			Source:   wearableMetricReadingSource,
			Metadata: map[string]any{"device_uid": deviceUID, "reading_index": i},
		})
	}
	return nil
}

func activityReadingsToSteps15m(deviceID string, readings []WearableActivityReading, tzOffsetMin int) ([]Step15mRow, error) {
	rows := make([]Step15mRow, 0, len(readings))
	for i, reading := range readings {
		if !reading.hasValues() {
			continue
		}
		ts, err := parseRequiredWearableTime(reading.Ts, fmt.Sprintf("activity.readings[%d].ts", i))
		if err != nil {
			return nil, err
		}
		deviceDate, timeIndex := wearableDeviceDateAndTimeIndex(ts, tzOffsetMin)
		rows = append(rows, Step15mRow{
			DeviceID:       deviceID,
			TsUTC:          ts,
			DeviceDate:     deviceDate,
			TimeIndex:      timeIndex,
			TzOffsetMin:    tzOffsetMin,
			WalkSteps:      roundNumber(reading.Steps),
			Calories:       roundNumber(reading.Calories),
			DistanceM:      roundNumber(reading.Distance),
			SportDurationS: roundMinutesToSeconds(reading.ActiveTime),
			Source:         wearableActivityReadingSource,
		})
	}
	return rows, nil
}

func sleepSegmentsFromWearable(deviceID, syncDate string, tzOffsetMin int, segments []WearableSleepSegment) ([]SleepSegmentRow, error) {
	rows := make([]SleepSegmentRow, 0, len(segments))
	for i, segment := range segments {
		if segment.isEmpty() {
			continue
		}
		start, err := parseRequiredWearableTime(segment.Start, fmt.Sprintf("sleep.segments[%d].start", i))
		if err != nil {
			return nil, err
		}
		end, err := parseRequiredWearableTime(segment.End, fmt.Sprintf("sleep.segments[%d].end", i))
		if err != nil {
			return nil, err
		}
		if !start.Before(end) {
			return nil, wearablePayloadError(fmt.Sprintf("sleep.segments[%d].end must be after start", i))
		}
		rows = append(rows, SleepSegmentRow{
			DeviceID:    deviceID,
			StartUTC:    start,
			EndUTC:      end,
			SleepDate:   syncDate,
			TzOffsetMin: tzOffsetMin,
			Stage:       strings.TrimSpace(segment.Stage),
			Source:      wearableSleepSegmentSource,
		})
	}
	return rows, nil
}

func sleepSummaryFromWearable(deviceID, syncDate string, sleep *WearableSleepSection) (*SleepSummaryRow, error) {
	var sleepStart *time.Time
	if strings.TrimSpace(sleep.SleepStart) != "" {
		t, err := parseWearableTime(sleep.SleepStart, "sleep.sleepStart")
		if err != nil {
			return nil, err
		}
		t = t.UTC()
		sleepStart = &t
	}

	var sleepEnd *time.Time
	if strings.TrimSpace(sleep.SleepEnd) != "" {
		t, err := parseWearableTime(sleep.SleepEnd, "sleep.sleepEnd")
		if err != nil {
			return nil, err
		}
		t = t.UTC()
		sleepEnd = &t
	}

	var score *int
	if sleep.Score != nil {
		v := roundNumber(sleep.Score)
		score = &v
	}

	var stageData []int
	if sleep.StageData != nil {
		stageData = append([]int(nil), sleep.StageData...)
	}

	return &SleepSummaryRow{
		DeviceID:      deviceID,
		SleepDate:     syncDate,
		SleepStartUTC: sleepStart,
		WakeUTC:       sleepEnd,
		TotalSleepS:   roundMinutesToSeconds(sleep.TotalMinutes),
		DeepS:         roundMinutesToSeconds(sleep.DeepMinutes),
		LightS:        roundMinutesToSeconds(sleep.LightMinutes),
		AwakeS:        roundMinutesToSeconds(sleep.AwakeMinutes),
		RemS:          roundMinutesToSeconds(sleep.RemMinutes),
		Score:         score,
		StageData:     stageData,
		Source:        wearableSyncSource,
	}, nil
}

func (doc wearableSyncDoc) toRangeItem(includeReadings bool) WearableRangeItem {
	hrv := derefMetricSection(doc.Payload.HRV)
	heartRate := derefMetricSection(doc.Payload.HeartRate)
	spo2 := derefMetricSection(doc.Payload.SpO2)
	temperature := derefMetricSection(doc.Payload.Temperature)
	stress := derefMetricSection(doc.Payload.Stress)
	activity := derefActivitySection(doc.Payload.Activity)
	sleep := derefSleepSection(doc.Payload.Sleep)
	bloodPressure := derefBloodPressureSection(doc.Payload.BloodPressure)
	if !includeReadings {
		hrv.Readings = nil
		heartRate.Readings = nil
		spo2.Readings = nil
		temperature.Readings = nil
		stress.Readings = nil
		activity.Readings = nil
		sleep.Segments = nil
		bloodPressure.Readings = nil
	}

	return WearableRangeItem{
		Date:            doc.SyncDate,
		DeviceUID:       doc.DeviceUID,
		TzOffsetMin:     doc.Payload.TzOffsetMin,
		HRV:             hrv,
		HeartRate:       heartRate,
		SpO2:            spo2,
		Temperature:     temperature,
		Stress:          stress,
		Activity:        activity,
		Sleep:           sleep,
		BloodPressure:   bloodPressure,
		TotalActivities: totalActivitiesOrEmpty(doc.Payload.TotalActivities),
	}
}

func (section *WearableMetricSection) hasValues() bool {
	return section.Current != nil ||
		section.Average != nil ||
		section.Min != nil ||
		section.Max != nil ||
		section.Raw != nil ||
		section.ReadingsCount != nil
}

func (section *WearableMetricSection) metricTime(fallback time.Time, field string) (time.Time, error) {
	if strings.TrimSpace(section.LastUpdated) == "" {
		return fallback.UTC(), nil
	}
	return parseWearableTime(section.LastUpdated, field)
}

func (section *WearableMetricSection) values() map[string]float64 {
	values := map[string]float64{}
	addFloatValue(values, "average", section.Average)
	addFloatValue(values, "min", section.Min)
	addFloatValue(values, "max", section.Max)
	addFloatValue(values, "raw", section.Raw)
	addFloatValue(values, "readings_count", section.ReadingsCount)
	if len(values) == 0 {
		return nil
	}
	return values
}

func (section *WearableActivitySection) hasValues() bool {
	return section.Steps != nil ||
		section.Calories != nil ||
		section.Distance != nil ||
		section.ActiveTime != nil
}

func (section *WearableSleepSection) hasValues() bool {
	return section.TotalMinutes != nil ||
		section.DeepMinutes != nil ||
		section.LightMinutes != nil ||
		section.RemMinutes != nil ||
		section.AwakeMinutes != nil ||
		section.Score != nil ||
		strings.TrimSpace(section.SleepStart) != "" ||
		strings.TrimSpace(section.SleepEnd) != "" ||
		len(section.StageData) > 0
}

func (section *WearableBloodPressureSection) hasValues() bool {
	return section.Systolic != nil || section.Diastolic != nil || section.HeartRate != nil
}

func (section *WearableBloodPressureSection) values() map[string]float64 {
	values := map[string]float64{}
	addFloatValue(values, "systolic", section.Systolic)
	addFloatValue(values, "diastolic", section.Diastolic)
	addFloatValue(values, "heart_rate", section.HeartRate)
	return values
}

func (reading WearableMetricReading) hasValues() bool {
	return reading.Value != nil || reading.Raw != nil
}

func (reading WearableActivityReading) hasValues() bool {
	return reading.Steps != nil || reading.Calories != nil || reading.Distance != nil || reading.ActiveTime != nil
}

func (segment WearableSleepSegment) isEmpty() bool {
	return strings.TrimSpace(segment.Start) == "" &&
		strings.TrimSpace(segment.End) == "" &&
		strings.TrimSpace(segment.Stage) == ""
}

func (reading WearableBloodPressureReading) hasValues() bool {
	return reading.Systolic != nil || reading.Diastolic != nil || reading.HeartRate != nil
}

func (reading WearableBloodPressureReading) values() map[string]float64 {
	values := map[string]float64{}
	addFloatValue(values, "systolic", reading.Systolic)
	addFloatValue(values, "diastolic", reading.Diastolic)
	addFloatValue(values, "heart_rate", reading.HeartRate)
	return values
}

func bloodPressureMetricTime(section *WearableBloodPressureSection, fallback time.Time) (time.Time, error) {
	if strings.TrimSpace(section.MeasurementTime) != "" {
		return parseWearableTime(section.MeasurementTime, "bloodPressure.measurementTime")
	}
	if strings.TrimSpace(section.LastUpdated) != "" {
		return parseWearableTime(section.LastUpdated, "bloodPressure.lastUpdated")
	}
	return fallback.UTC(), nil
}

func totalActivityKind(index int, explicit string) string {
	if kind := sanitizeTotalActivityKind(explicit); kind != "" {
		return kind
	}
	switch index {
	case 0:
		return "steps"
	case 1:
		return "calories"
	case 2:
		return "distance"
	default:
		return fmt.Sprintf("unknown_%d", index)
	}
}

func sanitizeTotalActivityKind(kind string) string {
	kind = strings.ToLower(strings.TrimSpace(kind))
	kind = strings.ReplaceAll(kind, " ", "_")
	kind = strings.ReplaceAll(kind, "-", "_")
	return kind
}

func addFloatValue(values map[string]float64, key string, value *float64) {
	if value != nil {
		values[key] = *value
	}
}

func roundNumber(value *float64) int {
	if value == nil {
		return 0
	}
	return int(math.Round(*value))
}

func roundMinutesToSeconds(value *float64) int {
	if value == nil {
		return 0
	}
	return int(math.Round(*value * 60))
}

func parseWearableTime(value, field string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(value))
	if err != nil {
		return time.Time{}, wearablePayloadError("invalid " + field)
	}
	return t.UTC(), nil
}

func parseRequiredWearableTime(value, field string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, wearablePayloadError("missing " + field)
	}
	return parseWearableTime(value, field)
}

func wearableDeviceDateAndTimeIndex(ts time.Time, tzOffsetMin int) (string, int) {
	local := ts.UTC().Add(time.Duration(tzOffsetMin) * time.Minute)
	minuteOfDay := local.Hour()*60 + local.Minute()
	return local.Format(wearableDateLayout), minuteOfDay / 15
}

func wearablePayloadError(message string) error {
	return fmt.Errorf("%w: %s", ErrInvalidWearablePayload, message)
}

func metricLastUpdated(section *WearableMetricSection) string {
	if section == nil {
		return ""
	}
	return section.LastUpdated
}

func activityLastUpdated(section *WearableActivitySection) string {
	if section == nil {
		return ""
	}
	return section.LastUpdated
}

func sleepLastUpdated(section *WearableSleepSection) string {
	if section == nil {
		return ""
	}
	return section.LastUpdated
}

func sleepStart(section *WearableSleepSection) string {
	if section == nil {
		return ""
	}
	return section.SleepStart
}

func sleepEnd(section *WearableSleepSection) string {
	if section == nil {
		return ""
	}
	return section.SleepEnd
}

func bloodPressureLastUpdated(section *WearableBloodPressureSection) string {
	if section == nil {
		return ""
	}
	return section.LastUpdated
}

func bloodPressureMeasurementTime(section *WearableBloodPressureSection) string {
	if section == nil {
		return ""
	}
	return section.MeasurementTime
}

func derefMetricSection(section *WearableMetricSection) WearableMetricSection {
	if section == nil {
		return WearableMetricSection{}
	}
	return *section
}

func derefActivitySection(section *WearableActivitySection) WearableActivitySection {
	if section == nil {
		return WearableActivitySection{}
	}
	return *section
}

func derefSleepSection(section *WearableSleepSection) WearableSleepSection {
	if section == nil {
		return WearableSleepSection{}
	}
	return *section
}

func derefBloodPressureSection(section *WearableBloodPressureSection) WearableBloodPressureSection {
	if section == nil {
		return WearableBloodPressureSection{}
	}
	return *section
}

func totalActivitiesOrEmpty(items []WearableTotalActivitySection) []WearableTotalActivitySection {
	if items == nil {
		return []WearableTotalActivitySection{}
	}
	return items
}
