package store

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type UserDeviceRow struct {
	ID         string  `json:"id"`
	UserID     string  `json:"user_id"`
	DeviceID   string  `json:"device_id"`
	Nickname   *string `json:"nickname,omitempty"`
	IsPrimary  bool    `json:"is_primary"`
	PairedAt   string  `json:"paired_at"`
	UnpairedAt *string `json:"unpaired_at,omitempty"`
}

type PairDeviceParams struct {
	UserID    string
	DeviceID  string
	Nickname  string
	IsPrimary bool
}

type UpdateUserDeviceParams struct {
	Nickname   *string
	IsPrimary  *bool
	UnpairedAt *string
}

type userDeviceDoc struct {
	ID         primitive.ObjectID `bson:"_id,omitempty"`
	UserID     string             `bson:"user_id"`
	DeviceID   string             `bson:"device_id"`
	Nickname   *string            `bson:"nickname,omitempty"`
	IsPrimary  bool               `bson:"is_primary"`
	Active     bool               `bson:"active"`
	PairedAt   time.Time          `bson:"paired_at"`
	UnpairedAt *time.Time         `bson:"unpaired_at,omitempty"`
}

func (s *Store) PairDeviceToUser(ctx context.Context, p PairDeviceParams) (*UserDeviceRow, error) {
	exists, err := s.UserExists(ctx, p.UserID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrNotFound
	}
	// Do not create orphan pairing rows. v1 keeps using the internal device ID,
	// while canonical v3 pairing accepts device_uid and resolves it first.
	if _, err := s.GetDevice(ctx, p.DeviceID); err != nil {
		return nil, err
	}

	cctx, cancel := ctxTimeout(ctx)
	defer cancel()

	col := s.db.Collection("user_devices")
	if p.IsPrimary {
		if _, err := col.UpdateMany(cctx, bson.M{"user_id": p.UserID, "active": true}, bson.M{"$set": bson.M{"is_primary": false}}); err != nil {
			return nil, err
		}
	}

	doc := userDeviceDoc{
		ID:        primitive.NewObjectID(),
		UserID:    p.UserID,
		DeviceID:  p.DeviceID,
		Nickname:  optionalString(p.Nickname),
		IsPrimary: p.IsPrimary,
		Active:    true,
		PairedAt:  time.Now().UTC(),
	}
	if _, err := col.InsertOne(cctx, doc); err != nil {
		return nil, err
	}
	return userDeviceDocToRow(doc), nil
}

func (s *Store) ListUserDevices(ctx context.Context, userID string) ([]UserDeviceRow, error) {
	exists, err := s.UserExists(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrNotFound
	}

	cctx, cancel := ctxTimeout(ctx)
	defer cancel()

	cursor, err := s.db.Collection("user_devices").Find(
		cctx,
		bson.M{"user_id": userID},
		options.Find().SetSort(bson.D{{Key: "is_primary", Value: -1}, {Key: "paired_at", Value: -1}}),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(cctx)

	var out []UserDeviceRow
	for cursor.Next(cctx) {
		var doc userDeviceDoc
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		out = append(out, *userDeviceDocToRow(doc))
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) UpdateUserDevice(ctx context.Context, id string, p UpdateUserDeviceParams) (*UserDeviceRow, error) {
	cctx, cancel := ctxTimeout(ctx)
	defer cancel()

	oid, err := objectIDFromString(id)
	if err != nil {
		return nil, err
	}

	col := s.db.Collection("user_devices")
	if p.IsPrimary != nil && *p.IsPrimary {
		var existing userDeviceDoc
		if err := col.FindOne(cctx, bson.M{"_id": oid}).Decode(&existing); err != nil {
			if isNotFound(err) {
				return nil, ErrNotFound
			}
			return nil, err
		}
		if _, err := col.UpdateMany(cctx, bson.M{"user_id": existing.UserID, "active": true}, bson.M{"$set": bson.M{"is_primary": false}}); err != nil {
			return nil, err
		}
	}

	set := bson.M{}
	if p.Nickname != nil {
		set["nickname"] = *p.Nickname
	}
	if p.IsPrimary != nil {
		set["is_primary"] = *p.IsPrimary
	}
	if p.UnpairedAt != nil && *p.UnpairedAt != "" {
		unpaired, err := time.Parse(time.RFC3339, *p.UnpairedAt)
		if err != nil {
			return nil, err
		}
		set["unpaired_at"] = unpaired.UTC()
		set["active"] = false
		set["is_primary"] = false
	}
	var doc userDeviceDoc
	if len(set) == 0 {
		if err := col.FindOne(cctx, bson.M{"_id": oid}).Decode(&doc); err != nil {
			if isNotFound(err) {
				return nil, ErrNotFound
			}
			return nil, err
		}
		return userDeviceDocToRow(doc), nil
	}

	err = col.FindOneAndUpdate(
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
	return userDeviceDocToRow(doc), nil
}

func (s *Store) DeleteUserDevice(ctx context.Context, id string) error {
	cctx, cancel := ctxTimeout(ctx)
	defer cancel()

	oid, err := objectIDFromString(id)
	if err != nil {
		return err
	}

	res, err := s.db.Collection("user_devices").DeleteOne(cctx, bson.M{"_id": oid})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return ErrNotFound
	}
	return nil
}

func userDeviceDocToRow(doc userDeviceDoc) *UserDeviceRow {
	row := &UserDeviceRow{
		ID:        objectIDString(doc.ID),
		UserID:    doc.UserID,
		DeviceID:  doc.DeviceID,
		Nickname:  doc.Nickname,
		IsPrimary: doc.IsPrimary,
		PairedAt:  doc.PairedAt.Format(time.RFC3339),
	}
	if doc.UnpairedAt != nil {
		unpairedAt := doc.UnpairedAt.Format(time.RFC3339)
		row.UnpairedAt = &unpairedAt
	}
	return row
}
