// Package handler is the HTTP layer: parse the request, call the service,
// write the response. No business rules live here.
package handler

import (
	"net/http"

	"github.com/srtipu5/mini-wallet/internal/application"
	"github.com/srtipu5/mini-wallet/internal/domain"
	"github.com/srtipu5/mini-wallet/internal/middleware"
)

type Handler struct {
	svc *application.Service
}

// NewRouter wires all routes (Go 1.22+ ServeMux supports methods and {params}).
func NewRouter(svc *application.Service) http.Handler {
	h := &Handler{svc: svc}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", h.health)
	mux.HandleFunc("POST /api/v1/wallets", h.createWallet)
	mux.HandleFunc("GET /api/v1/wallets/{wallet_id}", h.getWallet)
	mux.HandleFunc("POST /api/v1/wallets/{wallet_id}/add-money", h.addMoney)
	mux.HandleFunc("GET /api/v1/wallets/{wallet_id}/transactions", h.listTransactions)
	mux.HandleFunc("POST /api/v1/transfers", h.transfer)

	// Anything else gets the standard error envelope instead of a plain-text 404.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, r, domain.NewError(domain.CodeNotFound, "route not found"))
	})

	return middleware.Recover(middleware.Logger(mux))
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Ping(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "database unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// writeIdempotent writes a success response and marks replays with a header,
// so clients (and tests) can tell a replay from a fresh execution.
func writeIdempotent(w http.ResponseWriter, replayed bool, v any) {
	if replayed {
		w.Header().Set("Idempotent-Replayed", "true")
	}
	writeJSON(w, http.StatusCreated, v)
}
