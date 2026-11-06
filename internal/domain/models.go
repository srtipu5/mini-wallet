package domain

import "time"

// Transaction types shown in the wallet history.
const (
	TxAddMoney    = "ADD_MONEY"
	TxTransferOut = "TRANSFER_OUT"
	TxTransferIn  = "TRANSFER_IN"
)

const TransferCompleted = "COMPLETED"

// All JSON field names below are part of the public API.

type Wallet struct {
	ID        string    `json:"wallet_id"`
	UserID    string    `json:"user_id"`
	Phone     string    `json:"phone"`
	Currency  string    `json:"currency"`
	Balance   int64     `json:"balance"` // poisha
	CreatedAt time.Time `json:"created_at"`
}

type Transfer struct {
	ID           string    `json:"transfer_id"`
	FromWalletID string    `json:"from_wallet_id"`
	ToWalletID   string    `json:"to_wallet_id"`
	Amount       int64     `json:"amount"`
	Note         *string   `json:"note"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}

// AddMoneyResult is the response of a successful add-money call.
type AddMoneyResult struct {
	TransactionID int64     `json:"transaction_id"`
	WalletID      string    `json:"wallet_id"`
	Amount        int64     `json:"amount"`
	GatewayRef    string    `json:"gateway_ref"`
	BalanceAfter  int64     `json:"balance_after"`
	CreatedAt     time.Time `json:"created_at"`
}

// Transaction is one entry in a wallet's history (ledger).
type Transaction struct {
	ID                   int64     `json:"transaction_id"`
	Type                 string    `json:"type"`
	Amount               int64     `json:"amount"`
	BalanceAfter         int64     `json:"balance_after"`
	CounterpartyWalletID *string   `json:"counterparty_wallet_id"`
	ReferenceID          string    `json:"reference_id"`
	CreatedAt            time.Time `json:"created_at"`
}

// TransactionFilter selects a page of one wallet's history.
// From is inclusive, To is exclusive: created_at in [From, To).
type TransactionFilter struct {
	WalletID string
	Type     string     // "" = all types
	From     *time.Time // nil = no lower bound
	To       *time.Time // nil = no upper bound
	Page     int        // 1-based
	PageSize int
}

// IdempotencyRecord is an Idempotency-Key that was already used successfully.
type IdempotencyRecord struct {
	RequestHash  string
	ResponseBody []byte
}
