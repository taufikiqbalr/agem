package handlers

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"agem/internal/httpx"
	"agem/internal/store"
)

type WorkoutRequest struct {
	StartUTC      string                  `json:"start_utc"` // RFC3339
	EndUTC        string                  `json:"end_utc"`   // RFC3339 optional
	TzOffsetMin   int                     `json:"tz_offset_min"`
	SportTypeID   *int                    `json:"sport_type_id"`
	DurationS     *int                    `json:"duration_s"`
	DistanceM     *int                    `json:"distance_m"`
	Calories      *float64                `json:"calories"`
	AvgHR         *int                    `json:"avg_hr"`
	MaxHR         *int                    `json:"max_hr"`
	MinHR         *int                    `json:"min_hr"`
	AvgSpeedCmS   *int                    `json:"avg_speed_cm_s"`
	MaxSpeedCmS   *int                    `json:"max_speed_cm_s"`
	ElevationCm   *int                    `json:"elevation_cm"`
	UphillCm      *int                    `json:"uphill_cm"`
	DownhillCm    *int                    `json:"downhill_cm"`
	AvgCadenceSPM *int                    `json:"avg_cadence_spm"`
	SportCount    *int                    `json:"sport_count"`
	Steps         *int                    `json:"steps"`
	Locations     []store.WorkoutLocation `json:"locations"`
	Source        string                  `json:"source"`
}

// CreateWorkout godoc
//
// @Summary Create workout
// @Description Creates or replaces a workout keyed by device ID and start_utc.
// @Tags Workouts
// @Accept json
// @Produce json
// @Param device_id path string true "Device ID"
// @Param request body WorkoutRequest true "Workout payload"
// @Success 201 {object} store.WorkoutResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /v1/devices/{device_id}/workouts [post]
func (h *Handler) CreateWorkout(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "device_id")

	var req WorkoutRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Err(w, 400, "invalid json")
		return
	}
	start, err := time.Parse(time.RFC3339, req.StartUTC)
	if err != nil {
		httpx.Err(w, 400, "invalid start_utc")
		return
	}
	var endPtr *time.Time
	if req.EndUTC != "" {
		et, err := time.Parse(time.RFC3339, req.EndUTC)
		if err != nil {
			httpx.Err(w, 400, "invalid end_utc")
			return
		}
		endPtr = &et
	}

	row, err := h.st.UpsertWorkout(r.Context(), store.WorkoutRow{
		DeviceID:      deviceID,
		StartUTC:      start,
		EndUTC:        endPtr,
		TzOffsetMin:   req.TzOffsetMin,
		SportTypeID:   req.SportTypeID,
		DurationS:     req.DurationS,
		DistanceM:     req.DistanceM,
		Calories:      req.Calories,
		AvgHR:         req.AvgHR,
		MaxHR:         req.MaxHR,
		MinHR:         req.MinHR,
		AvgSpeedCmS:   req.AvgSpeedCmS,
		MaxSpeedCmS:   req.MaxSpeedCmS,
		ElevationCm:   req.ElevationCm,
		UphillCm:      req.UphillCm,
		DownhillCm:    req.DownhillCm,
		AvgCadenceSPM: req.AvgCadenceSPM,
		SportCount:    req.SportCount,
		Steps:         req.Steps,
		Locations:     req.Locations,
		Source:        req.Source,
	})
	if err != nil {
		httpx.Err(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 201, row)
}

// ListWorkouts godoc
//
// @Summary List workouts
// @Description Lists workouts in an RFC3339 time range.
// @Tags Workouts
// @Produce json
// @Param device_id path string true "Device ID"
// @Param from query string true "Inclusive RFC3339 start timestamp" Format(date-time)
// @Param to query string true "Exclusive RFC3339 end timestamp" Format(date-time)
// @Success 200 {array} store.WorkoutResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /v1/devices/{device_id}/workouts [get]
func (h *Handler) ListWorkouts(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "device_id")
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	if from == "" || to == "" {
		httpx.Err(w, 400, "from and to (RFC3339) required")
		return
	}
	ft, err := time.Parse(time.RFC3339, from)
	if err != nil {
		httpx.Err(w, 400, "invalid from")
		return
	}
	tt, err := time.Parse(time.RFC3339, to)
	if err != nil {
		httpx.Err(w, 400, "invalid to")
		return
	}

	rows, err := h.st.ListWorkouts(r.Context(), deviceID, ft, tt)
	if err != nil {
		httpx.Err(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 200, rows)
}

// UpdateWorkoutByStartUTC godoc
//
// @Summary Update workout
// @Description Updates a workout identified by device ID and start_utc in the request body.
// @Tags Workouts
// @Accept json
// @Produce json
// @Param device_id path string true "Device ID"
// @Param request body WorkoutRequest true "Workout update payload"
// @Success 200 {object} store.WorkoutResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /v1/devices/{device_id}/workouts [patch]
func (h *Handler) UpdateWorkoutByStartUTC(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "device_id")
	var req WorkoutRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Err(w, 400, "invalid json")
		return
	}
	start, err := time.Parse(time.RFC3339, req.StartUTC)
	if err != nil {
		httpx.Err(w, 400, "invalid start_utc")
		return
	}

	row, err := h.st.PatchWorkout(r.Context(), deviceID, start, store.PatchWorkoutParams{
		EndUTC:        req.EndUTC,
		TzOffsetMin:   &req.TzOffsetMin,
		SportTypeID:   req.SportTypeID,
		DurationS:     req.DurationS,
		DistanceM:     req.DistanceM,
		Calories:      req.Calories,
		AvgHR:         req.AvgHR,
		MaxHR:         req.MaxHR,
		MinHR:         req.MinHR,
		AvgSpeedCmS:   req.AvgSpeedCmS,
		MaxSpeedCmS:   req.MaxSpeedCmS,
		ElevationCm:   req.ElevationCm,
		UphillCm:      req.UphillCm,
		DownhillCm:    req.DownhillCm,
		AvgCadenceSPM: req.AvgCadenceSPM,
		SportCount:    req.SportCount,
		Steps:         req.Steps,
		Locations:     req.Locations,
		Source:        &req.Source,
	})
	if err != nil {
		if err == store.ErrNotFound {
			httpx.Err(w, 404, "not found")
			return
		}
		httpx.Err(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 200, row)
}

// DeleteWorkoutByStartUTC godoc
//
// @Summary Delete workout
// @Description Deletes a workout identified by device ID and start_utc query parameter.
// @Tags Workouts
// @Produce json
// @Param device_id path string true "Device ID"
// @Param start_utc query string true "Workout start timestamp" Format(date-time)
// @Success 200 {object} StatusResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /v1/devices/{device_id}/workouts [delete]
func (h *Handler) DeleteWorkoutByStartUTC(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "device_id")
	startStr := r.URL.Query().Get("start_utc")
	if startStr == "" {
		httpx.Err(w, 400, "start_utc (RFC3339) required")
		return
	}
	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		httpx.Err(w, 400, "invalid start_utc")
		return
	}

	if err := h.st.DeleteWorkout(r.Context(), deviceID, start); err != nil {
		if err == store.ErrNotFound {
			httpx.Err(w, 404, "not found")
			return
		}
		httpx.Err(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 200, map[string]string{"status": "deleted"})
}
