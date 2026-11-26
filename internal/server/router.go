package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type healthResponse struct {
	Status string `json:"status"`
}

func NewRouter(pool *pgxpool.Pool) http.Handler {
	r := chi.NewRouter()

	r.Get("/health/live", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, healthResponse{Status: "ok"})
	})

	r.Get("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, healthResponse{Status: "not_ready"})
			return
		}
		writeJSON(w, http.StatusOK, healthResponse{Status: "ready"})
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/orders", createOrderHandler(pool))
		r.Get("/orders/{orderNumber}", getOrderHandler(pool))
	})

	return r
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
