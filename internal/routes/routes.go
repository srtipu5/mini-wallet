// Package routes registers the API routes and middleware.
package routes

import (
	"net/http"

	"github.com/srtipu5/mini-wallet/internal/handler"
	"github.com/srtipu5/mini-wallet/internal/middleware"
	"github.com/srtipu5/mini-wallet/internal/service"
)

func NewRouter(svc *service.Service) http.Handler {
	h := handler.New(svc)
	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("GET /health", h.Health)

	// Wallets
	mux.HandleFunc("POST /api/v1/wallets", h.CreateWallet)
	mux.HandleFunc("GET /api/v1/wallets", h.ListWallets)
	mux.HandleFunc("GET /api/v1/wallets/{wallet_id}", h.GetWallet)
	mux.HandleFunc("POST /api/v1/wallets/{wallet_id}/add-money", h.AddMoney)
	mux.HandleFunc("GET /api/v1/wallets/{wallet_id}/transactions", h.ListTransactions)

	// Transfers
	mux.HandleFunc("POST /api/v1/transfers", h.Transfer)

	mux.HandleFunc("/", h.NotFound)

	// Recover is outermost so it also catches logger panics.
	return middleware.Recover(middleware.Logger(mux))
}
