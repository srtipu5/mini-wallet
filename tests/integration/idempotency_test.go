package tests

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// Cross-cutting Idempotency-Key rules shared by add-money and transfers.

func TestIdempotency_KeyFormat(t *testing.T) {
	resetDB(t)
	a := createWallet(t, "u_a")
	body := func(ref string) string { return fmt.Sprintf(`{"amount":1000,"gateway_ref":%q}`, ref) }

	// A whitespace-only key counts as missing.
	expectError(t, call(t, "POST", addMoneyPath(a), body("GW-1"), "   "), 400, "MISSING_IDEMPOTENCY_KEY")
	// 255 characters is the limit.
	expectStatus(t, call(t, "POST", addMoneyPath(a), body("GW-2"), strings.Repeat("k", 255)), 201)
	expectError(t, call(t, "POST", addMoneyPath(a), body("GW-3"), strings.Repeat("k", 256)), 400, "VALIDATION_ERROR")
	expectError(t, transfer(t, a, createWallet(t, "u_b"), 100, strings.Repeat("k", 256)), 400, "VALIDATION_ERROR")
}

// Add-money and transfers have separate key spaces.
func TestIdempotency_KeysScopedPerOperation(t *testing.T) {
	resetDB(t)
	a := createWallet(t, "u_a")
	b := createWallet(t, "u_b")

	expectStatus(t, call(t, "POST", addMoneyPath(a), `{"amount":5000,"gateway_ref":"GW-1"}`, "shared"), 201)
	r := transfer(t, a, b, 1000, "shared")
	expectStatus(t, r, 201)
	if r.Header.Get("Idempotent-Replayed") != "" {
		t.Fatal("a transfer must not replay an add-money response")
	}
	if balanceOf(t, a) != 4000 || balanceOf(t, b) != 1000 {
		t.Fatal("both operations must have run")
	}
}

// The wallet id is part of the add-money request, so reusing a key for
// another wallet is a different request.
func TestIdempotency_AddMoneyKeyReusedForAnotherWallet(t *testing.T) {
	resetDB(t)
	a := createWallet(t, "u_a")
	b := createWallet(t, "u_b")

	expectStatus(t, call(t, "POST", addMoneyPath(a), `{"amount":1000,"gateway_ref":"GW-1"}`, "k"), 201)
	expectError(t, call(t, "POST", addMoneyPath(b), `{"amount":1000,"gateway_ref":"GW-1"}`, "k"),
		http.StatusUnprocessableEntity, "IDEMPOTENCY_KEY_REUSED")
	if balanceOf(t, b) != 0 {
		t.Fatal("wallet b must not be credited")
	}
}

// Validation errors are returned before the key is looked at, so the key
// stays free for the corrected request.
func TestIdempotency_ValidationErrorDoesNotUseKey(t *testing.T) {
	resetDB(t)
	a := createWallet(t, "u_a")

	expectError(t, call(t, "POST", addMoneyPath(a), `{"amount":5,"gateway_ref":"GW-1"}`, "k"), 400, "INVALID_AMOUNT")
	r := call(t, "POST", addMoneyPath(a), `{"amount":1000,"gateway_ref":"GW-1"}`, "k")
	expectStatus(t, r, 201)
	if r.Header.Get("Idempotent-Replayed") != "" {
		t.Fatal("the corrected request must run, not replay")
	}
}

// A request that fails inside the transaction releases its key.
func TestIdempotency_FailedAddMoneyReleasesKey(t *testing.T) {
	resetDB(t)
	a := createWallet(t, "u_a")
	expectStatus(t, call(t, "POST", addMoneyPath(a), `{"amount":1000,"gateway_ref":"GW-1"}`, "k1"), 201)

	expectError(t, call(t, "POST", addMoneyPath(a), `{"amount":1000,"gateway_ref":"GW-1"}`, "k2"), 409, "DUPLICATE_GATEWAY_REF")
	expectStatus(t, call(t, "POST", addMoneyPath(a), `{"amount":500,"gateway_ref":"GW-2"}`, "k2"), 201)
	if balanceOf(t, a) != 1500 {
		t.Fatalf("balance = %d, want 1500", balanceOf(t, a))
	}
}

func TestIdempotency_TransferReplayHeader(t *testing.T) {
	resetDB(t)
	a := createWallet(t, "u_a")
	b := createWallet(t, "u_b")
	addMoney(t, a, 5000)

	first := transfer(t, a, b, 1000, "k")
	if first.Header.Get("Idempotent-Replayed") != "" {
		t.Fatal("the first response must not be marked as a replay")
	}
	second := transfer(t, a, b, 1000, "k")
	expectStatus(t, second, 201)
	if second.Header.Get("Idempotent-Replayed") != "true" {
		t.Fatal("the replay must set Idempotent-Replayed: true")
	}
}

// The key is only remembered after success: replaying it returns the first
// successful response even if the balance has changed since.
func TestIdempotency_ReplayReturnsOriginalEvenAfterBalanceChanges(t *testing.T) {
	resetDB(t)
	a := createWallet(t, "u_a")
	b := createWallet(t, "u_b")
	addMoney(t, a, 1000)

	expectStatus(t, transfer(t, a, b, 1000, "k"), 201) // a is now 0
	// A fresh transfer would fail with INSUFFICIENT_BALANCE, but a replay must not.
	expectStatus(t, transfer(t, a, b, 1000, "k"), 201)
	if balanceOf(t, a) != 0 || balanceOf(t, b) != 1000 {
		t.Fatal("replay must not move money")
	}
}
