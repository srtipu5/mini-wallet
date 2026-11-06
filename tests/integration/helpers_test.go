// Package tests contains end-to-end tests: real HTTP router + real Postgres.
// They run only when TEST_DATABASE_URL is set (see README).
package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/srtipu5/mini-wallet/internal/delivery/http/routes"
	"github.com/srtipu5/mini-wallet/internal/infrastructure/postgres"
	"github.com/srtipu5/mini-wallet/internal/infrastructure/repository"
	"github.com/srtipu5/mini-wallet/internal/service"
)

var (
	server *httptest.Server
	db     *pgxpool.Pool
)

func TestMain(m *testing.M) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		fmt.Println("TEST_DATABASE_URL not set: skipping integration tests")
		os.Exit(0)
	}

	var err error
	db, err = postgres.Connect(context.Background(), url)
	if err != nil {
		fmt.Println("connect:", err)
		os.Exit(1)
	}
	if err := postgres.Migrate(url); err != nil {
		fmt.Println("migrate:", err)
		os.Exit(1)
	}

	server = httptest.NewServer(routes.NewRouter(service.New(repository.NewStore(db))))
	code := m.Run()
	server.Close()
	db.Close()
	os.Exit(code)
}

// resetDB wipes all data so every test starts from an empty database.
func resetDB(t *testing.T) {
	t.Helper()
	_, err := db.Exec(context.Background(),
		`TRUNCATE idempotency_keys, transactions, transfers, wallets RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatal(err)
	}
}

type response struct {
	Status int
	Header http.Header
	Body   []byte
}

// JSON decodes the body into a generic map.
func (r response) JSON(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(r.Body, &m); err != nil {
		t.Fatalf("invalid JSON body %q: %v", r.Body, err)
	}
	return m
}

// ErrorCode returns error.code from the standard error envelope.
func (r response) ErrorCode(t *testing.T) string {
	t.Helper()
	e, ok := r.JSON(t)["error"].(map[string]any)
	if !ok {
		t.Fatalf("no error envelope in %s", r.Body)
	}
	return e["code"].(string)
}

// call sends a request. body may be a string (sent as-is) or any value (sent as JSON).
// idemKey "" means: do not send the Idempotency-Key header.
func call(t *testing.T, method, path string, body any, idemKey string) response {
	t.Helper()

	var reader io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		reader = bytes.NewBufferString(b)
	default:
		raw, err := json.Marshal(b)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewBuffer(raw)
	}

	req, err := http.NewRequest(method, server.URL+path, reader)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if idemKey != "" {
		req.Header.Set("Idempotency-Key", idemKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response{Status: resp.StatusCode, Header: resp.Header, Body: data}
}

func expectStatus(t *testing.T, r response, status int) {
	t.Helper()
	if r.Status != status {
		t.Fatalf("status = %d, want %d; body: %s", r.Status, status, r.Body)
	}
}

func expectError(t *testing.T, r response, status int, code string) {
	t.Helper()
	expectStatus(t, r, status)
	if got := r.ErrorCode(t); got != code {
		t.Fatalf("error code = %s, want %s; body: %s", got, code, r.Body)
	}
}

// --- shortcuts for common setup ---

var phoneSeq int
var phoneMu sync.Mutex

func nextPhone() string {
	phoneMu.Lock()
	defer phoneMu.Unlock()
	phoneSeq++
	return fmt.Sprintf("0171%07d", phoneSeq)
}

func createWallet(t *testing.T, userID string) string {
	t.Helper()
	r := call(t, "POST", "/api/v1/wallets", map[string]any{"user_id": userID, "phone": nextPhone()}, "")
	expectStatus(t, r, http.StatusCreated)
	return r.JSON(t)["wallet_id"].(string)
}

func addMoney(t *testing.T, walletID string, amount int64) {
	t.Helper()
	ref := fmt.Sprintf("GW-%s-%d-%s", walletID[:8], amount, randKey())
	r := call(t, "POST", "/api/v1/wallets/"+walletID+"/add-money",
		map[string]any{"amount": amount, "gateway_ref": ref}, randKey())
	expectStatus(t, r, http.StatusCreated)
}

func balanceOf(t *testing.T, walletID string) int64 {
	t.Helper()
	r := call(t, "GET", "/api/v1/wallets/"+walletID, nil, "")
	expectStatus(t, r, http.StatusOK)
	return int64(r.JSON(t)["balance"].(float64))
}

func transfer(t *testing.T, from, to string, amount int64, key string) response {
	t.Helper()
	return call(t, "POST", "/api/v1/transfers",
		map[string]any{"from_wallet_id": from, "to_wallet_id": to, "amount": amount}, key)
}

var keySeq int
var keyMu sync.Mutex

func randKey() string {
	keyMu.Lock()
	defer keyMu.Unlock()
	keySeq++
	return fmt.Sprintf("key-%d", keySeq)
}

// runConcurrently starts n goroutines at the same moment and waits for all.
func runConcurrently(n int, fn func(i int)) {
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			fn(i)
		}(i)
	}
	close(start)
	wg.Wait()
}
