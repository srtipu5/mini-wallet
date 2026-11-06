// Package utils holds small, reusable helpers for the HTTP layer: JSON request
// decoding, response/error writing and query-parameter parsing. It contains no
// business rules.
package utils

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/srtipu5/mini-wallet/internal/domain"
)

// statusByCode maps every business error code to its HTTP status.
var statusByCode = map[string]int{
	domain.CodeValidation:            http.StatusBadRequest,
	domain.CodeInvalidAmount:         http.StatusBadRequest,
	domain.CodeMissingIdempotencyKey: http.StatusBadRequest,
	domain.CodeSameWalletTransfer:    http.StatusBadRequest,
	domain.CodeWalletNotFound:        http.StatusNotFound,
	domain.CodeNotFound:              http.StatusNotFound,
	domain.CodeDuplicateWallet:       http.StatusConflict,
	domain.CodeDuplicateGatewayRef:   http.StatusConflict,
	domain.CodeIdempotencyKeyReused:  http.StatusUnprocessableEntity,
	domain.CodeInsufficientBalance:   http.StatusUnprocessableEntity,
}

// ErrorBody is the standard error envelope: {"error": {"code", "message"}}.
type ErrorBody struct {
	Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("write response", "error", err)
	}
}

// WriteError renders any error in the standard envelope.
// Unknown (non-business) errors become a generic 500 so internals never leak.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	if e, ok := domain.AsError(err); ok {
		status, known := statusByCode[e.Code]
		if !known {
			status = http.StatusBadRequest
		}
		WriteJSON(w, status, ErrorBody{ErrorDetail{e.Code, e.Message}})
		return
	}

	slog.Error("internal error", "method", r.Method, "path", r.URL.Path, "error", err)
	WriteInternalError(w)
}

// WriteInternalError writes a generic 500 without any details.
func WriteInternalError(w http.ResponseWriter) {
	WriteJSON(w, http.StatusInternalServerError,
		ErrorBody{ErrorDetail{domain.CodeInternal, "internal server error"}})
}

// WriteIdempotent writes a 201 for an idempotent operation and marks replays
// with a header, so clients (and tests) can tell a replay from a fresh execution.
func WriteIdempotent(w http.ResponseWriter, replayed bool, v any) {
	if replayed {
		w.Header().Set("Idempotent-Replayed", "true")
	}
	WriteJSON(w, http.StatusCreated, v)
}
