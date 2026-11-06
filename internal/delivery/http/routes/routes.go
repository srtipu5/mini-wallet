// Package routes maps every URL to its handler and applies the middleware
// chain. It is the only place to look to see the whole API surface.
package routes

import (
	"net/http"

	"github.com/srtipu5/mini-wallet/internal/application"
	"github.com/srtipu5/mini-wallet/internal/delivery/http/handler"
	"github.com/srtipu5/mini-wallet/internal/delivery/http/middleware"
)

// NewRouter registers every route and wraps them in the middleware chain.
// Go 1.22+ ServeMux supports methods and {params} in patterns.
func NewRouter(svc *application.Service) http.Handler {
	h := handler.New(svc)
	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("GET /health", h.Health)

	// Wallets
	mux.HandleFunc("POST /api/v1/wallets", h.CreateWallet)
	mux.HandleFunc("GET /api/v1/wallets/{wallet_id}", h.GetWallet)
	mux.HandleFunc("POST /api/v1/wallets/{wallet_id}/add-money", h.AddMoney)
	mux.HandleFunc("GET /api/v1/wallets/{wallet_id}/transactions", h.ListTransactions)

	// Transfers
	mux.HandleFunc("POST /api/v1/transfers", h.Transfer)

	// Anything else gets the standard error envelope instead of a plain-text 404.
	mux.HandleFunc("/", h.NotFound)

	// Recover is outermost so it also catches panics in the logger.
	return middleware.Recover(middleware.Logger(mux))
}
