package store

import "errors"

const (
	wearableDateLayout          = "2006-01-02"
	wearableV3Source            = "qring_sdk_v3"
	rawSensorRetentionSeconds   = 30 * secondsPerDay
	rawSensorKeyRetentionSecond = rawSensorRetentionSeconds + keyRetentionGraceSeconds
)

var (
	ErrInvalidWearablePayload = errors.New("invalid wearable payload")
	ErrDeviceAlreadyPaired    = errors.New("device already paired to another user")
	ErrPairingNotFound        = errors.New("active pairing not found")
)

// WearableSyncRequest is the canonical QRing SDK v3 ingest contract.
//
// One request can contain multiple device-local days because the QRing SDK can
// synchronize historical data by day offset. Every day is explicit; the backend
// never derives a single date from the latest timestamp in the payload.
type WearableSyncRequest struct {
	DeviceUID      string                 `json:"deviceUid,omitempty"`
	DeviceUIDAlias string                 `json:"device_uid,omitempty"`
	TzOffsetMin    int                    `json:"tzOffsetMin"`
	Device         WearableDeviceMetadata `json:"device,omitempty"`
	DeviceState    *WearableDeviceState   `json:"deviceState,omitempty"`
	Days           []WearableDay          `json:"days,omitempty"`
}

type WearableDeviceMetadata struct {
	Vendor       string          `json:"vendor,omitempty"`
	Model        string          `json:"model,omitempty"`
	HWRev        string          `json:"hwRev,omitempty"`
	FWRev        string          `json:"fwRev,omitempty"`
	SerialNumber string          `json:"serialNumber,omitempty"`
	SDKVersion   string          `json:"sdkVersion,omitempty"`
	Capabilities map[string]bool `json:"capabilities,omitempty"`
}

type WearableDeviceState struct {
	BatteryPercent *int   `json:"batteryPercent,omitempty"`
	Charging       *bool  `json:"charging,omitempty"`
	LastSeenAt     string `json:"lastSeenAt,omitempty"`
}

type WearableDay struct {
	Date            string                    `json:"date"`
	Activity        *WearableActivitySummary  `json:"activity,omitempty"`
	ActivityBuckets []WearableActivityBucket  `json:"activityBuckets,omitempty"`
	Measurements    []WearableMeasurement     `json:"measurements,omitempty"`
	SleepSessions   []WearableSleepSession    `json:"sleepSessions,omitempty"`
	Targets         []WearableTargetProgress  `json:"targets,omitempty"`
	Workouts        []WearableWorkout         `json:"workouts,omitempty"`
	Events          []WearableDeviceEvent     `json:"events,omitempty"`
	RawSamples      []WearableRawSensorSample `json:"rawSamples,omitempty"`
}

type WearableActivitySummary struct {
	TotalSteps     *int     `json:"totalSteps,omitempty"`
	RunningSteps   *int     `json:"runningSteps,omitempty"`
	CaloriesRaw    *int     `json:"caloriesRaw,omitempty"`
	CaloriesKcal   *float64 `json:"caloriesKcal,omitempty"`
	DistanceM      *int     `json:"distanceM,omitempty"`
	SportDurationS *int     `json:"sportDurationS,omitempty"`
	SleepDurationS *int     `json:"sleepDurationS,omitempty"`
	Source         string   `json:"source,omitempty"`
}

type WearableActivityBucket struct {
	TimeIndex      *int     `json:"timeIndex"`
	WalkSteps      int      `json:"walkSteps"`
	RunSteps       int      `json:"runSteps"`
	CaloriesRaw    int      `json:"caloriesRaw"`
	CaloriesKcal   *float64 `json:"caloriesKcal,omitempty"`
	DistanceM      int      `json:"distanceM"`
	SportDurationS int      `json:"sportDurationS,omitempty"`
	Source         string   `json:"source,omitempty"`
}

