package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"agem/internal/httpx"
	"agem/internal/store"
)

type HealthMetricPoint struct {
	TsUTC    string             `json:"ts_utc"`
	Metric   string             `json:"metric"`
	Value    *float64           `json:"value"`
	Values   map[string]float64 `json:"values"`
	Unit     string             `json:"unit"`
	Source   string             `json:"source"`
	Metadata map[string]any     `json:"metadata"`
}

type IngestHealthMetricsRequest struct {
	Points []HealthMetricPoint `json:"points"`
}

type DailyActivityRequest struct {
	DeviceDate     string `json:"device_date"`
	TzOffsetMin    int    `json:"tz_offset_min"`
	TotalSteps     int    `json:"total_steps"`
	RunningSteps   int    `json:"running_steps"`
	Calories       int    `json:"calories"`
	WalkDistanceM  int    `json:"walk_distance_m"`
	SportDurationS int    `json:"sport_duration_s"`
	SleepDurationS int    `json:"sleep_duration_s"`
	Source         string `json:"source"`
}

// IngestHealthMetricsBulk godoc
//
// @Summary Ingest health metrics
// @Description Idempotently ingests qring health metric points such as heart rate, SpO2, stress, HRV, and temperature.
// @Tags Health Metrics
// @Accept json
// @Produce json
// @Param device_id path string true "Device ID"
// @Param request body IngestHealthMetricsRequest true "Health metric points"
// @Success 200 {object} UpsertResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /v1/devices/{device_id}/health/metrics [post]
func (h *Handler) IngestHealthMetricsBulk(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "device_id")

	var req IngestHealthMetricsRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Err(w, 400, "invalid json")
		return
	}
	if len(req.Points) == 0 {
		httpx.Err(w, 400, "points is required")
		return
	}

	points := make([]store.HealthMetricRow, 0, len(req.Points))
	for _, p := range req.Points {
		ts, err := time.Parse(time.RFC3339, p.TsUTC)
		if err != nil {
			httpx.Err(w, 400, "invalid ts_utc")
			return
		}
		metric := strings.TrimSpace(p.Metric)
		if metric == "" {
			httpx.Err(w, 400, "metric is required")
			return
		}
		points = append(points, store.HealthMetricRow{
			DeviceID: deviceID,
			TsUTC:    ts,
			Metric:   metric,
			Value:    p.Value,
			Values:   p.Values,
			Unit:     strings.TrimSpace(p.Unit),
			Source:   strings.TrimSpace(p.Source),
			Metadata: p.Metadata,
		})
	}

	n, err := h.st.IngestHealthMetricsBulk(r.Context(), points)
	if err != nil {
		httpx.Err(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 200, map[string]any{"upserted": n})
}

// ListHealthMetrics godoc
//
// @Summary List health metrics
// @Description Lists health metric points in an RFC3339 time range, optionally filtered by metric name.
// @Tags Health Metrics
// @Produce json
// @Param device_id path string true "Device ID"
// @Param metric query string false "Metric name filter"
// @Param from query string true "Inclusive RFC3339 start timestamp" Format(date-time)
// @Param to query string true "Exclusive RFC3339 end timestamp" Format(date-time)
// @Success 200 {array} store.HealthMetricResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /v1/devices/{device_id}/health/metrics [get]
func (h *Handler) ListHealthMetrics(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "device_id")
	metric := strings.TrimSpace(r.URL.Query().Get("metric"))
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")

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
	if !fromT.Before(toT) {
		httpx.Err(w, 400, "from must be before to")
		return
	}

	rows, err := h.st.ListHealthMetrics(r.Context(), deviceID, metric, fromT, toT)
	if err != nil {
		httpx.Err(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 200, rows)
}

// UpsertDailyActivity godoc
//
// @Summary Upsert daily activity
// @Description Creates or updates one device-local daily activity summary for a device.
// @Tags Daily Activity
// @Accept json
// @Produce json
// @Param device_id path string true "Device ID"
// @Param request body DailyActivityRequest true "Daily activity payload"
// @Success 200 {object} store.DailyActivityResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /v1/devices/{device_id}/activity/daily [post]
func (h *Handler) UpsertDailyActivity(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "device_id")

	var req DailyActivityRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Err(w, 400, "invalid json")
		return
	}
	if _, err := time.Parse("2006-01-02", req.DeviceDate); err != nil {
		httpx.Err(w, 400, "invalid device_date")
		return
	}

	row, err := h.st.UpsertDailyActivity(r.Context(), store.DailyActivityRow{
		DeviceID:       deviceID,
		DeviceDate:     req.DeviceDate,
		TzOffsetMin:    req.TzOffsetMin,
		TotalSteps:     req.TotalSteps,
		RunningSteps:   req.RunningSteps,
		Calories:       req.Calories,
		WalkDistanceM:  req.WalkDistanceM,
		SportDurationS: req.SportDurationS,
		SleepDurationS: req.SleepDurationS,
		Source:         strings.TrimSpace(req.Source),
	})
	if err != nil {
		httpx.Err(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 200, row)
}

// ListDailyActivity godoc
//
// @Summary List daily activity
// @Description Lists device-local daily activity summaries in an inclusive date range.
// @Tags Daily Activity
// @Produce json
// @Param device_id path string true "Device ID"
// @Param from query string true "Inclusive device-local start date" Format(date)
// @Param to query string true "Inclusive device-local end date" Format(date)
// @Success 200 {array} store.DailyActivityResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /v1/devices/{device_id}/activity/daily [get]
func (h *Handler) ListDailyActivity(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "device_id")
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	if from == "" || to == "" {
		httpx.Err(w, 400, "from and to (YYYY-MM-DD) required")
		return
	}
	if _, err := time.Parse("2006-01-02", from); err != nil {
		httpx.Err(w, 400, "invalid from")
		return
	}
	if _, err := time.Parse("2006-01-02", to); err != nil {
		httpx.Err(w, 400, "invalid to")
		return
	}
	if from > to {
		httpx.Err(w, 400, "from must be before or equal to to")
		return
	}

	rows, err := h.st.ListDailyActivity(r.Context(), deviceID, from, to)
	if err != nil {
		httpx.Err(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 200, rows)
}
