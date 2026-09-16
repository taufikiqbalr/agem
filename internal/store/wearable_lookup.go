package store

import (
	"context"
	"errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"strings"
	"time"
)

func (s *Store) getWearableDeviceDocByUID(ctx context.Context, uid string) (*wearableV3DeviceDoc, error) {
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var doc wearableV3DeviceDoc
	if err := s.db.Collection("devices").FindOne(cctx, bson.M{"device_uid": strings.TrimSpace(uid)}).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &doc, nil
}

func (s *Store) getWearableDeviceDocByIDString(ctx context.Context, id string) (*wearableV3DeviceDoc, error) {
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
	return &doc, nil
}

func (s *Store) getWearablePairingView(ctx context.Context, userID, deviceID string, device wearableV3DeviceDoc) (*WearablePairingView, error) {
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var pair wearableV3PairingDoc
	if err := s.db.Collection("user_devices").FindOne(cctx, bson.M{"user_id": userID, "device_id": deviceID, "active": true}).Decode(&pair); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrPairingNotFound
		}
		return nil, err
	}
	view := wearableV3PairingDocToView(pair, device)
	return &view, nil
}

func parseDeviceDayStart(date string, tzOffsetMin int) (time.Time, error) {
	date = strings.TrimSpace(date)
	loc := time.FixedZone("device", tzOffsetMin*60)
	local, err := time.ParseInLocation(wearableDateLayout, date, loc)
	if err != nil {
		return time.Time{}, wearablePayloadError("invalid day date " + date)
	}
	return local.UTC(), nil
}

func resolveDayTimestamp(date string, tzOffsetMin int, explicit string, minuteOfDay *int) (time.Time, error) {
	if strings.TrimSpace(explicit) != "" {
		parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(explicit))
		if err != nil {
			return time.Time{}, wearablePayloadError("invalid timestamp")
		}
		parsed = parsed.UTC()
		if deviceDateForUTC(parsed, tzOffsetMin) != strings.TrimSpace(date) {
			return time.Time{}, wearablePayloadError("timestamp does not belong to day date in device timezone")
		}
		return parsed, nil
	}
	if minuteOfDay == nil || *minuteOfDay < 0 || *minuteOfDay > 1439 {
		return time.Time{}, wearablePayloadError("ts or minuteOfDay (0..1439) is required")
	}
	start, err := parseDeviceDayStart(date, tzOffsetMin)
	if err != nil {
		return time.Time{}, err
	}
	return start.Add(time.Duration(*minuteOfDay) * time.Minute), nil
}

func deviceDateForUTC(ts time.Time, tzOffsetMin int) string {
	return ts.UTC().Add(time.Duration(tzOffsetMin) * time.Minute).Format(wearableDateLayout)
}

func wearableV3DeviceDocToView(doc wearableV3DeviceDoc) WearableDeviceView {
	view := WearableDeviceView{
		ID:             doc.ID.Hex(),
		DeviceUID:      doc.DeviceUID,
		Vendor:         doc.Vendor,
		Model:          doc.Model,
		HWRev:          doc.HWRev,
		FWRev:          doc.FWRev,
		SerialNumber:   doc.SerialNumber,
		SDKVersion:     doc.SDKVersion,
		Capabilities:   doc.Capabilities,
		BatteryPercent: doc.BatteryPercent,
		Charging:       doc.Charging,
	}
	if !doc.FirstSeenAt.IsZero() {
		view.FirstSeenAt = doc.FirstSeenAt.UTC().Format(time.RFC3339)
	}
	if doc.LastSeenAt != nil {
		view.LastSeenAt = doc.LastSeenAt.UTC().Format(time.RFC3339)
	}
	if doc.StateUpdatedAt != nil {
		view.StateUpdatedAt = doc.StateUpdatedAt.UTC().Format(time.RFC3339)
	}
	if !doc.CreatedAt.IsZero() {
		view.CreatedAt = doc.CreatedAt.UTC().Format(time.RFC3339)
	}
	if !doc.UpdatedAt.IsZero() {
		view.UpdatedAt = doc.UpdatedAt.UTC().Format(time.RFC3339)
	}
	return view
}

func wearableV3PairingDocToView(pair wearableV3PairingDoc, device wearableV3DeviceDoc) WearablePairingView {
	view := WearablePairingView{
		ID:        pair.ID.Hex(),
		UserID:    pair.UserID,
		DeviceID:  pair.DeviceID,
		DeviceUID: device.DeviceUID,
		IsPrimary: pair.IsPrimary,
		PairedAt:  pair.PairedAt.UTC().Format(time.RFC3339),
		Device:    wearableV3DeviceDocToView(device),
	}
	if pair.Nickname != nil {
		view.Nickname = *pair.Nickname
	}
	if pair.UnpairedAt != nil {
		view.UnpairedAt = pair.UnpairedAt.UTC().Format(time.RFC3339)
	}
	return view
}
