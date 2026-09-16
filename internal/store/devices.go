package store

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type DeviceRow struct {
	ID           string  `json:"id"`
	Vendor       string  `json:"vendor"`
	Model        *string `json:"model,omitempty"`
	HwRev        *string `json:"hw_rev,omitempty"`
	FwRev        *string `json:"fw_rev,omitempty"`
	SerialNumber *string `json:"serial_number,omitempty"`
	DeviceUID    string  `json:"device_uid"`
	FirstSeenAt  string  `json:"first_seen_at"`
	LastSeenAt   *string `json:"last_seen_at,omitempty"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
}

type CreateDeviceParams struct {
	Vendor       string
	Model        string
	HwRev        string
	FwRev        string
	SerialNumber string
	DeviceUID    string
}

type UpdateDeviceParams struct {
	Vendor       *string
	Model        *string
	HwRev        *string
	FwRev        *string
	SerialNumber *string
	LastSeenAt   *string
}

type deviceDoc struct {
	ID           primitive.ObjectID `bson:"_id,omitempty"`
	Vendor       string             `bson:"vendor"`
	Model        *string            `bson:"model,omitempty"`
	HwRev        *string            `bson:"hw_rev,omitempty"`
	FwRev        *string            `bson:"fw_rev,omitempty"`
	SerialNumber *string            `bson:"serial_number,omitempty"`
	DeviceUID    string             `bson:"device_uid"`
	FirstSeenAt  time.Time          `bson:"first_seen_at"`
	LastSeenAt   *time.Time         `bson:"last_seen_at,omitempty"`
	CreatedAt    time.Time          `bson:"created_at"`
	UpdatedAt    time.Time          `bson:"updated_at"`
}

func (s *Store) CreateDevice(ctx context.Context, p CreateDeviceParams) (*DeviceRow, error) {
	cctx, cancel := ctxTimeout(ctx)
	defer cancel()

	now := time.Now().UTC()
	doc := deviceDoc{
		ID:           primitive.NewObjectID(),
		Vendor:       p.Vendor,
		Model:        optionalString(p.Model),
		HwRev:        optionalString(p.HwRev),
		FwRev:        optionalString(p.FwRev),
		SerialNumber: optionalString(p.SerialNumber),
		DeviceUID:    p.DeviceUID,
		FirstSeenAt:  now,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if _, err := s.db.Collection("devices").InsertOne(cctx, doc); err != nil {
		return nil, err
	}
	return deviceDocToRow(doc), nil
}

func (s *Store) GetDevice(ctx context.Context, id string) (*DeviceRow, error) {
	cctx, cancel := ctxTimeout(ctx)
	defer cancel()

	oid, err := objectIDFromString(id)
	if err != nil {
		return nil, err
	}

	var doc deviceDoc
	if err := s.db.Collection("devices").FindOne(cctx, bson.M{"_id": oid}).Decode(&doc); err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return deviceDocToRow(doc), nil
}

func (s *Store) GetDeviceByUID(ctx context.Context, uid string) (*DeviceRow, error) {
	cctx, cancel := ctxTimeout(ctx)
	defer cancel()

	var doc deviceDoc
	if err := s.db.Collection("devices").FindOne(cctx, bson.M{"device_uid": uid}).Decode(&doc); err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return deviceDocToRow(doc), nil
}

func (s *Store) UpdateDevice(ctx context.Context, id string, p UpdateDeviceParams) (*DeviceRow, error) {
	cctx, cancel := ctxTimeout(ctx)
	defer cancel()

	oid, err := objectIDFromString(id)
	if err != nil {
		return nil, err
	}

	set := bson.M{"updated_at": time.Now().UTC()}
	if p.Vendor != nil {
		set["vendor"] = *p.Vendor
	}
	if p.Model != nil {
		set["model"] = *p.Model
	}
	if p.HwRev != nil {
		set["hw_rev"] = *p.HwRev
	}
	if p.FwRev != nil {
		set["fw_rev"] = *p.FwRev
	}
	if p.SerialNumber != nil {
		set["serial_number"] = *p.SerialNumber
	}
	if p.LastSeenAt != nil && *p.LastSeenAt != "" {
		lastSeen, err := time.Parse(time.RFC3339, *p.LastSeenAt)
		if err != nil {
			return nil, err
		}
		set["last_seen_at"] = lastSeen
	}

	var doc deviceDoc
	err = s.db.Collection("devices").FindOneAndUpdate(
		cctx,
		bson.M{"_id": oid},
		bson.M{"$set": set},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&doc)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return deviceDocToRow(doc), nil
}

func (s *Store) DeleteDevice(ctx context.Context, id string) error {
	cctx, cancel := ctxTimeout(ctx)
	defer cancel()

	oid, err := objectIDFromString(id)
	if err != nil {
		return err
	}

	res, err := s.db.Collection("devices").DeleteOne(cctx, bson.M{"_id": oid})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return ErrNotFound
	}
	return nil
}

func deviceDocToRow(doc deviceDoc) *DeviceRow {
	row := &DeviceRow{
		ID:           objectIDString(doc.ID),
		Vendor:       doc.Vendor,
		Model:        doc.Model,
		HwRev:        doc.HwRev,
		FwRev:        doc.FwRev,
		SerialNumber: doc.SerialNumber,
		DeviceUID:    doc.DeviceUID,
		FirstSeenAt:  doc.FirstSeenAt.Format(time.RFC3339),
		CreatedAt:    doc.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    doc.UpdatedAt.Format(time.RFC3339),
	}
	if doc.LastSeenAt != nil {
		lastSeen := doc.LastSeenAt.Format(time.RFC3339)
		row.LastSeenAt = &lastSeen
	}
	return row
}
