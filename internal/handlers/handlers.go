package handlers

import "agem/internal/store"

type Handler struct {
	st *store.Store
}

type StatusResponse struct {
	Status string `json:"status" example:"deleted"`
}

type UpsertResponse struct {
	Upserted int `json:"upserted" example:"1"`
}

func New(st *store.Store) *Handler {
	return &Handler{st: st}
}
