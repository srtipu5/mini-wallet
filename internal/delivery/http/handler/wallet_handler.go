package handler

import (
	"net/http"

	"github.com/srtipu5/mini-wallet/internal/application"
	"github.com/srtipu5/mini-wallet/internal/utils"
)

// POST /api/v1/wallets
func (h *Handler) createWallet(w http.ResponseWriter, r *http.Request) {
	var req createWalletRequest
	if err := utils.DecodeJSON(w, r, &req); err != nil {
		utils.WriteError(w, r, err)
		return
	}

	wallet, err := h.svc.CreateWallet(r.Context(), req.UserID, req.Phone)
	if err != nil {
		utils.WriteError(w, r, err)
		return
	}
	utils.WriteJSON(w, http.StatusCreated, wallet)
}

// GET /api/v1/wallets/{wallet_id}
func (h *Handler) getWallet(w http.ResponseWriter, r *http.Request) {
	wallet, err := h.svc.GetWallet(r.Context(), r.PathValue("wallet_id"))
	if err != nil {
		utils.WriteError(w, r, err)
		return
	}
	utils.WriteJSON(w, http.StatusOK, wallet)
}

// POST /api/v1/wallets/{wallet_id}/add-money
func (h *Handler) addMoney(w http.ResponseWriter, r *http.Request) {
	key, err := utils.IdempotencyKey(r)
	if err != nil {
		utils.WriteError(w, r, err)
		return
	}

	var req addMoneyRequest
	if err := utils.DecodeJSON(w, r, &req); err != nil {
		utils.WriteError(w, r, err)
		return
	}
	amount, err := utils.ParseAmount(req.Amount)
	if err != nil {
		utils.WriteError(w, r, err)
		return
	}

	result, replayed, err := h.svc.AddMoney(r.Context(), key, application.AddMoneyInput{
		WalletID:   r.PathValue("wallet_id"),
		Amount:     amount,
		GatewayRef: req.GatewayRef,
	})
	if err != nil {
		utils.WriteError(w, r, err)
		return
	}
	utils.WriteIdempotent(w, replayed, result)
}
