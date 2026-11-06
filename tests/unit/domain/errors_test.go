package domain_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/srtipu5/mini-wallet/internal/domain"
)

func TestError(t *testing.T) {
	e := domain.NewError(domain.CodeInsufficientBalance, "insufficient balance")
	if e.Error() != "INSUFFICIENT_BALANCE: insufficient balance" {
		t.Fatalf("Error() = %q", e.Error())
	}
	if v := domain.Validation("bad"); v.Code != domain.CodeValidation || v.Message != "bad" {
		t.Fatalf("Validation() = %+v", v)
	}
}

func TestAsError(t *testing.T) {
	if e, ok := domain.AsError(domain.ErrWalletNotFound); !ok || e.Code != domain.CodeWalletNotFound {
		t.Fatal("direct domain error not recognised")
	}
	if e, ok := domain.AsError(fmt.Errorf("lookup: %w", domain.ErrSameWalletTransfer)); !ok || e.Code != domain.CodeSameWalletTransfer {
		t.Fatal("wrapped domain error not recognised")
	}
	if _, ok := domain.AsError(errors.New("plain")); ok {
		t.Fatal("plain error must not be a domain error")
	}
	if _, ok := domain.AsError(nil); ok {
		t.Fatal("nil must not be a domain error")
	}
}

// The predefined errors must use the codes required by the API spec.
func TestPredefinedErrorCodes(t *testing.T) {
	cases := map[*domain.Error]string{
		domain.ErrWalletNotFound:       "WALLET_NOT_FOUND",
		domain.ErrSameWalletTransfer:   "SAME_WALLET_TRANSFER",
		domain.ErrInsufficientBalance:  "INSUFFICIENT_BALANCE",
		domain.ErrDuplicateGatewayRef:  "DUPLICATE_GATEWAY_REF",
		domain.ErrIdempotencyKeyReused: "IDEMPOTENCY_KEY_REUSED",
	}
	for e, code := range cases {
		if e.Code != code || e.Message == "" {
			t.Errorf("%+v: want code %s and a message", e, code)
		}
	}
}
