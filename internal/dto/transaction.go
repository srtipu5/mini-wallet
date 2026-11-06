package dto

import "github.com/srtipu5/mini-wallet/internal/domain"

// HistoryPage is the response of GET /api/v1/wallets/{wallet_id}/transactions.
type HistoryPage struct {
	Data       []domain.Transaction `json:"data"`
	Pagination Pagination           `json:"pagination"`
}
