package handlers

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"agem/internal/httpx"
	"agem/internal/store"
)

type Step15mPoint struct {
	TsUTC          string `json:"ts_utc"`      // RFC3339
	DeviceDate     string `json:"device_date"` // YYYY-MM-DD (device-local)
	TimeIndex      int    `json:"time_index"`  // 0..95
	TzOffsetMin    int    `json:"tz_offset_min"`
	WalkSteps      int    `json:"walk_steps"`
	RunSteps       int    `json:"run_steps"`
	Calories       int    `json:"calories"`
	DistanceM      int    `json:"distance_m"`
	SportDurationS int    `json:"sport_duration_s"` // active duration in seconds
	Source         string `json:"source"`           // device/derived/manual/import (optional)
}

type IngestSteps15mRequest struct {
	Points []Step15mPoint `json:"points"`
}

// IngestSteps15mBulk godoc
//
// @Summary Ingest 15-minute steps
// @Description Idempotently ingests 15-minute qring step detail points.
// @Tags Steps
// @Accept json
// @Produce json
// @Param device_id path string true "Device ID"
// @Param request body IngestSteps15mRequest true "Step points"
// @Success 200 {object} UpsertResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /v1/devices/{device_id}/steps15m [post]
func (h *Handler) IngestSteps15mBulk(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "device_id")

	var req IngestSteps15mRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Err(w, 400, "invalid json")
		return
	}
	if len(req.Points) == 0 {
		httpx.Err(w, 400, "points is required")
		return
	}

	points := make([]store.Step15mRow, 0, len(req.Points))
	for _, p := range req.Points {
		ts, err := time.Parse(time.RFC3339, p.TsUTC)
		if err != nil {
			httpx.Err(w, 400, "invalid ts_utc")
			return
		}
		points = append(points, store.Step15mRow{
			DeviceID:       deviceID,
			TsUTC:          ts,
			DeviceDate:     p.DeviceDate,
			TimeIndex:      p.TimeIndex,
			TzOffsetMin:    p.TzOffsetMin,
			WalkSteps:      p.WalkSteps,
			RunSteps:       p.RunSteps,
			Calories:       p.Calories,
			DistanceM:      p.DistanceM,
			SportDurationS: p.SportDurationS,
			Source:         p.Source,
		})
	}

	n, err := h.st.UpsertSteps15mBulk(r.Context(), points)
	if err != nil {
		httpx.Err(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 200, map[string]any{"upserted": n})
}

// ListSteps15mByDate godoc
//
// @Summary List 15-minute steps
// @Description Lists 15-minute step points by device-local date.
// @Tags Steps
// @Produce json
// @Param device_id path string true "Device ID"
// @Param date query string true "Device-local date" Format(date)
// @Success 200 {array} store.Steps15mResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /v1/devices/{device_id}/steps15m [get]
func (h *Handler) ListSteps15mByDate(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "device_id")
	date := r.URL.Query().Get("date") // YYYY-MM-DD device_date
	if date == "" {
		httpx.Err(w, 400, "date=YYYY-MM-DD required")
		return
	}
	rows, err := h.st.ListSteps15mByDeviceDate(r.Context(), deviceID, date)
	if err != nil {
		httpx.Err(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 200, rows)
}

// ListStepsDaily godoc
//
// @Summary Aggregate daily steps
// @Description Aggregates daily step totals from the steps_15m time series collection.
// @Tags Steps
// @Produce json
// @Param device_id path string true "Device ID"
// @Param from query string true "Inclusive RFC3339 start timestamp" Format(date-time)
// @Param to query string true "Exclusive RFC3339 end timestamp" Format(date-time)
// @Success 200 {array} store.StepsDailyRow
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /v1/devices/{device_id}/steps/daily [get]
func (h *Handler) ListStepsDaily(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "device_id")
	from := r.URL.Query().Get("from") // RFC3339
	to := r.URL.Query().Get("to")     // RFC3339

	if from == "" || to == "" {
		httpx.Err(w, 400, "from and to (RFC3339) required")
		return
	}
	fromT, err := time.Parse(time.RFC3339, from)
	if err != nil {
		httpx.Err(w, 400, "invalid from")
		return
	}
	toT, err := time.Parse(time.RFC3339, to)
	if err != nil {
		httpx.Err(w, 400, "invalid to")
		return
	}

	rows, err := h.st.ListStepsDailyCagg(r.Context(), deviceID, fromT, toT)
	if err != nil {
		httpx.Err(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 200, rows)
}
