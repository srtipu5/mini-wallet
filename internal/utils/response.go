// Package utils holds HTTP helpers for decoding requests and writing responses.
package utils

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

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

func SendData(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("write response", "error", err)
	}
}

// SendError writes the error envelope; non-domain errors become a generic 500.
func SendError(w http.ResponseWriter, r *http.Request, err error) {
	if e, ok := domain.AsError(err); ok {
		status, known := statusByCode[e.Code]
		if !known {
			status = http.StatusBadRequest
		}
		SendData(w, status, ErrorBody{ErrorDetail{e.Code, formatMessage(e.Message)}})
		return
	}

	slog.Error("internal error", "method", r.Method, "path", r.URL.Path, "error", err)
	SendInternalError(w)
}

// SendInternalError writes a generic 500 without any details.
func SendInternalError(w http.ResponseWriter) {
	SendData(w, http.StatusInternalServerError,
		ErrorBody{ErrorDetail{domain.CodeInternal, "Internal server error."}})
}

// SendIdempotent writes 201 and sets Idempotent-Replayed on replays.
func SendIdempotent(w http.ResponseWriter, replayed bool, v any) {
	if replayed {
		w.Header().Set("Idempotent-Replayed", "true")
	}
	SendData(w, http.StatusCreated, v)
}

// formatMessage capitalizes the message and ends it with a full stop.
func formatMessage(s string) string {
	s = strings.TrimSpace(s)
	r, size := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return s
	}
	s = string(unicode.ToUpper(r)) + s[size:]
	if !strings.HasSuffix(s, ".") && !strings.HasSuffix(s, "!") && !strings.HasSuffix(s, "?") {
		s += "."
	}
	return s
}
