package dto

import "encoding/json"

// TransferRequest is the body of POST /api/v1/transfers.
// Amount is raw JSON so the handler can reject floats, exponents and strings.
type TransferRequest struct {
	FromWalletID string          `json:"from_wallet_id"`
	ToWalletID   string          `json:"to_wallet_id"`
	Amount       json.RawMessage `json:"amount"`
	Note         *string         `json:"note"`
}

// TransferInput is the transfer use-case input.
//
// The JSON tags matter: the input is hashed to detect an Idempotency-Key
// reused with a different request. Changing them changes every hash.
type TransferInput struct {
	FromWalletID string  `json:"from_wallet_id"`
	ToWalletID   string  `json:"to_wallet_id"`
	Amount       int64   `json:"amount"`
	Note         *string `json:"note"`
}
