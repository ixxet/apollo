package server

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type healthResponse struct {
	Service         string `json:"service"`
	Status          string `json:"status"`
	ConsumerEnabled bool   `json:"consumer_enabled"`
}

func NewHandler(consumerEnabled bool) http.Handler {
	router := chi.NewRouter()
	router.Get("/api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, healthResponse{
			Service:         "apollo",
			Status:          "ok",
			ConsumerEnabled: consumerEnabled,
		})
	})

	return router
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
