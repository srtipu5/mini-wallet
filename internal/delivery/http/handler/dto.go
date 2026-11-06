package handler

import "encoding/json"

// Request bodies. Amounts are json.RawMessage so utils.ParseAmount can
// reject floats, exponents and strings (money is integer poisha only).

type createWalletRequest struct {
	UserID string `json:"user_id"`
	Phone  string `json:"phone"`
}

type addMoneyRequest struct {
	Amount     json.RawMessage `json:"amount"`
	GatewayRef string          `json:"gateway_ref"`
}

type transferRequest struct {
	FromWalletID string          `json:"from_wallet_id"`
	ToWalletID   string          `json:"to_wallet_id"`
	Amount       json.RawMessage `json:"amount"`
	Note         *string         `json:"note"`
}
