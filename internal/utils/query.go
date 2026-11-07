package utils

import (
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/srtipu5/mini-wallet/internal/domain"
)

// bangladeshTime is used for date-only values like 2026-11-05 (UTC+6, no DST).
var bangladeshTime = time.FixedZone("Asia/Dhaka", 6*60*60)

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

// QueryTime parses a date or RFC 3339 date-time. A date-only upper bound moves
// to the next day so the whole day is included.
func QueryTime(q url.Values, name string, isUpperBound bool) (*time.Time, error) {
	raw := strings.TrimSpace(q.Get(name))
	if raw == "" {
		return nil, nil
	}

	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return &t, nil
	}
	if d, err := time.ParseInLocation(time.DateOnly, raw, bangladeshTime); err == nil {
		if isUpperBound {
			d = d.AddDate(0, 0, 1)
		}
		return &d, nil
	}
	return nil, domain.Validation("The " + name + " parameter must be an ISO-8601 date (2026-11-05) or date-time (2026-11-05T10:00:00Z)")
}

// QueryPage reads page (default 1) and page_size (default 20, max 100).
func QueryPage(q url.Values) (page, pageSize int, err error) {
	if page, err = QueryInt(q, "page", 1, 1, domain.MaxPage); err != nil {
		return 0, 0, err
	}
	if pageSize, err = QueryInt(q, "page_size", domain.DefaultPageSize, 1, domain.MaxPageSize); err != nil {
		return 0, 0, err
	}
	return page, pageSize, nil
}
