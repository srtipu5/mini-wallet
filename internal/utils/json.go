package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/srtipu5/mini-wallet/internal/domain"
)

const maxBodyBytes = 1 << 20 // 1 MB

// DecodeJSON reads exactly one JSON object into dst.
// It rejects unknown fields, wrong types, oversized bodies and trailing data.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		var syntaxErr *json.SyntaxError
		var typeErr *json.UnmarshalTypeError
		var sizeErr *http.MaxBytesError
		switch {
		case errors.Is(err, io.EOF):
			return domain.Validation("Request body is required")
		case errors.As(err, &syntaxErr), errors.Is(err, io.ErrUnexpectedEOF):
			return domain.Validation("Request body is not valid JSON")
		case errors.As(err, &typeErr):
			return domain.Validation(fmt.Sprintf("Field %q has the wrong type", typeErr.Field))
		case errors.As(err, &sizeErr):
			return domain.Validation("Request body is too large")
		case strings.HasPrefix(err.Error(), "json: unknown field"):
			return domain.Validation("Unknown field " + strings.TrimPrefix(err.Error(), "json: unknown field "))
		default:
			return domain.Validation("Invalid request body")
		}
	}

	if dec.Decode(&struct{}{}) != io.EOF {
		return domain.Validation("Request body must contain a single JSON object")
	}
	return nil
}
