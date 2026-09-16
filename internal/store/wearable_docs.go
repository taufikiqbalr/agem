package store

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

type wearableV3DeviceDoc struct {
	ID             primitive.ObjectID `bson:"_id,omitempty"`
	DeviceUID      string             `bson:"device_uid"`
	Vendor         string             `bson:"vendor,omitempty"`
	Model          string             `bson:"model,omitempty"`
	HWRev          string             `bson:"hw_rev,omitempty"`
	FWRev          string             `bson:"fw_rev,omitempty"`
	SerialNumber   string             `bson:"serial_number,omitempty"`
	SDKVersion     string             `bson:"sdk_version,omitempty"`
	Capabilities   map[string]bool    `bson:"capabilities,omitempty"`
	BatteryPercent *int               `bson:"battery_percent,omitempty"`
	Charging       *bool              `bson:"charging,omitempty"`
	FirstSeenAt    time.Time          `bson:"first_seen_at,omitempty"`
	LastSeenAt     *time.Time         `bson:"last_seen_at,omitempty"`
	StateUpdatedAt *time.Time         `bson:"state_updated_at,omitempty"`
	CreatedAt      time.Time          `bson:"created_at,omitempty"`
	UpdatedAt      time.Time          `bson:"updated_at,omitempty"`
}

type wearableV3PairingDoc struct {
	ID         primitive.ObjectID `bson:"_id,omitempty"`
	UserID     string             `bson:"user_id"`
	DeviceID   string             `bson:"device_id"`
	Nickname   *string            `bson:"nickname,omitempty"`
	IsPrimary  bool               `bson:"is_primary"`
	Active     bool               `bson:"active"`
	PairedAt   time.Time          `bson:"paired_at"`
	UnpairedAt *time.Time         `bson:"unpaired_at,omitempty"`
}

// EnsureSDKV3Schema adds the v3-specific regular collections, time-series
// collection, idempotency key collections, and query/ownership indexes. The
// base schema still comes from EnsureIndexes so v1 storage remains readable.
