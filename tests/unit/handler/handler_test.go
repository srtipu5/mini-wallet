package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/srtipu5/mini-wallet/internal/application"

	"github.com/srtipu5/mini-wallet/internal/delivery/http/routes"
)

// stubStore lets the router run without a database. Its repositories are
// nil, so any request that wrongly reaches the database panics, and the
// Recover middleware turns that into a 500 the tests would catch.
type stubStore struct{ pingErr error }

func (s stubStore) Repos() application.Repositories { return application.Repositories{} }
func (s stubStore) WithinTx(_ context.Context, fn func(application.Repositories) error) error {
	return fn(application.Repositories{})
}
func (s stubStore) Ping(context.Context) error { return s.pingErr }

func serve(t *testing.T, store application.Store, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	routes.NewRouter(application.NewService(store)).ServeHTTP(rec, req)
	return rec
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct{ Code string } `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid body %q: %v", rec.Body, err)
	}
	return body.Error.Code
}

func TestHealth(t *testing.T) {
	rec := serve(t, stubStore{}, "GET", "/health", "", nil)
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"status":"ok"}` {
		t.Fatalf("healthy: %d %s", rec.Code, rec.Body)
	}

	rec = serve(t, stubStore{pingErr: errors.New("connection refused")}, "GET", "/health", "", nil)
	if rec.Code != http.StatusServiceUnavailable || strings.TrimSpace(rec.Body.String()) != `{"status":"database unavailable"}` {
		t.Fatalf("db down: %d %s", rec.Code, rec.Body)
	}
}

func TestUnknownRoute(t *testing.T) {
	for _, path := range []string{"/", "/api/v1/nope", "/api/v2/wallets"} {
		rec := serve(t, stubStore{}, "GET", path, "", nil)
		if rec.Code != http.StatusNotFound || errorCode(t, rec) != "NOT_FOUND" {
			t.Errorf("%s: %d %s", path, rec.Code, rec.Body)
		}
		if rec.Header().Get("Content-Type") != "application/json" {
			t.Errorf("%s: unknown routes must still return JSON", path)
		}
	}
}

// Bad requests are rejected by the handler/service before any database call.
func TestRequestsRejectedBeforeDatabase(t *testing.T) {
	const wallet = "/api/v1/wallets/3f2b8c4e-1a2b-4c3d-8e9f-0a1b2c3d4e5f"
	key := map[string]string{"Idempotency-Key": "k1"}

	cases := []struct {
		name, method, path, body string
		headers                  map[string]string
		status                   int
		code                     string
	}{
		{"create wallet: bad JSON", "POST", "/api/v1/wallets", `{`, nil, 400, "VALIDATION_ERROR"},
		{"create wallet: bad phone", "POST", "/api/v1/wallets", `{"user_id":"u","phone":"1"}`, nil, 400, "VALIDATION_ERROR"},
		{"get wallet: malformed id", "GET", "/api/v1/wallets/xyz", "", nil, 404, "WALLET_NOT_FOUND"},

		{"add money: no key", "POST", wallet + "/add-money", `{"amount":1000,"gateway_ref":"G"}`, nil, 400, "MISSING_IDEMPOTENCY_KEY"},
		{"add money: bad JSON", "POST", wallet + "/add-money", `nope`, key, 400, "VALIDATION_ERROR"},
		{"add money: float amount", "POST", wallet + "/add-money", `{"amount":10.5,"gateway_ref":"G"}`, key, 400, "INVALID_AMOUNT"},
		{"add money: string amount", "POST", wallet + "/add-money", `{"amount":"1000","gateway_ref":"G"}`, key, 400, "INVALID_AMOUNT"},
		{"add money: amount too small", "POST", wallet + "/add-money", `{"amount":50,"gateway_ref":"G"}`, key, 400, "INVALID_AMOUNT"},
		{"add money: malformed wallet", "POST", "/api/v1/wallets/xyz/add-money", `{"amount":1000,"gateway_ref":"G"}`, key, 404, "WALLET_NOT_FOUND"},

		{"transfer: no key", "POST", "/api/v1/transfers", `{}`, nil, 400, "MISSING_IDEMPOTENCY_KEY"},
		{"transfer: unknown field", "POST", "/api/v1/transfers", `{"amount":1000,"fee":1}`, key, 400, "VALIDATION_ERROR"},
		{"transfer: missing amount", "POST", "/api/v1/transfers", `{"from_wallet_id":"a","to_wallet_id":"b"}`, key, 400, "INVALID_AMOUNT"},
		{"transfer: same wallet", "POST", "/api/v1/transfers",
			`{"from_wallet_id":"3f2b8c4e-1a2b-4c3d-8e9f-0a1b2c3d4e5f","to_wallet_id":"3f2b8c4e-1a2b-4c3d-8e9f-0a1b2c3d4e5f","amount":1000}`,
			key, 400, "SAME_WALLET_TRANSFER"},

		{"history: bad type", "GET", wallet + "/transactions?type=REFUND", "", nil, 400, "VALIDATION_ERROR"},
		{"history: page_size too big", "GET", wallet + "/transactions?page_size=500", "", nil, 400, "VALIDATION_ERROR"},
		{"history: malformed wallet", "GET", "/api/v1/wallets/xyz/transactions", "", nil, 404, "WALLET_NOT_FOUND"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := serve(t, stubStore{}, c.method, c.path, c.body, c.headers)
			if rec.Code != c.status || errorCode(t, rec) != c.code {
				t.Fatalf("got %d %s; want %d %s", rec.Code, rec.Body, c.status, c.code)
			}
		})
	}
}

// Routes only answer to their own HTTP method.
func TestRoutesAreMethodSpecific(t *testing.T) {
	cases := [][2]string{
		{"GET", "/api/v1/transfers"},
		{"DELETE", "/api/v1/wallets/3f2b8c4e-1a2b-4c3d-8e9f-0a1b2c3d4e5f"},
		{"GET", "/api/v1/wallets/3f2b8c4e-1a2b-4c3d-8e9f-0a1b2c3d4e5f/add-money"},
		{"POST", "/health"},
	}
	for _, c := range cases {
		rec := serve(t, stubStore{}, c[0], c[1], "", nil)
		if rec.Code < 400 || rec.Code >= 500 {
			t.Errorf("%s %s: status %d, want a 4xx", c[0], c[1], rec.Code)
		}
	}
}
