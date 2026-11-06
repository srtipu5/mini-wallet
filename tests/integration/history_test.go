package tests

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func historyPath(walletID, query string) string {
	return "/api/v1/wallets/" + walletID + "/transactions" + query
}

func TestHistory(t *testing.T) {
	resetDB(t)
	a := createWallet(t, "u_a")
	b := createWallet(t, "u_b")
	addMoney(t, a, 5000)                // a: ADD_MONEY      -> 5000
	tr := transfer(t, a, b, 2000, "t1") // a: TRANSFER_OUT   -> 3000, b: TRANSFER_IN -> 2000
	expectStatus(t, tr, 201)
	expectStatus(t, transfer(t, b, a, 500, "t2"), 201) // b: TRANSFER_OUT -> 1500, a: TRANSFER_IN -> 3500
	transferID := tr.JSON(t)["transfer_id"]

	r := call(t, "GET", historyPath(a, ""), nil, "")
	expectStatus(t, r, http.StatusOK)
	body := r.JSON(t)
	data := body["data"].([]any)
	if len(data) != 3 {
		t.Fatalf("want 3 entries, got %s", r.Body)
	}

	// Newest first, with correct running balances.
	want := []struct {
		typ          string
		amount       float64
		balanceAfter float64
		counterparty any
	}{
		{"TRANSFER_IN", 500, 3500, b},
		{"TRANSFER_OUT", 2000, 3000, b},
		{"ADD_MONEY", 5000, 5000, nil},
	}
	for i, w := range want {
		e := data[i].(map[string]any)
		if e["type"] != w.typ || e["amount"] != w.amount || e["balance_after"] != w.balanceAfter ||
			e["counterparty_wallet_id"] != w.counterparty || e["reference_id"] == "" || e["created_at"] == nil {
			t.Fatalf("entry %d = %v, want %+v", i, e, w)
		}
	}
	if data[1].(map[string]any)["reference_id"] != transferID {
		t.Fatal("transfer entry should reference the transfer id")
	}

	p := body["pagination"].(map[string]any)
	if p["page"] != 1.0 || p["page_size"] != 20.0 || p["total_items"] != 3.0 || p["total_pages"] != 1.0 {
		t.Fatalf("unexpected pagination: %v", p)
	}
}

func TestHistory_FiltersAndPagination(t *testing.T) {
	resetDB(t)
	a := createWallet(t, "u_a")
	b := createWallet(t, "u_b")
	for i := 0; i < 5; i++ {
		addMoney(t, a, 1000)
	}
	expectStatus(t, transfer(t, a, b, 100, "t1"), 201)

	count := func(query string) (int, map[string]any) {
		r := call(t, "GET", historyPath(a, query), nil, "")
		expectStatus(t, r, 200)
		body := r.JSON(t)
		return len(body["data"].([]any)), body["pagination"].(map[string]any)
	}

	if n, _ := count("?type=ADD_MONEY"); n != 5 {
		t.Fatalf("ADD_MONEY filter: got %d, want 5", n)
	}
	if n, _ := count("?type=TRANSFER_OUT"); n != 1 {
		t.Fatalf("TRANSFER_OUT filter: got %d, want 1", n)
	}
	if n, _ := count("?type=TRANSFER_IN"); n != 0 {
		t.Fatalf("TRANSFER_IN filter: got %d, want 0", n)
	}

	n, p := count("?page=2&page_size=4")
	if n != 2 || p["total_items"] != 6.0 || p["total_pages"] != 2.0 {
		t.Fatalf("page 2: n=%d pagination=%v", n, p)
	}
	if n, _ := count("?page=9&page_size=4"); n != 0 {
		t.Fatal("page past the end should be empty, not an error")
	}

	today := time.Now().UTC().Format(time.DateOnly)
	future := time.Now().AddDate(0, 0, 3).Format(time.DateOnly)
	if n, _ := count("?from=" + future); n != 0 {
		t.Fatal("from in the future should match nothing")
	}
	if n, _ := count("?to=" + time.Now().AddDate(0, 0, -3).Format(time.DateOnly)); n != 0 {
		t.Fatal("to in the past should match nothing")
	}
	yesterday := time.Now().AddDate(0, 0, -1).Format(time.DateOnly)
	if n, _ := count("?from=" + yesterday + "&to=" + future); n != 6 {
		t.Fatalf("range around today (%s) should match all 6", today)
	}
}

