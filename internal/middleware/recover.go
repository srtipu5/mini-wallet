// Package middleware has small HTTP wrappers shared by all routes.
package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/srtipu5/mini-wallet/internal/utils"
)

// Recover turns a panic into a 500 with the standard error envelope,
// instead of dropping the connection.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic", "error", rec, "stack", string(debug.Stack()))
				utils.SendInternalError(w)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
