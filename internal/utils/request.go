package utils

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/srtipu5/mini-wallet/internal/domain"
)

// IdempotencyKey reads and validates the Idempotency-Key header.
func IdempotencyKey(r *http.Request) (string, error) {
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" {
		return "", domain.NewError(domain.CodeMissingIdempotencyKey, "Idempotency-Key header is required")
	}
	if len(key) > domain.MaxIdempotencyKeyLength {
		return "", domain.Validation("Idempotency-Key must be at most 255 characters")
	}
	return key, nil
}

// ParseAmount accepts only a plain JSON integer (e.g. 50000).
// Floats (100.5, 1e3), strings ("100"), null and missing values are rejected,
// because money is always an integer number of poisha.
func ParseAmount(raw json.RawMessage) (int64, error) {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return 0, domain.NewError(domain.CodeInvalidAmount, "Amount is required")
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, domain.NewError(domain.CodeInvalidAmount, "Amount must be an integer number of poisha")
	}
	return n, nil
}
