package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"agem/internal/httpx"
	"agem/internal/store"

	"github.com/go-chi/chi/v5"
)

// SyncWearable godoc
//
// @Summary Sync wearable snapshot
// @Description Stores a QRing-style daily wearable payload, preserves the snapshot, and fans out recognized sections into MongoDB collections.
// @Tags Wearable v3
// @Accept json
// @Produce json
// @Param request body store.WearableSyncRequest true "Wearable sync payload"
// @Success 200 {object} store.WearableSyncResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /v3/wearable/sync [post]
func (h *Handler) SyncWearable(w http.ResponseWriter, r *http.Request) {
	var req store.WearableSyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Err(w, 400, "invalid json")
		return
	}

	resp, err := h.st.SyncWearable(r.Context(), req)
	if err != nil {
		if errors.Is(err, store.ErrInvalidWearablePayload) {
			httpx.Err(w, 400, trimWearableError(err))
			return
		}
		httpx.Err(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 200, resp)
}

// GetWearableRange godoc
//
// @Summary List synced wearable snapshots
// @Description Returns QRing-style daily wearable snapshots in an inclusive device-local date range.
// @Tags Wearable v3
// @Produce json
// @Param device_uid query string true "Stable wearable UID"
// @Param deviceUid query string false "Alias for device_uid"
// @Param from query string true "Inclusive start date" Format(date)
// @Param to query string true "Inclusive end date" Format(date)
// @Param include_readings query bool false "Include raw readings and sleep segments in snapshot payloads"
// @Success 200 {object} store.WearableRangeResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /v3/wearable/range [get]
func (h *Handler) GetWearableRange(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	deviceUID := strings.TrimSpace(query.Get("device_uid"))
	if deviceUID == "" {
		deviceUID = strings.TrimSpace(query.Get("deviceUid"))
	}

	if deviceUID == "" {
		httpx.Err(w, 400, "device_uid is required")
		return
	}

	from, to, includeReadings, ok := parseWearableRangeQuery(w, r)
	if !ok {
		return
	}

	items, err := h.st.ListWearableSnapshots(r.Context(), deviceUID, from, to, includeReadings)
	if err != nil {
		httpx.Err(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 200, store.WearableRangeResponse{
		DeviceUID: deviceUID,
		From:      from,
		To:        to,
		Items:     items,
	})
}

// GetUserWearableRange godoc
//
// @Summary List user wearable snapshots
// @Description Returns QRing-style daily wearable snapshots grouped by devices paired to the user.
// @Tags Wearable v3
// @Produce json
// @Param user_id path string true "User ID"
// @Param from query string true "Inclusive start date" Format(date)
// @Param to query string true "Inclusive end date" Format(date)
// @Param include_readings query bool false "Include raw readings and sleep segments in snapshot payloads"
// @Success 200 {object} store.UserWearableRangeResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /v3/users/{user_id}/wearable/range [get]
func (h *Handler) GetUserWearableRange(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(chi.URLParam(r, "user_id"))
	if userID == "" {
		httpx.Err(w, 400, "user_id is required")
		return
	}

	from, to, includeReadings, ok := parseWearableRangeQuery(w, r)
	if !ok {
		return
	}

	response, err := h.st.ListWearableSnapshotsByUser(r.Context(), userID, from, to, includeReadings)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httpx.Err(w, 404, "user not found")
			return
		}
		httpx.Err(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 200, response)
}

func parseWearableRangeQuery(w http.ResponseWriter, r *http.Request) (string, string, bool, bool) {
	query := r.URL.Query()
	from := strings.TrimSpace(query.Get("from"))
	to := strings.TrimSpace(query.Get("to"))

	if from == "" || to == "" {
		httpx.Err(w, 400, "from and to (YYYY-MM-DD) required")
		return "", "", false, false
	}
	fromDate, err := time.Parse("2006-01-02", from)
	if err != nil {
		httpx.Err(w, 400, "invalid from")
		return "", "", false, false
	}
	toDate, err := time.Parse("2006-01-02", to)
	if err != nil {
		httpx.Err(w, 400, "invalid to")
		return "", "", false, false
	}
	if fromDate.After(toDate) {
		httpx.Err(w, 400, "from must be before or equal to to")
		return "", "", false, false
	}
	includeReadings, err := parseOptionalBool(query.Get("include_readings"))
	if err != nil {
		httpx.Err(w, 400, "invalid include_readings")
		return "", "", false, false
	}

	return from, to, includeReadings, true
}

func trimWearableError(err error) string {
	msg := err.Error()
	const prefix = "invalid wearable payload: "
	return strings.TrimPrefix(msg, prefix)
}

func parseOptionalBool(value string) (bool, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return false, nil
	}
	return strconv.ParseBool(value)
}
