package handler

import (
	"net/url"
	"testing"
	"time"

	"github.com/srtipu5/mini-wallet/internal/domain"
)

func TestParseTransactionFilter_Defaults(t *testing.T) {
	f, err := parseTransactionFilter(url.Values{})
	if err != nil {
		t.Fatal(err)
	}
	if f.Page != 1 || f.PageSize != 20 || f.Type != "" || f.From != nil || f.To != nil {
		t.Errorf("unexpected defaults: %+v", f)
	}
}

func TestParseTransactionFilter_Valid(t *testing.T) {
	f, err := parseTransactionFilter(url.Values{
		"type":      {"transfer_in"},
		"page":      {"3"},
		"page_size": {"100"},
		"from":      {"2026-11-01"},
		"to":        {"2026-11-05"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if f.Type != domain.TxTransferIn || f.Page != 3 || f.PageSize != 100 {
		t.Errorf("unexpected filter: %+v", f)
	}
	// Dates are Bangladesh time; date-only "to" includes the whole day.
	wantFrom := time.Date(2026, 10, 31, 18, 0, 0, 0, time.UTC)
	wantTo := time.Date(2026, 11, 5, 18, 0, 0, 0, time.UTC)
	if !f.From.Equal(wantFrom) || !f.To.Equal(wantTo) {
		t.Errorf("from=%v to=%v; want %v and %v", f.From, f.To, wantFrom, wantTo)
	}

	f, err = parseTransactionFilter(url.Values{"from": {"2026-11-01T10:00:00Z"}, "to": {"2026-11-01T18:00:00+06:00"}})
	if err != nil {
		t.Fatal(err)
	}
	if !f.To.Equal(time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)) {
		t.Errorf("date-time 'to' must be used as given, got %v", f.To)
	}
}

func TestParseTransactionFilter_Invalid(t *testing.T) {
	cases := []url.Values{
		{"type": {"REFUND"}},
		{"page": {"0"}},
		{"page": {"abc"}},
		{"page_size": {"0"}},
		{"page_size": {"101"}},
		{"from": {"yesterday"}},
		{"to": {"2026-13-01"}},
		{"from": {"2026-11-05"}, "to": {"2026-11-01"}}, // from after to
	}
	for _, q := range cases {
		if _, err := parseTransactionFilter(q); err == nil {
			t.Errorf("parseTransactionFilter(%v) should fail", q)
		}
	}
}
