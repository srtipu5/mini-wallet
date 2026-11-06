package utils_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/srtipu5/mini-wallet/internal/domain"

	"github.com/srtipu5/mini-wallet/internal/utils"
)

func TestParseAmount(t *testing.T) {
	ok := map[string]int64{"100": 100, "2500000": 2500000, " 5000 ": 5000, "-5": -5}
	for raw, want := range ok {
		got, err := utils.ParseAmount(json.RawMessage(raw))
		if err != nil || got != want {
			t.Errorf("ParseAmount(%s) = %d, %v; want %d", raw, got, err, want)
		}
	}

	// Money must be an integer: floats, exponents, strings, null, missing -> INVALID_AMOUNT.
	bad := []string{"", "null", "100.5", "100.0", "1e3", `"100"`, "true", "99999999999999999999"}
	for _, raw := range bad {
		_, err := utils.ParseAmount(json.RawMessage(raw))
		e, isDomain := domain.AsError(err)
		if !isDomain || e.Code != domain.CodeInvalidAmount {
			t.Errorf("ParseAmount(%s): want INVALID_AMOUNT, got %v", raw, err)
		}
	}
}

func TestIdempotencyKey(t *testing.T) {
	cases := []struct {
		name   string
		header *string // nil = header not sent
		want   string
		code   string
	}{
		{"valid", ptr("abc-123"), "abc-123", ""},
		{"trimmed", ptr("  abc  "), "abc", ""},
		{"max length", ptr(strings.Repeat("k", domain.MaxIdempotencyKeyLength)), strings.Repeat("k", domain.MaxIdempotencyKeyLength), ""},
		{"missing", nil, "", domain.CodeMissingIdempotencyKey},
		{"empty", ptr(""), "", domain.CodeMissingIdempotencyKey},
		{"blank", ptr("   "), "", domain.CodeMissingIdempotencyKey},
		{"too long", ptr(strings.Repeat("k", domain.MaxIdempotencyKeyLength+1)), "", domain.CodeValidation},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/", nil)
			if c.header != nil {
				r.Header.Set("Idempotency-Key", *c.header)
			}
			got, err := utils.IdempotencyKey(r)
			if c.code == "" {
				if err != nil || got != c.want {
					t.Fatalf("IdempotencyKey = %q, %v; want %q", got, err, c.want)
				}
				return
			}
			if e, ok := domain.AsError(err); !ok || e.Code != c.code {
				t.Fatalf("err = %v, want %s", err, c.code)
			}
		})
	}
}

func ptr(s string) *string { return &s }
