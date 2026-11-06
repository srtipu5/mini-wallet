package handler

import (
	"net/http"

	"github.com/srtipu5/mini-wallet/internal/application"
	"github.com/srtipu5/mini-wallet/internal/delivery/http/middleware"
)

// NewRouter registers every route and wraps them in the middleware chain.
// Go 1.22+ ServeMux supports methods and {params} in patterns.
func NewRouter(svc *application.Service) http.Handler {
	h := New(svc)
	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("GET /health", h.health)

	// Wallets
	mux.HandleFunc("POST /api/v1/wallets", h.createWallet)
	mux.HandleFunc("GET /api/v1/wallets/{wallet_id}", h.getWallet)
	mux.HandleFunc("POST /api/v1/wallets/{wallet_id}/add-money", h.addMoney)
	mux.HandleFunc("GET /api/v1/wallets/{wallet_id}/transactions", h.listTransactions)

	// Transfers
	mux.HandleFunc("POST /api/v1/transfers", h.transfer)

	// Anything else gets the standard error envelope instead of a plain-text 404.
	mux.HandleFunc("/", h.notFound)

	return middleware.Recover(middleware.Logger(mux))
}
