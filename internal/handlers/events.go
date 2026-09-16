package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"agem/internal/httpx"
	"agem/internal/store"
)

type DeviceEventPoint struct {
	TsUTC     string         `json:"ts_utc"`
	EventType string         `json:"event_type"`
	Source    string         `json:"source"`
	Payload   map[string]any `json:"payload"`
}

type IngestDeviceEventsRequest struct {
	Events []DeviceEventPoint `json:"events"`
}

// IngestDeviceEventsBulk godoc
//
// @Summary Ingest device events
// @Description Idempotently ingests device events such as sync markers, battery state, binding events, or SDK notifications.
// @Tags Device Events
// @Accept json
// @Produce json
// @Param device_id path string true "Device ID"
// @Param request body IngestDeviceEventsRequest true "Device event payload"
// @Success 200 {object} UpsertResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /v1/devices/{device_id}/events [post]
func (h *Handler) IngestDeviceEventsBulk(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "device_id")

	var req IngestDeviceEventsRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Err(w, 400, "invalid json")
		return
	}
	if len(req.Events) == 0 {
		httpx.Err(w, 400, "events is required")
		return
	}

	events := make([]store.DeviceEventRow, 0, len(req.Events))
	for _, p := range req.Events {
		ts, err := time.Parse(time.RFC3339, p.TsUTC)
		if err != nil {
			httpx.Err(w, 400, "invalid ts_utc")
			return
		}
		eventType := strings.TrimSpace(p.EventType)
		if eventType == "" {
			httpx.Err(w, 400, "event_type is required")
			return
		}
		events = append(events, store.DeviceEventRow{
			DeviceID:  deviceID,
			TsUTC:     ts,
			EventType: eventType,
			Source:    strings.TrimSpace(p.Source),
			Payload:   p.Payload,
		})
	}

	n, err := h.st.IngestDeviceEventsBulk(r.Context(), events)
	if err != nil {
		httpx.Err(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 200, map[string]any{"upserted": n})
}

// ListDeviceEvents godoc
//
// @Summary List device events
// @Description Lists device events in an RFC3339 time range, optionally filtered by event type.
// @Tags Device Events
// @Produce json
// @Param device_id path string true "Device ID"
// @Param event_type query string false "Event type filter"
// @Param from query string true "Inclusive RFC3339 start timestamp" Format(date-time)
// @Param to query string true "Exclusive RFC3339 end timestamp" Format(date-time)
// @Success 200 {array} store.DeviceEventResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /v1/devices/{device_id}/events [get]
func (h *Handler) ListDeviceEvents(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "device_id")
	eventType := strings.TrimSpace(r.URL.Query().Get("event_type"))
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

	rows, err := h.st.ListDeviceEvents(r.Context(), deviceID, eventType, fromT, toT)
	if err != nil {
		httpx.Err(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 200, rows)
}
