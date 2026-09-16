package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"agem/internal/httpx"
	"agem/internal/store"
)

type User struct {
	ID        string `json:"id"`
	Email     string `json:"email,omitempty"`
	Phone     string `json:"phone,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

type CreateUserRequest struct {
	Email string `json:"email"`
	Phone string `json:"phone"`
	// password handling omitted; you'd store password_hash etc.
}

type UpdateUserRequest struct {
	Email *string `json:"email"`
	Phone *string `json:"phone"`
}

// CreateUser godoc
//
// @Summary Create user
// @Description Creates an AGEM user identity record.
// @Tags Users
// @Accept json
// @Produce json
// @Param request body CreateUserRequest true "User payload"
// @Success 201 {object} store.UserRow
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /v1/users/ [post]
func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req CreateUserRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Err(w, 400, "invalid json")
		return
	}
	u, err := h.st.CreateUser(r.Context(), req.Email, req.Phone)
	if err != nil {
		httpx.Err(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 201, u)
}

// GetUser godoc
//
// @Summary Get user
// @Description Returns one AGEM user by ID.
// @Tags Users
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} store.UserRow
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /v1/users/{id} [get]
func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	u, err := h.st.GetUser(r.Context(), id)
	if err != nil {
		if err == store.ErrNotFound {
			httpx.Err(w, 404, "user not found")
			return
		}
		httpx.Err(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 200, u)
}

// UpdateUser godoc
//
// @Summary Update user
// @Description Updates user email or phone.
// @Tags Users
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param request body UpdateUserRequest true "User update payload"
// @Success 200 {object} store.UserRow
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /v1/users/{id} [patch]
func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req UpdateUserRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Err(w, 400, "invalid json")
		return
	}
	u, err := h.st.UpdateUser(r.Context(), id, req.Email, req.Phone)
	if err != nil {
		if err == store.ErrNotFound {
			httpx.Err(w, 404, "user not found")
			return
		}
		httpx.Err(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 200, u)
}

// DeleteUser godoc
//
// @Summary Delete user
// @Description Deletes a user by ID.
// @Tags Users
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} StatusResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /v1/users/{id} [delete]
func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.st.DeleteUser(r.Context(), id); err != nil {
		if err == store.ErrNotFound {
			httpx.Err(w, 404, "user not found")
			return
		}
		httpx.Err(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 200, map[string]string{"status": "deleted"})
}
