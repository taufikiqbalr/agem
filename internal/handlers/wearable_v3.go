package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"agem/internal/httpx"
	"agem/internal/store"

	"github.com/go-chi/chi/v5"
)

const maxWearableV3BodyBytes int64 = 8 << 20 // 8 MiB; raw samples should be sent in bounded batches.

type PairWearableDeviceRequest struct {
	DeviceUID string `json:"deviceUid"`
	Nickname  string `json:"nickname,omitempty"`
	IsPrimary bool   `json:"isPrimary"`
}

type UpdateWearablePairingRequest struct {
	Nickname  *string `json:"nickname,omitempty"`
	IsPrimary *bool   `json:"isPrimary,omitempty"`
}

// SyncWearable godoc
//
// @Summary Synchronize QRing SDK wearable data
// @Description Canonical v3 endpoint for QRing device metadata/state, multi-day activity, measurements, sleep sessions, workouts, events, targets, and optional raw samples.
// @Tags Wearable v3
// @Accept json
// @Produce json
// @Param request body store.WearableSyncRequest true "QRing SDK v3 sync payload"
// @Success 200 {object} store.WearableSyncResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 409 {object} httpx.ErrorResponse
// @Failure 413 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /v3/wearable/sync [post]
func (h *Handler) SyncWearable(w http.ResponseWriter, r *http.Request) {
	var req store.WearableSyncRequest
	if err := decodeWearableV3JSON(w, r, &req); err != nil {
		writeWearableV3DecodeError(w, err)
		return
	}
	resp, err := h.st.SyncWearable(r.Context(), req)
	if err != nil {
		writeWearableV3StoreError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// GetWearableRange godoc
//
// @Summary Read normalized wearable data by device UID
// @Description Returns normalized v3 wearable data grouped by explicit device-local date. Raw sensor samples are excluded unless include_raw=true.
// @Tags Wearable v3
// @Produce json
// @Param device_uid query string true "Stable device UID"
// @Param from query string true "Inclusive device-local start date" Format(date)
// @Param to query string true "Inclusive device-local end date" Format(date)
// @Param include_raw query bool false "Include raw_sensor_samples"
// @Success 200 {object} store.WearableRangeResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /v3/wearable/range [get]
func (h *Handler) GetWearableRange(w http.ResponseWriter, r *http.Request) {
	deviceUID := strings.TrimSpace(r.URL.Query().Get("device_uid"))
	if deviceUID == "" {
		deviceUID = strings.TrimSpace(r.URL.Query().Get("deviceUid"))
	}
	if deviceUID == "" {
		httpx.Err(w, http.StatusBadRequest, "device_uid is required")
		return
	}
	from, to, includeRaw, ok := parseWearableV3RangeQuery(w, r)
	if !ok {
		return
	}
	resp, err := h.st.GetWearableRangeByUIDV3(r.Context(), deviceUID, from, to, includeRaw)
	if err != nil {
		writeWearableV3StoreError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// GetWearableRangeByDeviceID godoc
//
// @Summary Read normalized wearable data by device ID
// @Tags Wearable v3
// @Produce json
// @Param device_id path string true "AGEM MongoDB device ID"
// @Param from query string true "Inclusive device-local start date" Format(date)
// @Param to query string true "Inclusive device-local end date" Format(date)
// @Param include_raw query bool false "Include raw_sensor_samples"
// @Success 200 {object} store.WearableRangeResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /v3/devices/{device_id}/wearable/range [get]
func (h *Handler) GetWearableRangeByDeviceID(w http.ResponseWriter, r *http.Request) {
	deviceID := strings.TrimSpace(chi.URLParam(r, "device_id"))
	if deviceID == "" {
		httpx.Err(w, http.StatusBadRequest, "device_id is required")
		return
	}
	from, to, includeRaw, ok := parseWearableV3RangeQuery(w, r)
	if !ok {
		return
	}
	resp, err := h.st.GetWearableRangeByDeviceIDV3(r.Context(), deviceID, from, to, includeRaw)
	if err != nil {
		writeWearableV3StoreError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// GetUserWearableRange godoc
//
// @Summary Read normalized wearable data for all active devices paired to a user
// @Tags Wearable v3
// @Produce json
// @Param user_id path string true "Canonical Regene/AGEM user ID"
// @Param from query string true "Inclusive device-local start date" Format(date)
// @Param to query string true "Inclusive device-local end date" Format(date)
// @Param include_raw query bool false "Include raw_sensor_samples"
// @Success 200 {object} store.UserWearableRangeResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /v3/users/{user_id}/wearable/range [get]
func (h *Handler) GetUserWearableRange(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(chi.URLParam(r, "user_id"))
	if userID == "" {
		httpx.Err(w, http.StatusBadRequest, "user_id is required")
		return
	}
	from, to, includeRaw, ok := parseWearableV3RangeQuery(w, r)
	if !ok {
		return
	}
	resp, err := h.st.GetUserWearableRangeV3(r.Context(), userID, from, to, includeRaw)
	if err != nil {
		writeWearableV3StoreError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// GetWearableDeviceByUIDV3 godoc
//
// @Summary Get a registered QRing device by stable UID
// @Tags Wearable v3 Devices
// @Produce json
// @Param device_uid path string true "Stable device UID"
// @Success 200 {object} store.WearableDeviceView
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /v3/devices/by_uid/{device_uid} [get]
func (h *Handler) GetWearableDeviceByUIDV3(w http.ResponseWriter, r *http.Request) {
	deviceUID := strings.TrimSpace(chi.URLParam(r, "device_uid"))
	if deviceUID == "" {
		httpx.Err(w, http.StatusBadRequest, "device_uid is required")
		return
	}
	resp, err := h.st.GetWearableDeviceByUIDV3(r.Context(), deviceUID)
	if err != nil {
		writeWearableV3StoreError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// PairWearableDeviceV3 godoc
//
// @Summary Pair a registered wearable to a user by device UID
// @Description Keeps pairing history and enforces one active owner per device and one active primary device per user.
// @Tags Wearable v3 Pairing
// @Accept json
// @Produce json
// @Param user_id path string true "Canonical Regene/AGEM user ID"
// @Param request body PairWearableDeviceRequest true "Pairing request"
// @Success 201 {object} store.WearablePairingView
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 409 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /v3/users/{user_id}/devices [post]
func (h *Handler) PairWearableDeviceV3(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(chi.URLParam(r, "user_id"))
	var req PairWearableDeviceRequest
	if err := decodeWearableV3JSON(w, r, &req); err != nil {
		writeWearableV3DecodeError(w, err)
		return
	}
	if strings.TrimSpace(req.DeviceUID) == "" {
		httpx.Err(w, http.StatusBadRequest, "deviceUid is required")
		return
	}
	resp, err := h.st.PairWearableDeviceV3(r.Context(), store.PairWearableDeviceParams{
		UserID: userID, DeviceUID: req.DeviceUID, Nickname: req.Nickname, IsPrimary: req.IsPrimary,
	})
	if err != nil {
		writeWearableV3StoreError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, resp)
}

// ListWearableDevicesForUserV3 godoc
//
// @Summary List active wearable pairings for a user
// @Tags Wearable v3 Pairing
// @Produce json
// @Param user_id path string true "Canonical Regene/AGEM user ID"
// @Success 200 {array} store.WearablePairingView
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /v3/users/{user_id}/devices [get]
func (h *Handler) ListWearableDevicesForUserV3(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(chi.URLParam(r, "user_id"))
	resp, err := h.st.ListWearableDevicesForUserV3(r.Context(), userID)
	if err != nil {
		writeWearableV3StoreError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// UpdateWearablePairingV3 godoc
//
// @Summary Update nickname or primary flag of an active pairing
// @Tags Wearable v3 Pairing
// @Accept json
// @Produce json
// @Param user_id path string true "Canonical Regene/AGEM user ID"
// @Param device_uid path string true "Stable device UID"
// @Param request body UpdateWearablePairingRequest true "Pairing update"
// @Success 200 {object} store.WearablePairingView
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 409 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /v3/users/{user_id}/devices/{device_uid} [patch]
func (h *Handler) UpdateWearablePairingV3(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(chi.URLParam(r, "user_id"))
	deviceUID := strings.TrimSpace(chi.URLParam(r, "device_uid"))
	var req UpdateWearablePairingRequest
	if err := decodeWearableV3JSON(w, r, &req); err != nil {
		writeWearableV3DecodeError(w, err)
		return
	}
	resp, err := h.st.UpdateWearablePairingV3(r.Context(), userID, deviceUID, store.UpdateWearablePairingParams{
		Nickname: req.Nickname, IsPrimary: req.IsPrimary,
	})
	if err != nil {
		writeWearableV3StoreError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// UnpairWearableDeviceV3 godoc
//
// @Summary Soft-unpair a wearable while preserving pairing history
// @Tags Wearable v3 Pairing
// @Produce json
// @Param user_id path string true "Canonical Regene/AGEM user ID"
// @Param device_uid path string true "Stable device UID"
// @Success 200 {object} map[string]string
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /v3/users/{user_id}/devices/{device_uid} [delete]
func (h *Handler) UnpairWearableDeviceV3(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(chi.URLParam(r, "user_id"))
	deviceUID := strings.TrimSpace(chi.URLParam(r, "device_uid"))
	if err := h.st.UnpairWearableDeviceV3(r.Context(), userID, deviceUID); err != nil {
		writeWearableV3StoreError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "unpaired"})
}

func parseWearableV3RangeQuery(w http.ResponseWriter, r *http.Request) (string, string, bool, bool) {
	from := strings.TrimSpace(r.URL.Query().Get("from"))
	to := strings.TrimSpace(r.URL.Query().Get("to"))
	if from == "" || to == "" {
		httpx.Err(w, http.StatusBadRequest, "from and to (YYYY-MM-DD) are required")
		return "", "", false, false
	}
	if _, err := time.Parse("2006-01-02", from); err != nil {
		httpx.Err(w, http.StatusBadRequest, "invalid from")
		return "", "", false, false
	}
	if _, err := time.Parse("2006-01-02", to); err != nil {
		httpx.Err(w, http.StatusBadRequest, "invalid to")
		return "", "", false, false
	}
	if from > to {
		httpx.Err(w, http.StatusBadRequest, "from must be before or equal to to")
		return "", "", false, false
	}
	includeRaw := false
	if value := strings.TrimSpace(r.URL.Query().Get("include_raw")); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			httpx.Err(w, http.StatusBadRequest, "invalid include_raw")
			return "", "", false, false
		}
		includeRaw = parsed
	}
	return from, to, includeRaw, true
}

func decodeWearableV3JSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxWearableV3BodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return errors.New("multiple JSON values are not allowed")
		}
		return err
	}
	return nil
}

func writeWearableV3DecodeError(w http.ResponseWriter, err error) {
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		httpx.Err(w, http.StatusRequestEntityTooLarge, "request body exceeds 8 MiB")
		return
	}
	httpx.Err(w, http.StatusBadRequest, "invalid json: "+err.Error())
}

func writeWearableV3StoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrInvalidWearablePayload):
		msg := strings.TrimPrefix(err.Error(), store.ErrInvalidWearablePayload.Error()+": ")
		httpx.Err(w, http.StatusBadRequest, msg)
	case errors.Is(err, store.ErrNotFound), errors.Is(err, store.ErrPairingNotFound):
		httpx.Err(w, http.StatusNotFound, err.Error())
	case errors.Is(err, store.ErrDeviceAlreadyPaired):
		httpx.Err(w, http.StatusConflict, err.Error())
	default:
		// Keep internal MongoDB/index details out of the public API response.
		httpx.Err(w, http.StatusInternalServerError, "internal server error")
	}
}
