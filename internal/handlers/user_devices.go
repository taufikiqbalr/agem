package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"agem/internal/httpx"
	"agem/internal/store"
)

type PairDeviceRequest struct {
	DeviceID  string `json:"device_id"`
	Nickname  string `json:"nickname"`
	IsPrimary bool   `json:"is_primary"`
}

// PairDeviceToUser godoc
//
// @Summary Pair device to user
// @Description Creates a pairing record between a user and a device.
// @Tags Pairings
// @Accept json
// @Produce json
// @Param user_id path string true "User ID"
// @Param request body PairDeviceRequest true "Pairing payload"
// @Success 201 {object} store.UserDeviceRow
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /v1/users/{user_id}/devices [post]
func (h *Handler) PairDeviceToUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "user_id")
	var req PairDeviceRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Err(w, 400, "invalid json")
		return
	}
	if req.DeviceID == "" {
		httpx.Err(w, 400, "device_id is required")
		return
	}
	ud, err := h.st.PairDeviceToUser(r.Context(), store.PairDeviceParams{
		UserID: userID, DeviceID: req.DeviceID, Nickname: req.Nickname, IsPrimary: req.IsPrimary,
	})
	if err != nil {
		if err == store.ErrNotFound {
			httpx.Err(w, 404, "user not found")
			return
		}
		httpx.Err(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 201, ud)
}

// ListUserDevices godoc
//
// @Summary List user devices
// @Description Lists device pairings for one user.
// @Tags Pairings
// @Produce json
// @Param user_id path string true "User ID"
// @Success 200 {array} store.UserDeviceRow
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /v1/users/{user_id}/devices [get]
func (h *Handler) ListUserDevices(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "user_id")
	list, err := h.st.ListUserDevices(r.Context(), userID)
	if err != nil {
		if err == store.ErrNotFound {
			httpx.Err(w, 404, "user not found")
			return
		}
		httpx.Err(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 200, list)
}

type UpdateUserDeviceRequest struct {
	Nickname   *string `json:"nickname"`
	IsPrimary  *bool   `json:"is_primary"`
	UnpairedAt *string `json:"unpaired_at"` // RFC3339 or null
}

// UpdateUserDevice godoc
//
// @Summary Update pairing
// @Description Updates pairing nickname, primary flag, or unpaired time.
// @Tags Pairings
// @Accept json
// @Produce json
// @Param id path string true "Pairing ID"
// @Param request body UpdateUserDeviceRequest true "Pairing update payload"
// @Success 200 {object} store.UserDeviceRow
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /v1/user_devices/{id} [patch]
func (h *Handler) UpdateUserDevice(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req UpdateUserDeviceRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Err(w, 400, "invalid json")
		return
	}
	ud, err := h.st.UpdateUserDevice(r.Context(), id, store.UpdateUserDeviceParams{
		Nickname: req.Nickname, IsPrimary: req.IsPrimary, UnpairedAt: req.UnpairedAt,
	})
	if err != nil {
		if err == store.ErrNotFound {
			httpx.Err(w, 404, "pairing not found")
			return
		}
		httpx.Err(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 200, ud)
}

// DeleteUserDevice godoc
//
// @Summary Delete pairing
// @Description Deletes a user-device pairing record.
// @Tags Pairings
// @Produce json
// @Param id path string true "Pairing ID"
// @Success 200 {object} StatusResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /v1/user_devices/{id} [delete]
func (h *Handler) DeleteUserDevice(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.st.DeleteUserDevice(r.Context(), id); err != nil {
		if err == store.ErrNotFound {
			httpx.Err(w, 404, "pairing not found")
			return
		}
		httpx.Err(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 200, map[string]string{"status": "deleted"})
}
