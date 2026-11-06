package tests

import (
	"net/http"
	"testing"
)

func listWallets(t *testing.T, query string) ([]map[string]any, map[string]any) {
	t.Helper()
	r := call(t, "GET", "/api/v1/wallets"+query, nil, "")
	expectStatus(t, r, http.StatusOK)
	body := r.JSON(t)
	var data []map[string]any
	for _, w := range body["data"].([]any) {
		data = append(data, w.(map[string]any))
	}
	return data, body["pagination"].(map[string]any)
}

func TestListWallets_Empty(t *testing.T) {
	resetDB(t)
	r := call(t, "GET", "/api/v1/wallets", nil, "")
	expectStatus(t, r, http.StatusOK)
	if data, ok := r.JSON(t)["data"].([]any); !ok || len(data) != 0 {
		t.Fatalf("want an empty array, got %s", r.Body)
	}
}

func TestListWallets_PaginationAndOrder(t *testing.T) {
	resetDB(t)
	var ids []string
	for i := 0; i < 25; i++ {
		ids = append(ids, createWallet(t, "u_"+string(rune('a'+i))))
	}
	addMoney(t, ids[24], 5000)

	// Default page: 20 newest wallets, newest first.
	data, p := listWallets(t, "")
	if len(data) != 20 || p["page"] != 1.0 || p["page_size"] != 20.0 || p["total_items"] != 25.0 || p["total_pages"] != 2.0 {
		t.Fatalf("default page: %d items, pagination %v", len(data), p)
	}
	if data[0]["wallet_id"] != ids[24] || data[19]["wallet_id"] != ids[5] {
		t.Fatalf("wrong order: first %v, last %v", data[0]["wallet_id"], data[19]["wallet_id"])
	}
	// Each item is a full wallet, with its current balance.
	for _, field := range []string{"wallet_id", "user_id", "phone", "currency", "balance", "created_at"} {
		if _, ok := data[0][field]; !ok {
			t.Fatalf("missing field %q in %v", field, data[0])
		}
	}
	if data[0]["balance"] != 5000.0 {
		t.Fatalf("balance = %v, want 5000", data[0]["balance"])
	}

	// Second page: the 5 oldest.
	data, _ = listWallets(t, "?page=2")
	if len(data) != 5 || data[4]["wallet_id"] != ids[0] {
		t.Fatalf("page 2: %d items, last %v", len(data), data[len(data)-1]["wallet_id"])
	}

	// Walking all pages with a small page size returns every wallet once.
	seen := map[any]bool{}
	for page := 1; page <= 9; page++ {
		data, _ = listWallets(t, "?page_size=3&page="+string(rune('0'+page)))
		for _, w := range data {
			if seen[w["wallet_id"]] {
				t.Fatalf("wallet %v returned twice", w["wallet_id"])
			}
			seen[w["wallet_id"]] = true
		}
	}
	if len(seen) != 25 {
		t.Fatalf("saw %d wallets across pages, want 25", len(seen))
	}

	// Past the end: empty page, not an error.
	if data, _ = listWallets(t, "?page=99"); len(data) != 0 {
		t.Fatal("a page past the end must be empty")
	}
}

func TestListWallets_InvalidParams(t *testing.T) {
	resetDB(t)
	for _, q := range []string{"?page=0", "?page=-1", "?page=x", "?page_size=0", "?page_size=101"} {
		expectError(t, call(t, "GET", "/api/v1/wallets"+q, nil, ""), http.StatusBadRequest, "VALIDATION_ERROR")
	}
}
