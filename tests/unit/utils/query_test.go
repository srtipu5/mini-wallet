package utils_test

import (
	"net/url"
	"testing"
	"time"

	"github.com/srtipu5/mini-wallet/internal/utils"
)

func TestQueryInt(t *testing.T) {
	cases := []struct {
		raw     string
		want    int
		wantErr bool
	}{
		{"", 20, false}, // default
		{"  ", 20, false},
		{"1", 1, false},     // min
		{"100", 100, false}, // max
		{" 7 ", 7, false},
		{"0", 0, true},
		{"101", 0, true},
		{"-1", 0, true},
		{"abc", 0, true},
		{"1.5", 0, true},
	}
	for _, c := range cases {
		got, err := utils.QueryInt(url.Values{"n": {c.raw}}, "n", 20, 1, 100)
		if (err != nil) != c.wantErr || (!c.wantErr && got != c.want) {
			t.Errorf("QueryInt(%q) = %d, %v; want %d, err=%v", c.raw, got, err, c.want, c.wantErr)
		}
	}
}

func TestQueryTime(t *testing.T) {
	q := func(v string) url.Values { return url.Values{"t": {v}} }

	if got, err := utils.QueryTime(url.Values{}, "t", false); got != nil || err != nil {
		t.Fatalf("missing param = %v, %v; want nil, nil", got, err)
	}

	// Date-only values are midnight in Bangladesh time (UTC+6).
	got, err := utils.QueryTime(q("2026-11-05"), "t", false)
	if err != nil || !got.Equal(time.Date(2026, 11, 4, 18, 0, 0, 0, time.UTC)) {
		t.Fatalf("lower bound = %v, %v", got, err)
	}
	// A date-only upper bound moves to the next day, so the whole day is included.
	got, err = utils.QueryTime(q("2026-11-05"), "t", true)
	if err != nil || !got.Equal(time.Date(2026, 11, 5, 18, 0, 0, 0, time.UTC)) {
		t.Fatalf("upper bound = %v, %v", got, err)
	}
	// Date-times are used exactly as given, with any offset.
	for _, isUpper := range []bool{false, true} {
		got, err = utils.QueryTime(q("2026-11-05T10:30:00+06:00"), "t", isUpper)
		if err != nil || !got.Equal(time.Date(2026, 11, 5, 4, 30, 0, 0, time.UTC)) {
			t.Fatalf("date-time (upper=%v) = %v, %v", isUpper, got, err)
		}
	}
	got, err = utils.QueryTime(q("2026-11-05T10:30:00.123Z"), "t", false)
	if err != nil || got.Nanosecond() != 123_000_000 {
		t.Fatalf("fractional seconds = %v, %v", got, err)
	}

	for _, bad := range []string{"yesterday", "2026-13-01", "2026-11-31", "05-11-2026", "2026-11-05 10:00", "1730800000"} {
		if _, err := utils.QueryTime(q(bad), "t", false); err == nil {
			t.Errorf("QueryTime(%q) should fail", bad)
		}
	}
}

func TestQueryPage(t *testing.T) {
	page, size, err := utils.QueryPage(url.Values{})
	if err != nil || page != 1 || size != 20 {
		t.Fatalf("defaults = %d, %d, %v; want 1, 20, nil", page, size, err)
	}
	page, size, err = utils.QueryPage(url.Values{"page": {"4"}, "page_size": {"100"}})
	if err != nil || page != 4 || size != 100 {
		t.Fatalf("explicit = %d, %d, %v; want 4, 100, nil", page, size, err)
	}
	for _, q := range []url.Values{{"page": {"0"}}, {"page_size": {"101"}}, {"page_size": {"x"}}} {
		if _, _, err := utils.QueryPage(q); err == nil {
			t.Errorf("QueryPage(%v) should fail", q)
		}
	}
}
