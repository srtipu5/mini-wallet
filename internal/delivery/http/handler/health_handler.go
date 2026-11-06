package handler

import (
	"net/http"

	"github.com/srtipu5/mini-wallet/internal/domain"
	"github.com/srtipu5/mini-wallet/internal/utils"
)

// GET /health
func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Ping(r.Context()); err != nil {
		utils.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "database unavailable"})
		return
	}
	utils.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Fallback for unknown routes.
func (h *Handler) notFound(w http.ResponseWriter, r *http.Request) {
	utils.WriteError(w, r, domain.NewError(domain.CodeNotFound, "route not found"))
}
