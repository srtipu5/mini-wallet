package handler

import (
	"encoding/json"
	"net/http"

	"github.com/srtipu5/mini-wallet/internal/application"
)

type createWalletRequest struct {
	UserID string `json:"user_id"`
	Phone  string `json:"phone"`
}

// POST /api/v1/wallets
func (h *Handler) createWallet(w http.ResponseWriter, r *http.Request) {
	var req createWalletRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, err)
		return
	}

	wallet, err := h.svc.CreateWallet(r.Context(), req.UserID, req.Phone)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, wallet)
}

// GET /api/v1/wallets/{wallet_id}
func (h *Handler) getWallet(w http.ResponseWriter, r *http.Request) {
	wallet, err := h.svc.GetWallet(r.Context(), r.PathValue("wallet_id"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, wallet)
}

type addMoneyRequest struct {
	Amount     json.RawMessage `json:"amount"` // parsed by parseAmount (integers only)
	GatewayRef string          `json:"gateway_ref"`
}

// POST /api/v1/wallets/{wallet_id}/add-money
func (h *Handler) addMoney(w http.ResponseWriter, r *http.Request) {
	key, err := idempotencyKey(r)
	if err != nil {
		writeError(w, r, err)
		return
	}

	var req addMoneyRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, err)
		return
	}
	amount, err := parseAmount(req.Amount)
	if err != nil {
		writeError(w, r, err)
		return
	}

	result, replayed, err := h.svc.AddMoney(r.Context(), key, application.AddMoneyInput{
		WalletID:   r.PathValue("wallet_id"),
		Amount:     amount,
		GatewayRef: req.GatewayRef,
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeIdempotent(w, replayed, result)
}