type WearableMeasurement struct {
	Ts              string             `json:"ts,omitempty"`
	MinuteOfDay     *int               `json:"minuteOfDay,omitempty"`
	Metric          string             `json:"metric"`
	Value           *float64           `json:"value,omitempty"`
	Values          map[string]float64 `json:"values,omitempty"`
	RawValue        *float64           `json:"rawValue,omitempty"`
	RawValues       map[string]float64 `json:"rawValues,omitempty"`
	Unit            string             `json:"unit,omitempty"`
	MeasurementMode string             `json:"measurementMode,omitempty"`
	SessionID       string             `json:"sessionId,omitempty"`
	SampleIntervalS *int               `json:"sampleIntervalSec,omitempty"`
	ErrorCode       *int               `json:"errorCode,omitempty"`
	Derived         *bool              `json:"derived,omitempty"`
	Algorithm       string             `json:"algorithm,omitempty"`
	Source          string             `json:"source,omitempty"`
	Metadata        map[string]any     `json:"metadata,omitempty"`
}

type WearableSleepSession struct {
	SessionID   string                 `json:"sessionId,omitempty"`
	Type        string                 `json:"type,omitempty"`
	Protocol    string                 `json:"protocol,omitempty"`
	Start       string                 `json:"start"`
	End         string                 `json:"end,omitempty"`
	TotalSleepS *int                   `json:"totalSleepS,omitempty"`
	DeepS       *int                   `json:"deepS,omitempty"`
	LightS      *int                   `json:"lightS,omitempty"`
	RemS        *int                   `json:"remS,omitempty"`
	AwakeS      *int                   `json:"awakeS,omitempty"`
	WakingCount *int                   `json:"wakingCount,omitempty"`
	StageData   []int                  `json:"stageData,omitempty"`
	Segments    []WearableSleepSegment `json:"segments,omitempty"`
	Source      string                 `json:"source,omitempty"`
	Metadata    map[string]any         `json:"metadata,omitempty"`
}

type WearableSleepSegment struct {
	Start     string `json:"start,omitempty"`
	End       string `json:"end,omitempty"`
	DurationS *int   `json:"durationS,omitempty"`
	StageCode *int   `json:"stageCode,omitempty"`
	Stage     string `json:"stage,omitempty"`
}

type WearableTargetProgress struct {
	Kind   string   `json:"kind"`
	Value  *float64 `json:"value,omitempty"`
	Target *float64 `json:"target,omitempty"`
	Unit   string   `json:"unit,omitempty"`
	Source string   `json:"source,omitempty"`
}

type WearableWorkout struct {
	StartUTC      string                    `json:"startUtc"`
	EndUTC        string                    `json:"endUtc,omitempty"`
	SportTypeID   *int                      `json:"sportTypeId,omitempty"`
	SportTypeName string                    `json:"sportTypeName,omitempty"`
	DurationS     *int                      `json:"durationS,omitempty"`
	DistanceM     *int                      `json:"distanceM,omitempty"`
	Calories      *float64                  `json:"calories,omitempty"`
	AvgHR         *int                      `json:"avgHr,omitempty"`
	MaxHR         *int                      `json:"maxHr,omitempty"`
	MinHR         *int                      `json:"minHr,omitempty"`
	AvgSpeedCmS   *int                      `json:"avgSpeedCmS,omitempty"`
	MaxSpeedCmS   *int                      `json:"maxSpeedCmS,omitempty"`
	ElevationCm   *int                      `json:"elevationCm,omitempty"`
	UphillCm      *int                      `json:"uphillCm,omitempty"`
	DownhillCm    *int                      `json:"downhillCm,omitempty"`
	AvgCadenceSPM *int                      `json:"avgCadenceSpm,omitempty"`
	SportCount    *int                      `json:"sportCount,omitempty"`
	Steps         *int                      `json:"steps,omitempty"`
	Status        string                    `json:"status,omitempty"`
	Locations     []WearableWorkoutLocation `json:"locations,omitempty"`
	Source        string                    `json:"source,omitempty"`
	Metadata      map[string]any            `json:"metadata,omitempty"`
}

type WearableWorkoutLocation struct {
	RateReal *int `json:"rateReal,omitempty" bson:"rate_real,omitempty"`
}

