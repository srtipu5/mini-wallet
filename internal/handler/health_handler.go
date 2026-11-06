package handler

import (
	"net/http"

	"github.com/srtipu5/mini-wallet/internal/domain"
	"github.com/srtipu5/mini-wallet/internal/utils"
)

// GET /health
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Ping(r.Context()); err != nil {
		utils.SendData(w, http.StatusServiceUnavailable, map[string]string{"status": "database unavailable"})
		return
	}
	utils.SendData(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Fallback for unknown routes.
func (h *Handler) NotFound(w http.ResponseWriter, r *http.Request) {
	utils.SendError(w, r, domain.NewError(domain.CodeNotFound, "Route not found"))
}
