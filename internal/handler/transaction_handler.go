package handler

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/srtipu5/mini-wallet/internal/domain"
	"github.com/srtipu5/mini-wallet/internal/utils"
)

// GET /api/v1/wallets/{wallet_id}/transactions?type=&from=&to=&page=&page_size=
func (h *Handler) ListTransactions(w http.ResponseWriter, r *http.Request) {
	filter, err := parseTransactionFilter(r.URL.Query())
	if err != nil {
		utils.SendError(w, r, err)
		return
	}
	filter.WalletID = r.PathValue("wallet_id")

	page, err := h.svc.ListTransactions(r.Context(), filter)
	if err != nil {
		utils.SendError(w, r, err)
		return
	}
	utils.SendData(w, http.StatusOK, page)
}

// parseTransactionFilter validates the history query parameters.
func parseTransactionFilter(q url.Values) (domain.TransactionFilter, error) {
	f := domain.TransactionFilter{Page: 1, PageSize: domain.DefaultPageSize}

	if t := strings.ToUpper(strings.TrimSpace(q.Get("type"))); t != "" {
		if !domain.IsValidTransactionType(t) {
			return f, domain.Validation("The type parameter must be one of ADD_MONEY, TRANSFER_OUT, TRANSFER_IN")
		}
		f.Type = t
	}

	var err error
	if f.Page, f.PageSize, err = utils.QueryPage(q); err != nil {
		return f, err
	}

	if f.From, err = utils.QueryTime(q, "from", false); err != nil {
		return f, err
	}
	if f.To, err = utils.QueryTime(q, "to", true); err != nil {
		return f, err
	}
	if f.From != nil && f.To != nil && !f.From.Before(*f.To) {
		return f, domain.Validation("The from date must be before the to date")
	}
	return f, nil
}