type WearableDeviceEvent struct {
	Ts          string         `json:"ts,omitempty"`
	MinuteOfDay *int           `json:"minuteOfDay,omitempty"`
	EventType   string         `json:"eventType"`
	SessionID   string         `json:"sessionId,omitempty"`
	Source      string         `json:"source,omitempty"`
	Payload     map[string]any `json:"payload,omitempty"`
}

type WearableRawSensorSample struct {
	Ts          string             `json:"ts"`
	SessionID   string             `json:"sessionId,omitempty"`
	SampleIndex int                `json:"sampleIndex,omitempty"`
	Kind        string             `json:"kind,omitempty"`
	Channels    map[string]float64 `json:"channels"`
	Metadata    map[string]any     `json:"metadata,omitempty"`
}

type WearableSyncUpsertCounters struct {
	DeviceState     int `json:"deviceState"`
	DailyActivity   int `json:"dailyActivity"`
	ActivityBuckets int `json:"activityBuckets"`
	Measurements    int `json:"measurements"`
	SleepSessions   int `json:"sleepSessions"`
	SleepSegments   int `json:"sleepSegments"`
	SleepSummaries  int `json:"sleepSummaries"`
	Targets         int `json:"targets"`
	Workouts        int `json:"workouts"`
	Events          int `json:"events"`
	RawSamples      int `json:"rawSamples"`
	SyncMetadata    int `json:"syncMetadata"`
}

type WearableDaySyncResult struct {
	Date     string                     `json:"date"`
	Upserted WearableSyncUpsertCounters `json:"upserted"`
}

type WearableSyncResponse struct {
	Status   string                     `json:"status"`
	SyncID   string                     `json:"syncId"`
	Device   WearableDeviceView         `json:"device"`
	Days     []WearableDaySyncResult    `json:"days"`
	Upserted WearableSyncUpsertCounters `json:"upserted"`
}

type WearableDeviceView struct {
	ID             string          `json:"id"`
	DeviceUID      string          `json:"deviceUid"`
	Vendor         string          `json:"vendor,omitempty"`
	Model          string          `json:"model,omitempty"`
	HWRev          string          `json:"hwRev,omitempty"`
	FWRev          string          `json:"fwRev,omitempty"`
	SerialNumber   string          `json:"serialNumber,omitempty"`
	SDKVersion     string          `json:"sdkVersion,omitempty"`
	Capabilities   map[string]bool `json:"capabilities,omitempty"`
	BatteryPercent *int            `json:"batteryPercent,omitempty"`
	Charging       *bool           `json:"charging,omitempty"`
	FirstSeenAt    string          `json:"firstSeenAt,omitempty"`
	LastSeenAt     string          `json:"lastSeenAt,omitempty"`
	StateUpdatedAt string          `json:"stateUpdatedAt,omitempty"`
	CreatedAt      string          `json:"createdAt,omitempty"`
	UpdatedAt      string          `json:"updatedAt,omitempty"`
}

type WearablePairingView struct {
	ID         string             `json:"id"`
	UserID     string             `json:"userId"`
	DeviceID   string             `json:"deviceId"`
	DeviceUID  string             `json:"deviceUid"`
	Nickname   string             `json:"nickname,omitempty"`
	IsPrimary  bool               `json:"isPrimary"`
	PairedAt   string             `json:"pairedAt"`
	UnpairedAt string             `json:"unpairedAt,omitempty"`
	Device     WearableDeviceView `json:"device"`
}

type PairWearableDeviceParams struct {
	UserID    string
	DeviceUID string
	Nickname  string
	IsPrimary bool
}

type UpdateWearablePairingParams struct {
	Nickname  *string
	IsPrimary *bool
}

type WearableRangeResponse struct {
	Device WearableDeviceView `json:"device"`
	From   string             `json:"from"`
	To     string             `json:"to"`
	Days   []map[string]any   `json:"days"`
}

type UserWearableRangeDevice struct {
	Pairing WearablePairingView   `json:"pairing"`
	Range   WearableRangeResponse `json:"range"`
}

type UserWearableRangeResponse struct {
	UserID  string                    `json:"userId"`
	From    string                    `json:"from"`
	To      string                    `json:"to"`
	Devices []UserWearableRangeDevice `json:"devices"`
}
