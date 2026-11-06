package handler

import (
	"encoding/json"
	"net/http"

	"github.com/srtipu5/mini-wallet/internal/application"
)

type transferRequest struct {
	FromWalletID string          `json:"from_wallet_id"`
	ToWalletID   string          `json:"to_wallet_id"`
	Amount       json.RawMessage `json:"amount"` // parsed by parseAmount (integers only)
	Note         *string         `json:"note"`
}

// POST /api/v1/transfers
func (h *Handler) transfer(w http.ResponseWriter, r *http.Request) {
	key, err := idempotencyKey(r)
	if err != nil {
		writeError(w, r, err)
		return
	}

	var req transferRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, err)
		return
	}
	amount, err := parseAmount(req.Amount)
	if err != nil {
		writeError(w, r, err)
		return
	}

	transfer, replayed, err := h.svc.Transfer(r.Context(), key, application.TransferInput{
		FromWalletID: req.FromWalletID,
		ToWalletID:   req.ToWalletID,
		Amount:       amount,
		Note:         req.Note,
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeIdempotent(w, replayed, transfer)
}
