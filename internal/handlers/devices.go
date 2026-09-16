package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"agem/internal/httpx"
	"agem/internal/store"
)

type CreateDeviceRequest struct {
	Vendor    string `json:"vendor"`
	Model     string `json:"model"`
	HwRev     string `json:"hw_rev"`
	FwRev     string `json:"fw_rev"`
	Serial    string `json:"serial_number"`
	DeviceUID string `json:"device_uid"` // remoteId/MAC stable identifier
}

type UpdateDeviceRequest struct {
	Vendor     *string `json:"vendor"`
	Model      *string `json:"model"`
	HwRev      *string `json:"hw_rev"`
	FwRev      *string `json:"fw_rev"`
	Serial     *string `json:"serial_number"`
	LastSeenAt *string `json:"last_seen_at"` // RFC3339
}

// CreateDevice godoc
//
// @Summary Create device
// @Description Creates a wearable device registry record.
// @Tags Devices
// @Accept json
// @Produce json
// @Param request body CreateDeviceRequest true "Device payload"
// @Success 201 {object} store.DeviceRow
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /v1/devices/ [post]
func (h *Handler) CreateDevice(w http.ResponseWriter, r *http.Request) {
	var req CreateDeviceRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Err(w, 400, "invalid json")
		return
	}
	if req.DeviceUID == "" || req.Vendor == "" {
		httpx.Err(w, 400, "vendor and device_uid are required")
		return
	}
	d, err := h.st.CreateDevice(r.Context(), store.CreateDeviceParams{
		Vendor: req.Vendor, Model: req.Model, HwRev: req.HwRev, FwRev: req.FwRev, SerialNumber: req.Serial, DeviceUID: req.DeviceUID,
	})
	if err != nil {
		httpx.Err(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 201, d)
}

// GetDevice godoc
//
// @Summary Get device
// @Description Returns one device by API ID.
// @Tags Devices
// @Produce json
// @Param id path string true "Device ID"
// @Success 200 {object} store.DeviceRow
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /v1/devices/{id} [get]
func (h *Handler) GetDevice(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	d, err := h.st.GetDevice(r.Context(), id)
	if err != nil {
		if err == store.ErrNotFound {
			httpx.Err(w, 404, "device not found")
			return
		}
		httpx.Err(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 200, d)
}

// GetDeviceByUID godoc
//
// @Summary Get device by UID
// @Description Returns one device by stable wearable UID or MAC address.
// @Tags Devices
// @Produce json
// @Param device_uid path string true "Stable wearable UID"
// @Success 200 {object} store.DeviceRow
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /v1/devices/by_uid/{device_uid} [get]
func (h *Handler) GetDeviceByUID(w http.ResponseWriter, r *http.Request) {
	uid := chi.URLParam(r, "device_uid")
	d, err := h.st.GetDeviceByUID(r.Context(), uid)
	if err != nil {
		if err == store.ErrNotFound {
			httpx.Err(w, 404, "device not found")
			return
		}
		httpx.Err(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 200, d)
}

// UpdateDevice godoc
//
// @Summary Update device
// @Description Updates device metadata and last seen timestamp.
// @Tags Devices
// @Accept json
// @Produce json
// @Param id path string true "Device ID"
// @Param request body UpdateDeviceRequest true "Device update payload"
// @Success 200 {object} store.DeviceRow
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /v1/devices/{id} [patch]
func (h *Handler) UpdateDevice(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req UpdateDeviceRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Err(w, 400, "invalid json")
		return
	}
	d, err := h.st.UpdateDevice(r.Context(), id, store.UpdateDeviceParams{
		Vendor: req.Vendor, Model: req.Model, HwRev: req.HwRev, FwRev: req.FwRev, SerialNumber: req.Serial, LastSeenAt: req.LastSeenAt,
	})
	if err != nil {
		if err == store.ErrNotFound {
			httpx.Err(w, 404, "device not found")
			return
		}
		httpx.Err(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 200, d)
}

// DeleteDevice godoc
//
// @Summary Delete device
// @Description Deletes a device by API ID.
// @Tags Devices
// @Produce json
// @Param id path string true "Device ID"
// @Success 200 {object} StatusResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /v1/devices/{id} [delete]
func (h *Handler) DeleteDevice(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.st.DeleteDevice(r.Context(), id); err != nil {
		if err == store.ErrNotFound {
			httpx.Err(w, 404, "device not found")
			return
		}
		httpx.Err(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 200, map[string]string{"status": "deleted"})
}