func TestHistory_Errors(t *testing.T) {
	resetDB(t)
	a := createWallet(t, "u_a")

	expectError(t, call(t, "GET", historyPath("00000000-0000-0000-0000-000000000000", ""), nil, ""), 404, "WALLET_NOT_FOUND")
	for _, q := range []string{"?type=REFUND", "?page=0", "?page_size=101", "?page_size=x", "?from=bad", "?from=2026-11-05&to=2026-11-01"} {
		expectError(t, call(t, "GET", historyPath(a, q), nil, ""), 400, "VALIDATION_ERROR")
	}

	// A new wallet has an empty list, not null.
	r := call(t, "GET", historyPath(a, ""), nil, "")
	expectStatus(t, r, 200)
	if data, ok := r.JSON(t)["data"].([]any); !ok || len(data) != 0 {
		t.Fatalf("want empty array, got %s", r.Body)
	}
}

// The receiver's history shows TRANSFER_IN with the sender as counterparty.
func TestHistory_ReceiverSide(t *testing.T) {
	resetDB(t)
	a := createWallet(t, "u_a")
	b := createWallet(t, "u_b")
	addMoney(t, a, 5000)
	tr := transfer(t, a, b, 1200, "t1")
	expectStatus(t, tr, 201)

	r := call(t, "GET", historyPath(b, ""), nil, "")
	expectStatus(t, r, 200)
	data := r.JSON(t)["data"].([]any)
	if len(data) != 1 {
		t.Fatalf("want 1 entry, got %s", r.Body)
	}
	e := data[0].(map[string]any)
	if e["type"] != "TRANSFER_IN" || e["amount"] != 1200.0 || e["balance_after"] != 1200.0 ||
		e["counterparty_wallet_id"] != a || e["reference_id"] != tr.JSON(t)["transfer_id"] {
		t.Fatalf("unexpected entry: %v", e)
	}
}

func TestHistory_AddMoneyReferencesGatewayRef(t *testing.T) {
	resetDB(t)
	a := createWallet(t, "u_a")
	expectStatus(t, call(t, "POST", addMoneyPath(a), `{"amount":1000,"gateway_ref":"GW-REF-42"}`, "k"), 201)

	e := call(t, "GET", historyPath(a, ""), nil, "").JSON(t)["data"].([]any)[0].(map[string]any)
	if e["reference_id"] != "GW-REF-42" || e["counterparty_wallet_id"] != nil {
		t.Fatalf("unexpected entry: %v", e)
	}
}

func TestHistory_QueryFormats(t *testing.T) {
	resetDB(t)
	a := createWallet(t, "u_a")
	addMoney(t, a, 1000)
	count := func(query string) int {
		r := call(t, "GET", historyPath(a, query), nil, "")
		expectStatus(t, r, 200)
		return len(r.JSON(t)["data"].([]any))
	}

	if count("?type=add_money") != 1 {
		t.Fatal("type must be case-insensitive")
	}
	hourAgo := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	inHour := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	if count("?from="+hourAgo+"&to="+inHour) != 1 {
		t.Fatal("date-time range around now must match")
	}
	if count("?to="+hourAgo) != 0 {
		t.Fatal("date-time upper bound in the past must match nothing")
	}
	// Uppercase wallet id in the path.
	if r := call(t, "GET", historyPath(strings.ToUpper(a), ""), nil, ""); r.Status != 200 {
		t.Fatalf("uppercase wallet id: %d", r.Status)
	}
}

func TestHistory_PageSizeLimits(t *testing.T) {
	resetDB(t)
	a := createWallet(t, "u_a")
	for i := 0; i < 25; i++ {
		addMoney(t, a, 100)
	}
	page := func(query string) (int, map[string]any) {
		r := call(t, "GET", historyPath(a, query), nil, "")
		expectStatus(t, r, 200)
		body := r.JSON(t)
		return len(body["data"].([]any)), body["pagination"].(map[string]any)
	}

	if n, p := page(""); n != 20 || p["page_size"] != 20.0 || p["total_items"] != 25.0 || p["total_pages"] != 2.0 {
		t.Fatalf("default page: n=%d %v", n, p)
	}
	if n, _ := page("?page=2"); n != 5 {
		t.Fatalf("page 2: n=%d, want 5", n)
	}
	if n, p := page("?page_size=1"); n != 1 || p["total_pages"] != 25.0 {
		t.Fatalf("page_size=1: n=%d %v", n, p)
	}
	if n, _ := page("?page_size=100"); n != 25 {
		t.Fatalf("page_size=100: n=%d", n)
	}
	expectError(t, call(t, "GET", historyPath(a, "?page_size=0"), nil, ""), 400, "VALIDATION_ERROR")
	expectError(t, call(t, "GET", historyPath(a, "?page=-1"), nil, ""), 400, "VALIDATION_ERROR")
}
