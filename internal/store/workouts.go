package store

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type WorkoutRow struct {
	DeviceID      string
	StartUTC      time.Time
	EndUTC        *time.Time
	TzOffsetMin   int
	SportTypeID   *int
	DurationS     *int
	DistanceM     *int
	Calories      *float64
	AvgHR         *int
	MaxHR         *int
	MinHR         *int
	AvgSpeedCmS   *int
	MaxSpeedCmS   *int
	ElevationCm   *int
	UphillCm      *int
	DownhillCm    *int
	AvgCadenceSPM *int
	SportCount    *int
	Steps         *int
	Locations     []WorkoutLocation
	Source        string
}

type WorkoutResponse struct {
	DeviceID      string            `json:"device_id"`
	StartUTC      string            `json:"start_utc"`
	EndUTC        *string           `json:"end_utc,omitempty"`
	TzOffsetMin   int               `json:"tz_offset_min"`
	SportTypeID   *int              `json:"sport_type_id,omitempty"`
	DurationS     *int              `json:"duration_s,omitempty"`
	DistanceM     *int              `json:"distance_m,omitempty"`
	Calories      *float64          `json:"calories,omitempty"`
	AvgHR         *int              `json:"avg_hr,omitempty"`
	MaxHR         *int              `json:"max_hr,omitempty"`
	MinHR         *int              `json:"min_hr,omitempty"`
	AvgSpeedCmS   *int              `json:"avg_speed_cm_s,omitempty"`
	MaxSpeedCmS   *int              `json:"max_speed_cm_s,omitempty"`
	ElevationCm   *int              `json:"elevation_cm,omitempty"`
	UphillCm      *int              `json:"uphill_cm,omitempty"`
	DownhillCm    *int              `json:"downhill_cm,omitempty"`
	AvgCadenceSPM *int              `json:"avg_cadence_spm,omitempty"`
	SportCount    *int              `json:"sport_count,omitempty"`
	Steps         *int              `json:"steps,omitempty"`
	Locations     []WorkoutLocation `json:"locations,omitempty"`
	Source        string            `json:"source"`
	SyncedAt      string            `json:"synced_at"`
}

type PatchWorkoutParams struct {
	EndUTC        string
	TzOffsetMin   *int
	SportTypeID   *int
	DurationS     *int
	DistanceM     *int
	Calories      *float64
	AvgHR         *int
	MaxHR         *int
	MinHR         *int
	AvgSpeedCmS   *int
	MaxSpeedCmS   *int
	ElevationCm   *int
	UphillCm      *int
	DownhillCm    *int
	AvgCadenceSPM *int
	SportCount    *int
	Steps         *int
	Locations     []WorkoutLocation
	Source        *string
}

type WorkoutLocation struct {
	RateReal *int `json:"rate_real,omitempty" bson:"rate_real,omitempty"`
}

type workoutDoc struct {
	DeviceID      string            `bson:"device_id"`
	StartUTC      time.Time         `bson:"start_utc"`
	EndUTC        *time.Time        `bson:"end_utc,omitempty"`
	TzOffsetMin   int               `bson:"tz_offset_min"`
	SportTypeID   *int              `bson:"sport_type_id,omitempty"`
	DurationS     *int              `bson:"duration_s,omitempty"`
	DistanceM     *int              `bson:"distance_m,omitempty"`
	Calories      *float64          `bson:"calories,omitempty"`
	AvgHR         *int              `bson:"avg_hr,omitempty"`
	MaxHR         *int              `bson:"max_hr,omitempty"`
	MinHR         *int              `bson:"min_hr,omitempty"`
	AvgSpeedCmS   *int              `bson:"avg_speed_cm_s,omitempty"`
	MaxSpeedCmS   *int              `bson:"max_speed_cm_s,omitempty"`
	ElevationCm   *int              `bson:"elevation_cm,omitempty"`
	UphillCm      *int              `bson:"uphill_cm,omitempty"`
	DownhillCm    *int              `bson:"downhill_cm,omitempty"`
	AvgCadenceSPM *int              `bson:"avg_cadence_spm,omitempty"`
	SportCount    *int              `bson:"sport_count,omitempty"`
	Steps         *int              `bson:"steps,omitempty"`
	Locations     []WorkoutLocation `bson:"locations,omitempty"`
	Source        string            `bson:"source"`
	SyncedAt      time.Time         `bson:"synced_at"`
}

func (s *Store) UpsertWorkout(ctx context.Context, w WorkoutRow) (*WorkoutResponse, error) {
	cctx, cancel := ctxTimeout(ctx)
	defer cancel()

	src := w.Source
	if src == "" {
		src = "device"
	}
	set := bson.M{
		"device_id":       w.DeviceID,
		"start_utc":       w.StartUTC.UTC(),
		"end_utc":         w.EndUTC,
		"tz_offset_min":   w.TzOffsetMin,
		"sport_type_id":   w.SportTypeID,
		"duration_s":      w.DurationS,
		"distance_m":      w.DistanceM,
		"calories":        w.Calories,
		"avg_hr":          w.AvgHR,
		"max_hr":          w.MaxHR,
		"min_hr":          w.MinHR,
		"avg_speed_cm_s":  w.AvgSpeedCmS,
		"max_speed_cm_s":  w.MaxSpeedCmS,
		"elevation_cm":    w.ElevationCm,
		"uphill_cm":       w.UphillCm,
		"downhill_cm":     w.DownhillCm,
		"avg_cadence_spm": w.AvgCadenceSPM,
		"sport_count":     w.SportCount,
		"steps":           w.Steps,
		"locations":       w.Locations,
		"source":          src,
		"synced_at":       time.Now().UTC(),
	}

	var doc workoutDoc
	err := s.db.Collection("workouts").FindOneAndUpdate(
		cctx,
		bson.M{"device_id": w.DeviceID, "start_utc": w.StartUTC.UTC()},
		bson.M{"$set": set},
		options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After),
	).Decode(&doc)
	if err != nil {
		return nil, err
	}
	return workoutDocToResponse(doc), nil
}

