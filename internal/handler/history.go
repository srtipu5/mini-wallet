package handler

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/srtipu5/mini-wallet/internal/domain"
	"github.com/srtipu5/mini-wallet/internal/repository"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
	maxPage         = 100_000 // keeps OFFSET sane
)

// Date-only values (2026-11-05) are read in Bangladesh time (UTC+6, no DST).
var dhaka = time.FixedZone("Asia/Dhaka", 6*60*60)

// GET /api/v1/wallets/{wallet_id}/transactions?type=&from=&to=&page=&page_size=
func (h *Handler) listTransactions(w http.ResponseWriter, r *http.Request) {
	filter, err := parseHistoryFilter(r.URL.Query())
	if err != nil {
		writeError(w, r, err)
		return
	}
	filter.WalletID = r.PathValue("wallet_id")

	page, err := h.svc.ListTransactions(r.Context(), filter)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func parseHistoryFilter(q url.Values) (repository.HistoryFilter, error) {
	f := repository.HistoryFilter{Page: 1, PageSize: defaultPageSize}

	if t := strings.ToUpper(strings.TrimSpace(q.Get("type"))); t != "" {
		if t != domain.TxAddMoney && t != domain.TxTransferOut && t != domain.TxTransferIn {
			return f, domain.Validation("type must be one of ADD_MONEY, TRANSFER_OUT, TRANSFER_IN")
		}
		f.Type = t
	}

	var err error
	if f.Page, err = intParam(q, "page", 1, 1, maxPage); err != nil {
		return f, err
	}
	if f.PageSize, err = intParam(q, "page_size", defaultPageSize, 1, maxPageSize); err != nil {
		return f, err
	}

	if f.From, err = timeParam(q, "from", false); err != nil {
		return f, err
	}
	if f.To, err = timeParam(q, "to", true); err != nil {
		return f, err
	}
	if f.From != nil && f.To != nil && !f.From.Before(*f.To) {
		return f, domain.Validation("from must be before to")
	}
	return f, nil
}

// intParam reads an optional integer query parameter within [min, max].
func intParam(q url.Values, name string, def, min, max int) (int, error) {
	raw := strings.TrimSpace(q.Get(name))
	if raw == "" {
		return def, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < min || n > max {
		return 0, domain.Validation(name + " must be an integer between " + strconv.Itoa(min) + " and " + strconv.Itoa(max))
	}
	return n, nil
}

// timeParam reads an ISO-8601 date (2026-11-05) or date-time (2026-11-05T10:00:00Z).
//
// The range is half-open [from, to). For a date-only "to" we move to the
// start of the NEXT day, so "to=2026-11-05" includes the whole of Nov 5.
func timeParam(q url.Values, name string, isUpperBound bool) (*time.Time, error) {
	raw := strings.TrimSpace(q.Get(name))
	if raw == "" {
		return nil, nil
	}

	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return &t, nil
	}
	if d, err := time.ParseInLocation(time.DateOnly, raw, dhaka); err == nil {
		if isUpperBound {
			d = d.AddDate(0, 0, 1)
		}
		return &d, nil
	}
	return nil, domain.Validation(name + " must be an ISO-8601 date (2026-11-05) or date-time (2026-11-05T10:00:00Z)")
}
