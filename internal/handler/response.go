package handler

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

type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("write response", "error", err)
	}
}

// writeError renders any error in the standard envelope.
// Unknown (non-business) errors become a generic 500 so internals never leak.
func writeError(w http.ResponseWriter, r *http.Request, err error) {
	if e, ok := domain.AsError(err); ok {
		status, known := statusByCode[e.Code]
		if !known {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, errorBody{errorDetail{e.Code, e.Message}})
		return
	}

	slog.Error("internal error", "method", r.Method, "path", r.URL.Path, "error", err)
	writeJSON(w, http.StatusInternalServerError,
		errorBody{errorDetail{domain.CodeInternal, "internal server error"}})
}