func (s *Store) ListWorkouts(ctx context.Context, deviceID string, from, to time.Time) ([]WorkoutResponse, error) {
	cctx, cancel := ctxTimeout(ctx)
	defer cancel()

	cursor, err := s.db.Collection("workouts").Find(
		cctx,
		bson.M{
			"device_id": deviceID,
			"start_utc": bson.M{
				"$gte": from.UTC(),
				"$lt":  to.UTC(),
			},
		},
		options.Find().SetSort(bson.D{{Key: "start_utc", Value: -1}}),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(cctx)

	var out []WorkoutResponse
	for cursor.Next(cctx) {
		var doc workoutDoc
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		out = append(out, *workoutDocToResponse(doc))
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) PatchWorkout(ctx context.Context, deviceID string, startUTC time.Time, p PatchWorkoutParams) (*WorkoutResponse, error) {
	cctx, cancel := ctxTimeout(ctx)
	defer cancel()

	set := bson.M{"synced_at": time.Now().UTC()}
	if p.EndUTC != "" {
		end, err := time.Parse(time.RFC3339, p.EndUTC)
		if err != nil {
			return nil, err
		}
		set["end_utc"] = end.UTC()
	}
	if p.TzOffsetMin != nil {
		set["tz_offset_min"] = *p.TzOffsetMin
	}
	if p.SportTypeID != nil {
		set["sport_type_id"] = *p.SportTypeID
	}
	if p.DurationS != nil {
		set["duration_s"] = *p.DurationS
	}
	if p.DistanceM != nil {
		set["distance_m"] = *p.DistanceM
	}
	if p.Calories != nil {
		set["calories"] = *p.Calories
	}
	if p.AvgHR != nil {
		set["avg_hr"] = *p.AvgHR
	}
	if p.MaxHR != nil {
		set["max_hr"] = *p.MaxHR
	}
	if p.MinHR != nil {
		set["min_hr"] = *p.MinHR
	}
	if p.AvgSpeedCmS != nil {
		set["avg_speed_cm_s"] = *p.AvgSpeedCmS
	}
	if p.MaxSpeedCmS != nil {
		set["max_speed_cm_s"] = *p.MaxSpeedCmS
	}
	if p.ElevationCm != nil {
		set["elevation_cm"] = *p.ElevationCm
	}
	if p.UphillCm != nil {
		set["uphill_cm"] = *p.UphillCm
	}
	if p.DownhillCm != nil {
		set["downhill_cm"] = *p.DownhillCm
	}
	if p.AvgCadenceSPM != nil {
		set["avg_cadence_spm"] = *p.AvgCadenceSPM
	}
	if p.SportCount != nil {
		set["sport_count"] = *p.SportCount
	}
	if p.Steps != nil {
		set["steps"] = *p.Steps
	}
	if p.Locations != nil {
		set["locations"] = p.Locations
	}
	if p.Source != nil && *p.Source != "" {
		set["source"] = *p.Source
	}

	var doc workoutDoc
	err := s.db.Collection("workouts").FindOneAndUpdate(
		cctx,
		bson.M{"device_id": deviceID, "start_utc": startUTC.UTC()},
		bson.M{"$set": set},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&doc)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return workoutDocToResponse(doc), nil
}

func (s *Store) DeleteWorkout(ctx context.Context, deviceID string, startUTC time.Time) error {
	cctx, cancel := ctxTimeout(ctx)
	defer cancel()

	res, err := s.db.Collection("workouts").DeleteOne(cctx, bson.M{"device_id": deviceID, "start_utc": startUTC.UTC()})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return ErrNotFound
	}
	return nil
}

func workoutDocToResponse(doc workoutDoc) *WorkoutResponse {
	row := &WorkoutResponse{
		DeviceID:      doc.DeviceID,
		StartUTC:      doc.StartUTC.Format(time.RFC3339),
		TzOffsetMin:   doc.TzOffsetMin,
		SportTypeID:   doc.SportTypeID,
		DurationS:     doc.DurationS,
		DistanceM:     doc.DistanceM,
		Calories:      doc.Calories,
		AvgHR:         doc.AvgHR,
		MaxHR:         doc.MaxHR,
		MinHR:         doc.MinHR,
		AvgSpeedCmS:   doc.AvgSpeedCmS,
		MaxSpeedCmS:   doc.MaxSpeedCmS,
		ElevationCm:   doc.ElevationCm,
		UphillCm:      doc.UphillCm,
		DownhillCm:    doc.DownhillCm,
		AvgCadenceSPM: doc.AvgCadenceSPM,
		SportCount:    doc.SportCount,
		Steps:         doc.Steps,
		Locations:     doc.Locations,
		Source:        doc.Source,
		SyncedAt:      doc.SyncedAt.Format(time.RFC3339),
	}
	if doc.EndUTC != nil {
		end := doc.EndUTC.Format(time.RFC3339)
		row.EndUTC = &end
	}
	return row
}
