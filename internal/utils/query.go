package utils

import (
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/srtipu5/mini-wallet/internal/domain"
)

// BangladeshTime is used for date-only values like 2026-11-05 (UTC+6, no DST).
var BangladeshTime = time.FixedZone("Asia/Dhaka", 6*60*60)

// QueryInt reads an optional integer query parameter within [min, max].
func QueryInt(q url.Values, name string, def, min, max int) (int, error) {
	raw := strings.TrimSpace(q.Get(name))
	if raw == "" {
		return def, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < min || n > max {
		return 0, domain.Validation("The " + name + " parameter must be an integer between " + strconv.Itoa(min) + " and " + strconv.Itoa(max))
	}
	return n, nil
}

// QueryTime reads an optional ISO-8601 date (2026-11-05) or date-time
// (2026-11-05T10:00:00Z) query parameter.
//
// Ranges are half-open [from, to). For a date-only upper bound we move to the
// start of the NEXT day, so "to=2026-11-05" includes the whole of Nov 5.
func QueryTime(q url.Values, name string, isUpperBound bool) (*time.Time, error) {
	raw := strings.TrimSpace(q.Get(name))
	if raw == "" {
		return nil, nil
	}

	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return &t, nil
	}
	if d, err := time.ParseInLocation(time.DateOnly, raw, BangladeshTime); err == nil {
		if isUpperBound {
			d = d.AddDate(0, 0, 1)
		}
		return &d, nil
	}
	return nil, domain.Validation("The " + name + " parameter must be an ISO-8601 date (2026-11-05) or date-time (2026-11-05T10:00:00Z)")
}
