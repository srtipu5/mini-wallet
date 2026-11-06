package tests

import (
	"bytes"
	"fmt"
	"net/http"
	"testing"
)

func addMoneyPath(walletID string) string { return "/api/v1/wallets/" + walletID + "/add-money" }

func TestAddMoney(t *testing.T) {
	resetDB(t)
	id := createWallet(t, "u_1")

	r := call(t, "POST", addMoneyPath(id), `{"amount":50000,"gateway_ref":"GW-1"}`, "k1")
	expectStatus(t, r, http.StatusCreated)
	body := r.JSON(t)
	if body["amount"].(float64) != 50000 || body["balance_after"].(float64) != 50000 || body["gateway_ref"] != "GW-1" {
		t.Fatalf("unexpected body: %s", r.Body)
	}
	if got := balanceOf(t, id); got != 50000 {
		t.Fatalf("balance = %d, want 50000", got)
	}
}

func TestAddMoney_AmountRules(t *testing.T) {
	resetDB(t)
	id := createWallet(t, "u_1")

	// Boundaries are allowed.
	expectStatus(t, call(t, "POST", addMoneyPath(id), `{"amount":100,"gateway_ref":"GW-min"}`, "k-min"), 201)
	expectStatus(t, call(t, "POST", addMoneyPath(id), `{"amount":2500000,"gateway_ref":"GW-max"}`, "k-max"), 201)

	bad := []string{"99", "2500001", "0", "-100", "100.5", "1e3", `"500"`, "null"}
	for i, amount := range bad {
		body := fmt.Sprintf(`{"amount":%s,"gateway_ref":"GW-bad-%d"}`, amount, i)
		r := call(t, "POST", addMoneyPath(id), body, fmt.Sprintf("k-bad-%d", i))
		expectError(t, r, http.StatusBadRequest, "INVALID_AMOUNT")
	}
	// Missing amount.
	expectError(t, call(t, "POST", addMoneyPath(id), `{"gateway_ref":"GW-x"}`, "k-x"), 400, "INVALID_AMOUNT")

	if got := balanceOf(t, id); got != 100+2500000 {
		t.Fatalf("balance = %d, want %d", got, 100+2500000)
	}
}

func TestAddMoney_Validation(t *testing.T) {
	resetDB(t)
	id := createWallet(t, "u_1")

	// Missing Idempotency-Key header.
	expectError(t, call(t, "POST", addMoneyPath(id), `{"amount":1000,"gateway_ref":"GW-1"}`, ""),
		http.StatusBadRequest, "MISSING_IDEMPOTENCY_KEY")

	// Missing gateway_ref.
	expectError(t, call(t, "POST", addMoneyPath(id), `{"amount":1000}`, "k1"), 400, "VALIDATION_ERROR")

	// Unknown wallet.
	expectError(t, call(t, "POST", addMoneyPath("00000000-0000-0000-0000-000000000000"),
		`{"amount":1000,"gateway_ref":"GW-2"}`, "k2"), 404, "WALLET_NOT_FOUND")
}

func TestAddMoney_DuplicateGatewayRef(t *testing.T) {
	resetDB(t)
	a := createWallet(t, "u_a")
	b := createWallet(t, "u_b")

	expectStatus(t, call(t, "POST", addMoneyPath(a), `{"amount":1000,"gateway_ref":"GW-1"}`, "k1"), 201)

	// Same gateway payment, new idempotency key -> must not credit again.
	expectError(t, call(t, "POST", addMoneyPath(a), `{"amount":1000,"gateway_ref":"GW-1"}`, "k2"),
		http.StatusConflict, "DUPLICATE_GATEWAY_REF")
	// Not even into a different wallet.
	expectError(t, call(t, "POST", addMoneyPath(b), `{"amount":1000,"gateway_ref":"GW-1"}`, "k3"),
		http.StatusConflict, "DUPLICATE_GATEWAY_REF")

	if balanceOf(t, a) != 1000 || balanceOf(t, b) != 0 {
		t.Fatal("duplicate gateway_ref changed a balance")
	}
}

func TestAddMoney_Idempotency(t *testing.T) {
	resetDB(t)
	id := createWallet(t, "u_1")
	body := `{"amount":1000,"gateway_ref":"GW-1"}`

	first := call(t, "POST", addMoneyPath(id), body, "same-key")
	expectStatus(t, first, 201)

	// Same key + same body (even formatted differently) -> original response, no new credit.
	second := call(t, "POST", addMoneyPath(id), `{ "gateway_ref": "GW-1", "amount": 1000 }`, "same-key")
	expectStatus(t, second, 201)
	if !bytes.Equal(first.Body, second.Body) {
		t.Fatalf("replay body differs:\n%s\n%s", first.Body, second.Body)
	}
	if second.Header.Get("Idempotent-Replayed") != "true" {
		t.Fatal("replay should set Idempotent-Replayed header")
	}

	// Same key + different body -> 422.
	expectError(t, call(t, "POST", addMoneyPath(id), `{"amount":2000,"gateway_ref":"GW-1"}`, "same-key"),
		http.StatusUnprocessableEntity, "IDEMPOTENCY_KEY_REUSED")

	if got := balanceOf(t, id); got != 1000 {
		t.Fatalf("balance = %d, want 1000 (credited once)", got)
	}
}

func TestAddMoney_ConcurrentSameKey(t *testing.T) {
	resetDB(t)
	id := createWallet(t, "u_1")

	const n = 20
	results := make([]response, n)
	runConcurrently(n, func(i int) {
		results[i] = call(t, "POST", addMoneyPath(id), `{"amount":1000,"gateway_ref":"GW-1"}`, "race-key")
	})

	for _, r := range results {
		expectStatus(t, r, 201)
		if !bytes.Equal(r.Body, results[0].Body) {
			t.Fatalf("responses differ:\n%s\n%s", r.Body, results[0].Body)
		}
	}
	if got := balanceOf(t, id); got != 1000 {
		t.Fatalf("balance = %d, want 1000 (exactly one execution)", got)
	}
}

func TestAddMoney_ConcurrentSameGatewayRef(t *testing.T) {
	resetDB(t)
	id := createWallet(t, "u_1")

	const n = 20
	statuses := make([]int, n)
	runConcurrently(n, func(i int) {
		statuses[i] = call(t, "POST", addMoneyPath(id),
			`{"amount":1000,"gateway_ref":"GW-race"}`, fmt.Sprintf("key-%d", i)).Status
	})

	ok := 0
	for _, s := range statuses {
		if s == 201 {
			ok++
		} else if s != 409 {
			t.Fatalf("unexpected status %d", s)
		}
	}
	if ok != 1 || balanceOf(t, id) != 1000 {
		t.Fatalf("successes=%d balance=%d; want 1 and 1000", ok, balanceOf(t, id))
	}
}
