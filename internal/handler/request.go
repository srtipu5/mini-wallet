package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/srtipu5/mini-wallet/internal/domain"
)

const maxBodyBytes = 1 << 20 // 1 MB

// decodeJSON reads exactly one JSON object into dst.
// It rejects unknown fields, wrong types, oversized bodies and trailing data.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		var syntaxErr *json.SyntaxError
		var typeErr *json.UnmarshalTypeError
		var sizeErr *http.MaxBytesError
		switch {
		case errors.Is(err, io.EOF):
			return domain.Validation("request body is required")
		case errors.As(err, &syntaxErr), errors.Is(err, io.ErrUnexpectedEOF):
			return domain.Validation("request body is not valid JSON")
		case errors.As(err, &typeErr):
			return domain.Validation(fmt.Sprintf("field %q has the wrong type", typeErr.Field))
		case errors.As(err, &sizeErr):
			return domain.Validation("request body is too large")
		case strings.HasPrefix(err.Error(), "json: unknown field"):
			return domain.Validation(strings.TrimPrefix(err.Error(), "json: "))
		default:
			return domain.Validation("invalid request body")
		}
	}

	if dec.Decode(&struct{}{}) != io.EOF {
		return domain.Validation("request body must contain a single JSON object")
	}
	return nil
}

// parseAmount accepts only a plain JSON integer (e.g. 50000).
// Floats (100.5, 1e3), strings ("100"), null and missing values are rejected,
// because money is always an integer number of poisha.
func parseAmount(raw json.RawMessage) (int64, error) {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return 0, domain.NewError(domain.CodeInvalidAmount, "amount is required")
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, domain.NewError(domain.CodeInvalidAmount, "amount must be an integer number of poisha")
	}
	return n, nil
}

// idempotencyKey reads and validates the Idempotency-Key header.
func idempotencyKey(r *http.Request) (string, error) {
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" {
		return "", domain.NewError(domain.CodeMissingIdempotencyKey, "Idempotency-Key header is required")
	}
	if len(key) > domain.MaxIdempotencyKeyLength {
		return "", domain.Validation("Idempotency-Key must be at most 255 characters")
	}
	return key, nil
}
