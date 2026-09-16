package store

import (
	"context"
	"errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"strings"
	"time"
)

func (s *Store) GetWearableDeviceByUIDV3(ctx context.Context, uid string) (*WearableDeviceView, error) {
	doc, err := s.getWearableDeviceDocByUID(ctx, uid)
	if err != nil {
		return nil, err
	}
	view := wearableV3DeviceDocToView(*doc)
	return &view, nil
}

func (s *Store) GetWearableDeviceByIDV3(ctx context.Context, id string) (*WearableDeviceView, error) {
	oid, err := primitive.ObjectIDFromHex(strings.TrimSpace(id))
	if err != nil {
		return nil, ErrNotFound
	}
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var doc wearableV3DeviceDoc
	if err := s.db.Collection("devices").FindOne(cctx, bson.M{"_id": oid}).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	view := wearableV3DeviceDocToView(doc)
	return &view, nil
}

func (s *Store) PairWearableDeviceV3(ctx context.Context, p PairWearableDeviceParams) (*WearablePairingView, error) {
	p.UserID = strings.TrimSpace(p.UserID)
	p.DeviceUID = strings.TrimSpace(p.DeviceUID)
	if p.UserID == "" || p.DeviceUID == "" {
		return nil, wearablePayloadError("userId and deviceUid are required")
	}
	exists, err := s.UserExists(ctx, p.UserID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrNotFound
	}
	device, err := s.getWearableDeviceDocByUID(ctx, p.DeviceUID)
	if err != nil {
		return nil, err
	}
	deviceID := device.ID.Hex()
	now := time.Now().UTC()
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	col := s.db.Collection("user_devices")

	// Fail before changing the caller's current primary device if this wearable
	// already has another active owner. The partial unique index remains the
	// final race-condition guard.
	var activeOwner wearableV3PairingDoc
	ownerErr := col.FindOne(cctx, bson.M{"device_id": deviceID, "active": true}).Decode(&activeOwner)
	if ownerErr == nil && activeOwner.UserID != p.UserID {
		return nil, ErrDeviceAlreadyPaired
	}
	if ownerErr != nil && !errors.Is(ownerErr, mongo.ErrNoDocuments) {
		return nil, ownerErr
	}

	if p.IsPrimary {
		if _, err := col.UpdateMany(cctx, bson.M{"user_id": p.UserID, "active": true}, bson.M{"$set": bson.M{"is_primary": false}}); err != nil {
			return nil, err
		}
	}
	set := bson.M{"is_primary": p.IsPrimary, "active": true}
	if nickname := strings.TrimSpace(p.Nickname); nickname != "" {
		set["nickname"] = nickname
	}
	var pair wearableV3PairingDoc
	err = col.FindOneAndUpdate(
		cctx,
		bson.M{"user_id": p.UserID, "device_id": deviceID, "active": true},
		bson.M{
			"$set":         set,
			"$setOnInsert": bson.M{"_id": primitive.NewObjectID(), "user_id": p.UserID, "device_id": deviceID, "paired_at": now},
		},
		options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After),
	).Decode(&pair)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return nil, ErrDeviceAlreadyPaired
		}
		return nil, err
	}
	view := wearableV3PairingDocToView(pair, *device)
	return &view, nil
}

func (s *Store) ListWearableDevicesForUserV3(ctx context.Context, userID string) ([]WearablePairingView, error) {
	userID = strings.TrimSpace(userID)
	exists, err := s.UserExists(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrNotFound
	}
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cursor, err := s.db.Collection("user_devices").Find(
		cctx,
		bson.M{"user_id": userID, "active": true},
		options.Find().SetSort(bson.D{{Key: "is_primary", Value: -1}, {Key: "paired_at", Value: -1}}),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(cctx)
	out := make([]WearablePairingView, 0)
	for cursor.Next(cctx) {
		var pair wearableV3PairingDoc
		if err := cursor.Decode(&pair); err != nil {
			return nil, err
		}
		device, err := s.getWearableDeviceDocByIDString(cctx, pair.DeviceID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				continue
			}
			return nil, err
		}
		out = append(out, wearableV3PairingDocToView(pair, *device))
	}
	return out, cursor.Err()
}

func (s *Store) UpdateWearablePairingV3(ctx context.Context, userID, deviceUID string, p UpdateWearablePairingParams) (*WearablePairingView, error) {
	userID = strings.TrimSpace(userID)
	deviceUID = strings.TrimSpace(deviceUID)
	device, err := s.getWearableDeviceDocByUID(ctx, deviceUID)
	if err != nil {
		return nil, err
	}
	deviceID := device.ID.Hex()
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	col := s.db.Collection("user_devices")

	// Resolve the target first so a request for a nonexistent pairing cannot
	// accidentally demote the user's current primary wearable.
	var existing wearableV3PairingDoc
	if err := col.FindOne(cctx, bson.M{"user_id": userID, "device_id": deviceID, "active": true}).Decode(&existing); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrPairingNotFound
		}
		return nil, err
	}

	if p.IsPrimary != nil && *p.IsPrimary {
		if _, err := col.UpdateMany(
			cctx,
			bson.M{"user_id": userID, "active": true, "_id": bson.M{"$ne": existing.ID}},
			bson.M{"$set": bson.M{"is_primary": false}},
		); err != nil {
			return nil, err
		}
	}
	set := bson.M{}
	if p.Nickname != nil {
		set["nickname"] = strings.TrimSpace(*p.Nickname)
	}
	if p.IsPrimary != nil {
		set["is_primary"] = *p.IsPrimary
	}
	if len(set) == 0 {
		view := wearableV3PairingDocToView(existing, *device)
		return &view, nil
	}
	var pair wearableV3PairingDoc
	err = col.FindOneAndUpdate(
		cctx,
		bson.M{"_id": existing.ID, "active": true},
		bson.M{"$set": set},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&pair)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrPairingNotFound
		}
		if mongo.IsDuplicateKeyError(err) {
			return nil, ErrDeviceAlreadyPaired
		}
		return nil, err
	}
	view := wearableV3PairingDocToView(pair, *device)
	return &view, nil
}

func (s *Store) UnpairWearableDeviceV3(ctx context.Context, userID, deviceUID string) error {
	device, err := s.getWearableDeviceDocByUID(ctx, deviceUID)
	if err != nil {
		return err
	}
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	now := time.Now().UTC()
	res, err := s.db.Collection("user_devices").UpdateOne(
		cctx,
		bson.M{"user_id": strings.TrimSpace(userID), "device_id": device.ID.Hex(), "active": true},
		bson.M{"$set": bson.M{"active": false, "unpaired_at": now, "is_primary": false}},
	)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return ErrPairingNotFound
	}
	return nil
}
