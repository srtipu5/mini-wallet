package tests

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestTransfer(t *testing.T) {
	resetDB(t)
	a := createWallet(t, "u_a")
	b := createWallet(t, "u_b")
	addMoney(t, a, 50000)

	r := call(t, "POST", "/api/v1/transfers",
		map[string]any{"from_wallet_id": a, "to_wallet_id": b, "amount": 10000, "note": "Lunch"}, "t1")
	expectStatus(t, r, http.StatusCreated)

	body := r.JSON(t)
	if body["transfer_id"] == "" || body["from_wallet_id"] != a || body["to_wallet_id"] != b ||
		body["amount"].(float64) != 10000 || body["status"] != "COMPLETED" ||
		body["note"] != "Lunch" || body["created_at"] == nil {
		t.Fatalf("unexpected body: %s", r.Body)
	}
	if balanceOf(t, a) != 40000 || balanceOf(t, b) != 10000 {
		t.Fatalf("balances a=%d b=%d; want 40000 and 10000", balanceOf(t, a), balanceOf(t, b))
	}
}

func TestTransfer_WholeBalance(t *testing.T) {
	resetDB(t)
	a := createWallet(t, "u_a")
	b := createWallet(t, "u_b")
	addMoney(t, a, 1000)

	expectStatus(t, transfer(t, a, b, 1000, "t1"), 201)
	if balanceOf(t, a) != 0 {
		t.Fatal("sender should be at exactly 0")
	}
}

func TestTransfer_InsufficientBalance(t *testing.T) {
	resetDB(t)
	a := createWallet(t, "u_a")
	b := createWallet(t, "u_b")
	addMoney(t, a, 1000)

	expectError(t, transfer(t, a, b, 1001, "t1"), http.StatusUnprocessableEntity, "INSUFFICIENT_BALANCE")
	if balanceOf(t, a) != 1000 || balanceOf(t, b) != 0 {
		t.Fatal("failed transfer must not change balances")
	}
}

func TestTransfer_Validation(t *testing.T) {
	resetDB(t)
	a := createWallet(t, "u_a")
	b := createWallet(t, "u_b")
	addMoney(t, a, 5000)
	missing := "00000000-0000-0000-0000-000000000000"

	expectError(t, transfer(t, a, b, 1000, ""), 400, "MISSING_IDEMPOTENCY_KEY")
	expectError(t, transfer(t, a, a, 1000, "k1"), 400, "SAME_WALLET_TRANSFER")
	expectError(t, transfer(t, missing, b, 1000, "k2"), 404, "WALLET_NOT_FOUND")
	expectError(t, transfer(t, a, missing, 1000, "k3"), 404, "WALLET_NOT_FOUND")
	expectError(t, transfer(t, a, "garbage", 1000, "k4"), 404, "WALLET_NOT_FOUND")
	expectError(t, transfer(t, a, b, 99, "k5"), 400, "INVALID_AMOUNT")
	expectError(t, transfer(t, a, b, 2500001, "k6"), 400, "INVALID_AMOUNT")
	expectError(t, transfer(t, a, b, -100, "k7"), 400, "INVALID_AMOUNT")
	expectError(t, call(t, "POST", "/api/v1/transfers",
		fmt.Sprintf(`{"from_wallet_id":%q,"to_wallet_id":%q,"amount":100.5}`, a, b), "k8"), 400, "INVALID_AMOUNT")
	expectError(t, call(t, "POST", "/api/v1/transfers",
		fmt.Sprintf(`{"to_wallet_id":%q,"amount":1000}`, b), "k9"), 400, "VALIDATION_ERROR")
	expectError(t, call(t, "POST", "/api/v1/transfers", `not json`, "k10"), 400, "VALIDATION_ERROR")

	if balanceOf(t, a) != 5000 || balanceOf(t, b) != 0 {
		t.Fatal("rejected transfers must not change balances")
	}
}

