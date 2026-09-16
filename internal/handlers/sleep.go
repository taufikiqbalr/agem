package handlers

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"agem/internal/httpx"
	"agem/internal/store"
)

type SleepSummaryRequest struct {
	SleepDate     string `json:"sleep_date"` // YYYY-MM-DD (device-local night)
	TzOffsetMin   int    `json:"tz_offset_min"`
	SleepStartUTC string `json:"sleep_start_utc"` // RFC3339 optional
	WakeUTC       string `json:"wake_utc"`        // RFC3339 optional
	TotalSleepS   int    `json:"total_sleep_s"`
	DeepS         int    `json:"deep_s"`
	LightS        int    `json:"light_s"`
	AwakeS        int    `json:"awake_s"`
	RemS          int    `json:"rem_s"`
	Source        string `json:"source"`
}

// UpsertSleepSummary godoc
//
// @Summary Upsert sleep summary
// @Description Creates or updates one device-local sleep summary for a device and sleep date.
// @Tags Sleep
// @Accept json
// @Produce json
// @Param device_id path string true "Device ID"
// @Param request body SleepSummaryRequest true "Sleep summary payload"
// @Success 200 {object} store.SleepSummaryResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /v1/devices/{device_id}/sleep/summary [post]
func (h *Handler) UpsertSleepSummary(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "device_id")

	var req SleepSummaryRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Err(w, 400, "invalid json")
		return
	}
	if req.SleepDate == "" {
		httpx.Err(w, 400, "sleep_date is required")
		return
	}

	var startPtr *time.Time
	var endPtr *time.Time
	if req.SleepStartUTC != "" {
		t, err := time.Parse(time.RFC3339, req.SleepStartUTC)
		if err != nil {
			httpx.Err(w, 400, "invalid sleep_start_utc")
			return
		}
		startPtr = &t
	}
	if req.WakeUTC != "" {
		t, err := time.Parse(time.RFC3339, req.WakeUTC)
		if err != nil {
			httpx.Err(w, 400, "invalid wake_utc")
			return
		}
		endPtr = &t
	}

	row, err := h.st.UpsertSleepSummary(r.Context(), store.SleepSummaryRow{
		DeviceID:      deviceID,
		SleepDate:     req.SleepDate,
		TzOffsetMin:   req.TzOffsetMin,
		SleepStartUTC: startPtr,
		WakeUTC:       endPtr,
		TotalSleepS:   req.TotalSleepS,
		DeepS:         req.DeepS,
		LightS:        req.LightS,
		AwakeS:        req.AwakeS,
		RemS:          req.RemS,
		Source:        req.Source,
	})
	if err != nil {
		httpx.Err(w, 500, err.Error())
		return
	}

	httpx.JSON(w, 200, row)
}

// GetSleepSummaryByDate godoc
//
// @Summary Get sleep summary
// @Description Returns a sleep summary by device-local sleep date.
// @Tags Sleep
// @Produce json
// @Param device_id path string true "Device ID"
// @Param date query string true "Device-local sleep date" Format(date)
// @Success 200 {object} store.SleepSummaryResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /v1/devices/{device_id}/sleep/summary [get]
func (h *Handler) GetSleepSummaryByDate(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "device_id")
	date := r.URL.Query().Get("date")
	if date == "" {
		httpx.Err(w, 400, "date=YYYY-MM-DD required")
		return
	}

	row, err := h.st.GetSleepSummaryByDate(r.Context(), deviceID, date)
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

type SleepSegment struct {
	StartUTC    string `json:"start_utc"`  // RFC3339
	EndUTC      string `json:"end_utc"`    // RFC3339
	SleepDate   string `json:"sleep_date"` // YYYY-MM-DD
	TzOffsetMin int    `json:"tz_offset_min"`
	Stage       string `json:"stage"` // deep/light/rem/awake/off_wrist/unknown
	Source      string `json:"source"`
}

type IngestSleepSegmentsRequest struct {
	Segments []SleepSegment `json:"segments"`
}

// IngestSleepSegmentsBulk godoc
//
// @Summary Ingest sleep segments
// @Description Idempotently ingests sleep stage segments for a device.
// @Tags Sleep
// @Accept json
// @Produce json
// @Param device_id path string true "Device ID"
// @Param request body IngestSleepSegmentsRequest true "Sleep segment payload"
// @Success 200 {object} UpsertResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /v1/devices/{device_id}/sleep/segments [post]
func (h *Handler) IngestSleepSegmentsBulk(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "device_id")

	var req IngestSleepSegmentsRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Err(w, 400, "invalid json")
		return
	}
	if len(req.Segments) == 0 {
		httpx.Err(w, 400, "segments required")
		return
	}

	var rows []store.SleepSegmentRow
	for _, s := range req.Segments {
		st, err := time.Parse(time.RFC3339, s.StartUTC)
		if err != nil {
			httpx.Err(w, 400, "invalid start_utc")
			return
		}
		et, err := time.Parse(time.RFC3339, s.EndUTC)
		if err != nil {
			httpx.Err(w, 400, "invalid end_utc")
			return
		}
		rows = append(rows, store.SleepSegmentRow{
			DeviceID:    deviceID,
			StartUTC:    st,
			EndUTC:      et,
			SleepDate:   s.SleepDate,
			TzOffsetMin: s.TzOffsetMin,
			Stage:       s.Stage,
			Source:      s.Source,
		})
	}

	n, err := h.st.UpsertSleepSegmentsBulk(r.Context(), rows)
	if err != nil {
		httpx.Err(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 200, map[string]any{"upserted": n})
}

// ListSleepSegmentsByDate godoc
//
// @Summary List sleep segments
// @Description Lists sleep stage segments by device-local sleep date.
// @Tags Sleep
// @Produce json
// @Param device_id path string true "Device ID"
// @Param date query string true "Device-local sleep date" Format(date)
// @Success 200 {array} store.SleepSegmentResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /v1/devices/{device_id}/sleep/segments [get]
func (h *Handler) ListSleepSegmentsByDate(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "device_id")
	date := r.URL.Query().Get("date")
	if date == "" {
		httpx.Err(w, 400, "date=YYYY-MM-DD required")
		return
	}

	rows, err := h.st.ListSleepSegmentsByDate(r.Context(), deviceID, date)
	if err != nil {
		httpx.Err(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 200, rows)
}
