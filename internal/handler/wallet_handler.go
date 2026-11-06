package handler

import (
	"net/http"

	"github.com/srtipu5/mini-wallet/internal/dto"
	"github.com/srtipu5/mini-wallet/internal/utils"
)

// POST /api/v1/wallets
func (h *Handler) CreateWallet(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateWalletRequest
	if err := utils.DecodeJSON(w, r, &req); err != nil {
		utils.SendError(w, r, err)
		return
	}

	wallet, err := h.svc.CreateWallet(r.Context(), req.UserID, req.Phone)
	if err != nil {
		utils.SendError(w, r, err)
		return
	}
	utils.SendData(w, http.StatusCreated, wallet)
}

// GET /api/v1/wallets/{wallet_id}
func (h *Handler) GetWallet(w http.ResponseWriter, r *http.Request) {
	wallet, err := h.svc.GetWallet(r.Context(), r.PathValue("wallet_id"))
	if err != nil {
		utils.SendError(w, r, err)
		return
	}
	utils.SendData(w, http.StatusOK, wallet)
}

// POST /api/v1/wallets/{wallet_id}/add-money
func (h *Handler) AddMoney(w http.ResponseWriter, r *http.Request) {
	key, err := utils.IdempotencyKey(r)
	if err != nil {
		utils.SendError(w, r, err)
		return
	}

	var req dto.AddMoneyRequest
	if err := utils.DecodeJSON(w, r, &req); err != nil {
		utils.SendError(w, r, err)
		return
	}
	amount, err := utils.ParseAmount(req.Amount)
	if err != nil {
		utils.SendError(w, r, err)
		return
	}

	result, replayed, err := h.svc.AddMoney(r.Context(), key, dto.AddMoneyInput{
		WalletID:   r.PathValue("wallet_id"),
		Amount:     amount,
		GatewayRef: req.GatewayRef,
	})
	if err != nil {
		utils.SendError(w, r, err)
		return
	}
	utils.SendIdempotent(w, replayed, result)
}