func TestTransfer_Idempotency(t *testing.T) {
	resetDB(t)
	a := createWallet(t, "u_a")
	b := createWallet(t, "u_b")
	addMoney(t, a, 5000)

	first := transfer(t, a, b, 1000, "same-key")
	expectStatus(t, first, 201)
	second := transfer(t, a, b, 1000, "same-key")
	expectStatus(t, second, 201)
	if !bytes.Equal(first.Body, second.Body) {
		t.Fatalf("replay body differs:\n%s\n%s", first.Body, second.Body)
	}

	expectError(t, transfer(t, a, b, 2000, "same-key"), http.StatusUnprocessableEntity, "IDEMPOTENCY_KEY_REUSED")

	if balanceOf(t, a) != 4000 || balanceOf(t, b) != 1000 {
		t.Fatal("transfer must execute exactly once")
	}
}

// A failed request does not "use up" the key: nothing happened, so a retry
// after fixing the cause (here: topping up) is processed normally.
func TestTransfer_FailedRequestCanBeRetriedWithSameKey(t *testing.T) {
	resetDB(t)
	a := createWallet(t, "u_a")
	b := createWallet(t, "u_b")

	expectError(t, transfer(t, a, b, 1000, "retry-key"), 422, "INSUFFICIENT_BALANCE")
	addMoney(t, a, 1000)
	expectStatus(t, transfer(t, a, b, 1000, "retry-key"), 201)
}

func TestTransfer_ConcurrentSameKey(t *testing.T) {
	resetDB(t)
	a := createWallet(t, "u_a")
	b := createWallet(t, "u_b")
	addMoney(t, a, 10000)

	const n = 20
	results := make([]response, n)
	runConcurrently(n, func(i int) { results[i] = transfer(t, a, b, 1000, "race-key") })

	for _, r := range results {
		expectStatus(t, r, 201)
		if !bytes.Equal(r.Body, results[0].Body) {
			t.Fatal("all responses must be the same original response")
		}
	}
	if balanceOf(t, a) != 9000 || balanceOf(t, b) != 1000 {
		t.Fatalf("a=%d b=%d; transfer must execute exactly once", balanceOf(t, a), balanceOf(t, b))
	}
}

// Many different transfers race to spend the same balance: exactly as many
// succeed as the balance allows, and it never goes negative.
func TestTransfer_ConcurrentDoubleSpend(t *testing.T) {
	resetDB(t)
	a := createWallet(t, "u_a")
	b := createWallet(t, "u_b")
	addMoney(t, a, 10000)

	const n = 50
	statuses := make([]int, n)
	runConcurrently(n, func(i int) {
		statuses[i] = transfer(t, a, b, 1000, fmt.Sprintf("spend-%d", i)).Status
	})

	ok := 0
	for _, s := range statuses {
		switch s {
		case 201:
			ok++
		case 422:
		default:
			t.Fatalf("unexpected status %d", s)
		}
	}
	if ok != 10 || balanceOf(t, a) != 0 || balanceOf(t, b) != 10000 {
		t.Fatalf("ok=%d a=%d b=%d; want 10, 0, 10000", ok, balanceOf(t, a), balanceOf(t, b))
	}
}

// A->B and B->A at the same time must not deadlock, and money is conserved.
func TestTransfer_ConcurrentOppositeDirections(t *testing.T) {
	resetDB(t)
	a := createWallet(t, "u_a")
	b := createWallet(t, "u_b")
	addMoney(t, a, 100000)
	addMoney(t, b, 100000)

	const n = 40
	statuses := make([]int, n)
	runConcurrently(n, func(i int) {
		from, to := a, b
		if i%2 == 1 {
			from, to = b, a
		}
		statuses[i] = transfer(t, from, to, 1000, fmt.Sprintf("x-%d", i)).Status
	})

	for _, s := range statuses {
		if s != 201 {
			t.Fatalf("unexpected status %d", s)
		}
	}
	if balanceOf(t, a) != 100000 || balanceOf(t, b) != 100000 {
		t.Fatal("equal transfers each way should leave balances unchanged")
	}
}

