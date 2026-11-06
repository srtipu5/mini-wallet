package handler

import (
	"net/http"

	"github.com/srtipu5/mini-wallet/internal/dto"
	"github.com/srtipu5/mini-wallet/internal/utils"
)

// POST /api/v1/transfers
func (h *Handler) Transfer(w http.ResponseWriter, r *http.Request) {
	key, err := utils.IdempotencyKey(r)
	if err != nil {
		utils.SendError(w, r, err)
		return
	}

	var req dto.TransferRequest
	if err := utils.DecodeJSON(w, r, &req); err != nil {
		utils.SendError(w, r, err)
		return
	}
	amount, err := utils.ParseAmount(req.Amount)
	if err != nil {
		utils.SendError(w, r, err)
		return
	}

	transfer, replayed, err := h.svc.Transfer(r.Context(), key, dto.TransferInput{
		FromWalletID: req.FromWalletID,
		ToWalletID:   req.ToWalletID,
		Amount:       amount,
		Note:         req.Note,
	})
	if err != nil {
		utils.SendError(w, r, err)
		return
	}
	utils.SendIdempotent(w, replayed, transfer)
}
