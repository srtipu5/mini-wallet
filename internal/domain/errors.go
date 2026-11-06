package domain

import "errors"

// Error codes returned to API clients in {"error": {"code": ..., "message": ...}}.
const (
	CodeValidation            = "VALIDATION_ERROR"
	CodeInvalidAmount         = "INVALID_AMOUNT"
	CodeMissingIdempotencyKey = "MISSING_IDEMPOTENCY_KEY"
	CodeWalletNotFound        = "WALLET_NOT_FOUND"
	CodeSameWalletTransfer    = "SAME_WALLET_TRANSFER"
	CodeInsufficientBalance   = "INSUFFICIENT_BALANCE"
	CodeDuplicateWallet       = "DUPLICATE_WALLET"
	CodeDuplicateGatewayRef   = "DUPLICATE_GATEWAY_REF"
	CodeIdempotencyKeyReused  = "IDEMPOTENCY_KEY_REUSED"
	CodeNotFound              = "NOT_FOUND"
	CodeInternal              = "INTERNAL_ERROR"
)

// Error is a business error that is safe to show to the client.
// The HTTP layer decides which status code each Code maps to.
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

func NewError(code, message string) *Error { return &Error{Code: code, Message: message} }

// Validation is a shortcut for the most common error kind.
func Validation(message string) *Error { return NewError(CodeValidation, message) }

// Predefined errors used by the services.
var (
	ErrWalletNotFound       = NewError(CodeWalletNotFound, "Wallet not found")
	ErrSameWalletTransfer   = NewError(CodeSameWalletTransfer, "Cannot transfer to the same wallet")
	ErrInsufficientBalance  = NewError(CodeInsufficientBalance, "Wallet does not have enough balance for this transfer")
	ErrDuplicateGatewayRef  = NewError(CodeDuplicateGatewayRef, "The gateway_ref has already been used")
	ErrIdempotencyKeyReused = NewError(CodeIdempotencyKeyReused, "Idempotency-Key was already used with a different request body")
)

// AsError extracts a *domain.Error from err, if there is one.
func AsError(err error) (*Error, bool) {
	var e *Error
	ok := errors.As(err, &e)
	return e, ok
}
