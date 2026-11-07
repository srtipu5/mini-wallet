package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/srtipu5/mini-wallet/internal/domain"
	"github.com/srtipu5/mini-wallet/internal/routes"
	"github.com/srtipu5/mini-wallet/internal/service"
)

// recordingStore captures the history filter the handler builds from the
// query string, so query parsing can be tested through the public router.
type recordingStore struct {
	got      *domain.TransactionFilter
	listPage int // page passed to Wallets.List (0 = not called)
	listSize int
}

func (s *recordingStore) Repos() service.Repositories {
	return service.Repositories{Wallets: anyWallet{s}, Transactions: recordingTransactions{s}}
}

func (s *recordingStore) BeginTx(context.Context) (service.Tx, error) {
	panic("read-only endpoints must not open a transaction")
}

func (s *recordingStore) Ping(context.Context) error { return nil }

// anyWallet reports that every wallet exists, and records list calls.
type anyWallet struct{ s *recordingStore }

func (a anyWallet) List(_ context.Context, page, pageSize int) ([]domain.Wallet, int, error) {
	a.s.listPage, a.s.listSize = page, pageSize
	return nil, 0, nil
}

func (anyWallet) Get(_ context.Context, id string) (domain.Wallet, error) {
	return domain.Wallet{ID: id}, nil
}
func (anyWallet) Create(context.Context, string, string) (domain.Wallet, error)     { panic("unused") }
func (anyWallet) LockBalances(context.Context, ...string) (map[string]int64, error) { panic("unused") }
func (anyWallet) AdjustBalance(context.Context, string, int64) (int64, error)       { panic("unused") }

type recordingTransactions struct{ s *recordingStore }

func (r recordingTransactions) List(_ context.Context, f domain.TransactionFilter) ([]domain.Transaction, int, error) {
	r.s.got = &f
	return nil, 0, nil
}
func (recordingTransactions) Insert(context.Context, string, *domain.Transaction) error {
	panic("unused")
}

const historyURL = "/api/v1/wallets/3f2b8c4e-1a2b-4c3d-8e9f-0a1b2c3d4e5f/transactions"

// filterFor sends GET history?query and returns the parsed filter (nil if rejected).
func filterFor(t *testing.T, query string) (*domain.TransactionFilter, int) {
	t.Helper()
	store := &recordingStore{}
	rec := httptest.NewRecorder()
	routes.NewRouter(service.New(store)).ServeHTTP(rec, httptest.NewRequest("GET", historyURL+query, nil))
	return store.got, rec.Code
}

func TestTransactionFilter_Defaults(t *testing.T) {
	f, status := filterFor(t, "")
	if status != http.StatusOK || f == nil {
		t.Fatalf("status = %d", status)
	}
	if f.Page != 1 || f.PageSize != 20 || f.Type != "" || f.From != nil || f.To != nil {
		t.Errorf("unexpected defaults: %+v", f)
	}
	if f.WalletID != "3f2b8c4e-1a2b-4c3d-8e9f-0a1b2c3d4e5f" {
		t.Errorf("wallet id from the path not used: %q", f.WalletID)
	}
}

func TestTransactionFilter_Valid(t *testing.T) {
	f, status := filterFor(t, "?type=transfer_in&page=3&page_size=100&from=2026-11-01&to=2026-11-05")
	if status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	if f.Type != domain.TxTransferIn || f.Page != 3 || f.PageSize != 100 {
		t.Errorf("unexpected filter: %+v", f)
	}
	// Dates are Bangladesh time; a date-only "to" includes the whole day.
	wantFrom := time.Date(2026, 10, 31, 18, 0, 0, 0, time.UTC)
	wantTo := time.Date(2026, 11, 5, 18, 0, 0, 0, time.UTC)
	if !f.From.Equal(wantFrom) || !f.To.Equal(wantTo) {
		t.Errorf("from=%v to=%v; want %v and %v", f.From, f.To, wantFrom, wantTo)
	}

	// A date-time "to" is used exactly as given ("+" must be URL-encoded).
	f, _ = filterFor(t, "?from=2026-11-01T10:00:00Z&to=2026-11-01T18:00:00%2B06:00")
	if !f.To.Equal(time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)) {
		t.Errorf("date-time 'to' must be used as given, got %v", f.To)
	}
}

func TestTransactionFilter_Invalid(t *testing.T) {
	for _, q := range []string{
		"?type=REFUND",
		"?page=0",
		"?page=abc",
		"?page=100001",
		"?page_size=0",
		"?page_size=101",
		"?from=yesterday",
		"?to=2026-13-01",
		"?from=2026-11-05&to=2026-11-01", // from after to
		"?from=2026-11-05&to=2026-11-04T18:00:00Z", // from == to (empty range)
	} {
		f, status := filterFor(t, q)
		if status != http.StatusBadRequest || f != nil {
			t.Errorf("%s: status %d, reached repository=%v; want 400 and no query", q, status, f != nil)
		}
	}
}

// listCall sends GET /api/v1/wallets?query and returns the page and page
// size the handler passed to the repository (0, 0 if it was rejected).
func listCall(t *testing.T, query string) (page, size, status int) {
	t.Helper()
	store := &recordingStore{}
	rec := httptest.NewRecorder()
	routes.NewRouter(service.New(store)).ServeHTTP(rec, httptest.NewRequest("GET", "/api/v1/wallets"+query, nil))
	return store.listPage, store.listSize, rec.Code
}

func TestListWallets_Pagination(t *testing.T) {
	if p, s, st := listCall(t, ""); st != http.StatusOK || p != 1 || s != 20 {
		t.Fatalf("defaults: status %d page %d size %d; want 200, 1, 20", st, p, s)
	}
	if p, s, st := listCall(t, "?page=3&page_size=100"); st != http.StatusOK || p != 3 || s != 100 {
		t.Fatalf("explicit: status %d page %d size %d; want 200, 3, 100", st, p, s)
	}
	for _, q := range []string{"?page=0", "?page=x", "?page_size=0", "?page_size=101", "?page_size=abc"} {
		if p, _, st := listCall(t, q); st != http.StatusBadRequest || p != 0 {
			t.Errorf("%s: status %d, reached repository=%v; want 400 and no query", q, st, p != 0)
		}
	}
}
