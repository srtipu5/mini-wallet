package tests

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestCreateWallet(t *testing.T) {
	resetDB(t)

	r := call(t, "POST", "/api/v1/wallets", `{"user_id":"u_1001","phone":"01711000001"}`, "")
	expectStatus(t, r, http.StatusCreated)

	body := r.JSON(t)
	if body["wallet_id"] == "" || body["user_id"] != "u_1001" || body["currency"] != "BDT" ||
		body["balance"].(float64) != 0 || body["created_at"] == nil {
		t.Fatalf("unexpected body: %s", r.Body)
	}
}

func TestCreateWallet_Duplicates(t *testing.T) {
	resetDB(t)
	expectStatus(t, call(t, "POST", "/api/v1/wallets", `{"user_id":"u_1","phone":"01711000001"}`, ""), 201)

	// Same user, different phone.
	r := call(t, "POST", "/api/v1/wallets", `{"user_id":"u_1","phone":"01711000002"}`, "")
	expectError(t, r, http.StatusConflict, "DUPLICATE_WALLET")

	// Different user, same phone.
	r = call(t, "POST", "/api/v1/wallets", `{"user_id":"u_2","phone":"01711000001"}`, "")
	expectError(t, r, http.StatusConflict, "DUPLICATE_WALLET")
}

func TestCreateWallet_ConcurrentDuplicate(t *testing.T) {
	resetDB(t)
	statuses := make([]int, 10)
	runConcurrently(10, func(i int) {
		statuses[i] = call(t, "POST", "/api/v1/wallets", `{"user_id":"u_race","phone":"01711000009"}`, "").Status
	})

	created := 0
	for _, s := range statuses {
		switch s {
		case 201:
			created++
		case 409:
		default:
			t.Fatalf("unexpected status %d", s)
		}
	}
	if created != 1 {
		t.Fatalf("created %d wallets, want exactly 1", created)
	}
}

func TestCreateWallet_Validation(t *testing.T) {
	resetDB(t)
	cases := map[string]string{
		"empty body":     ``,
		"malformed json": `{"user_id":`,
		"not an object":  `[1,2]`,
		"missing user":   `{"phone":"01711000001"}`,
		"blank user":     `{"user_id":"   ","phone":"01711000001"}`,
		"user too long":  `{"user_id":"` + strings.Repeat("x", 65) + `","phone":"01711000001"}`,
		"missing phone":  `{"user_id":"u_1"}`,
		"bad phone":      `{"user_id":"u_1","phone":"12345"}`,
		"wrong type":     `{"user_id":123,"phone":"01711000001"}`,
		"unknown field":  `{"user_id":"u_1","phone":"01711000001","balance":999}`,
		"trailing data":  `{"user_id":"u_1","phone":"01711000001"} {}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			expectError(t, call(t, "POST", "/api/v1/wallets", body, ""), http.StatusBadRequest, "VALIDATION_ERROR")
		})
	}
}

func TestGetWallet(t *testing.T) {
	resetDB(t)
	id := createWallet(t, "u_1")
	addMoney(t, id, 50000)

	r := call(t, "GET", "/api/v1/wallets/"+id, nil, "")
	expectStatus(t, r, http.StatusOK)
	if r.JSON(t)["balance"].(float64) != 50000 {
		t.Fatalf("unexpected body: %s", r.Body)
	}

	// Uppercase UUID is the same wallet.
	expectStatus(t, call(t, "GET", "/api/v1/wallets/"+strings.ToUpper(id), nil, ""), http.StatusOK)

	// Unknown and malformed ids.
	expectError(t, call(t, "GET", "/api/v1/wallets/00000000-0000-0000-0000-000000000000", nil, ""), 404, "WALLET_NOT_FOUND")
	expectError(t, call(t, "GET", "/api/v1/wallets/not-a-uuid", nil, ""), 404, "WALLET_NOT_FOUND")
}

func TestUnknownRoute(t *testing.T) {
	expectError(t, call(t, "GET", "/api/v1/nope", nil, ""), 404, "NOT_FOUND")
}

func TestCreateWallet_BodyTooLarge(t *testing.T) {
	resetDB(t)
	body := `{"user_id":"u_1","phone":"01711000001","x":"` + strings.Repeat("a", 2<<20) + `"}`
	expectError(t, call(t, "POST", "/api/v1/wallets", body, ""), http.StatusBadRequest, "VALIDATION_ERROR")
}

func TestCreateWallet_ResponseShape(t *testing.T) {
	resetDB(t)
	r := call(t, "POST", "/api/v1/wallets", `{"user_id":"u_1","phone":"01711000001"}`, "")
	expectStatus(t, r, 201)
	if r.Header.Get("Content-Type") != "application/json" {
		t.Fatal("responses must be JSON")
	}
	body := r.JSON(t)
	for _, field := range []string{"wallet_id", "user_id", "phone", "currency", "balance", "created_at"} {
		if _, ok := body[field]; !ok {
			t.Errorf("missing field %q in %s", field, r.Body)
		}
	}
	if _, err := time.Parse(time.RFC3339Nano, body["created_at"].(string)); err != nil {
		t.Errorf("created_at is not ISO-8601: %v", err)
	}
	// GET returns the same wallet.
	got := call(t, "GET", "/api/v1/wallets/"+body["wallet_id"].(string), nil, "").JSON(t)
	if got["wallet_id"] != body["wallet_id"] || got["user_id"] != "u_1" || got["phone"] != "01711000001" {
		t.Fatalf("GET returned %v", got)
	}
}
