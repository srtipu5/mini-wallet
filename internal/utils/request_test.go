package utils

import (
	"encoding/json"
	"testing"

	"github.com/srtipu5/mini-wallet/internal/domain"
)

func TestParseAmount(t *testing.T) {
	ok := map[string]int64{"100": 100, "2500000": 2500000, " 5000 ": 5000, "-5": -5}
	for raw, want := range ok {
		got, err := ParseAmount(json.RawMessage(raw))
		if err != nil || got != want {
			t.Errorf("ParseAmount(%s) = %d, %v; want %d", raw, got, err, want)
		}
	}

	// Money must be an integer: floats, exponents, strings, null, missing -> INVALID_AMOUNT.
	bad := []string{"", "null", "100.5", "100.0", "1e3", `"100"`, "true", "99999999999999999999"}
	for _, raw := range bad {
		_, err := ParseAmount(json.RawMessage(raw))
		e, isDomain := domain.AsError(err)
		if !isDomain || e.Code != domain.CodeInvalidAmount {
			t.Errorf("ParseAmount(%s): want INVALID_AMOUNT, got %v", raw, err)
		}
	}
}