func TestTransfer_Note(t *testing.T) {
	resetDB(t)
	a := createWallet(t, "u_a")
	b := createWallet(t, "u_b")
	addMoney(t, a, 50000)
	send := func(key string, note any) response {
		body := map[string]any{"from_wallet_id": a, "to_wallet_id": b, "amount": 100}
		if note != nil {
			body["note"] = note
		}
		return call(t, "POST", "/api/v1/transfers", body, key)
	}

	// Omitted and blank notes are both returned as null.
	for i, note := range []any{nil, "   "} {
		r := send(fmt.Sprint("n", i), note)
		expectStatus(t, r, 201)
		if v, present := r.JSON(t)["note"]; !present || v != nil {
			t.Fatalf("note %q: want null, got %s", note, r.Body)
		}
	}
	expectStatus(t, send("n-max", strings.Repeat("x", 255)), 201)
	expectError(t, send("n-long", strings.Repeat("x", 256)), 400, "VALIDATION_ERROR")
	expectError(t, send("n-type", 123), 400, "VALIDATION_ERROR")
}

func TestTransfer_CaseInsensitiveWalletIDs(t *testing.T) {
	resetDB(t)
	a := createWallet(t, "u_a")
	b := createWallet(t, "u_b")
	addMoney(t, a, 5000)

	r := transfer(t, strings.ToUpper(a), strings.ToUpper(b), 1000, "k1")
	expectStatus(t, r, 201)
	if body := r.JSON(t); body["from_wallet_id"] != a || body["to_wallet_id"] != b {
		t.Fatalf("ids must be returned in canonical lowercase: %s", r.Body)
	}
	expectError(t, transfer(t, a, strings.ToUpper(a), 1000, "k2"), 400, "SAME_WALLET_TRANSFER")
}

func TestTransfer_NotFoundSaysWhichWallet(t *testing.T) {
	resetDB(t)
	a := createWallet(t, "u_a")
	missing := "00000000-0000-0000-0000-000000000000"

	for _, c := range []struct{ from, to, want string }{
		{missing, a, "sender wallet not found"},
		{a, missing, "receiver wallet not found"},
	} {
		r := transfer(t, c.from, c.to, 1000, randKey())
		expectError(t, r, 404, "WALLET_NOT_FOUND")
		if msg := r.JSON(t)["error"].(map[string]any)["message"]; msg != c.want {
			t.Fatalf("message = %q, want %q", msg, c.want)
		}
	}
}

func TestTransfer_MissingFields(t *testing.T) {
	resetDB(t)
	a := createWallet(t, "u_a")
	b := createWallet(t, "u_b")
	addMoney(t, a, 5000)

	expectError(t, call(t, "POST", "/api/v1/transfers",
		fmt.Sprintf(`{"from_wallet_id":%q,"to_wallet_id":%q}`, a, b), "k1"), 400, "INVALID_AMOUNT")
	expectError(t, call(t, "POST", "/api/v1/transfers",
		fmt.Sprintf(`{"from_wallet_id":%q,"amount":1000}`, a), "k2"), 400, "VALIDATION_ERROR")
	expectError(t, call(t, "POST", "/api/v1/transfers",
		fmt.Sprintf(`{"from_wallet_id":%q,"to_wallet_id":%q,"amount":1000,"fee":5}`, a, b), "k3"), 400, "VALIDATION_ERROR")
	expectError(t, call(t, "POST", "/api/v1/transfers", ``, "k4"), 400, "VALIDATION_ERROR")
	if balanceOf(t, a) != 5000 {
		t.Fatal("rejected transfers must not change balances")
	}
}

// The maximum single transfer is allowed when the balance covers it.
func TestTransfer_MaxAmount(t *testing.T) {
	resetDB(t)
	a := createWallet(t, "u_a")
	b := createWallet(t, "u_b")
	addMoney(t, a, 2500000)
	expectStatus(t, transfer(t, a, b, 2500000, "k"), 201)
	if balanceOf(t, a) != 0 || balanceOf(t, b) != 2500000 {
		t.Fatal("max-amount transfer did not move the full amount")
	}
}
