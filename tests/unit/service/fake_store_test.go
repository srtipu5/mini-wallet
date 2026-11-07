package service_test

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/srtipu5/mini-wallet/internal/domain"

	"github.com/srtipu5/mini-wallet/internal/service"
)

// fakeStore is an in-memory service.Store. A transaction snapshots the state
// on BeginTx and restores it on Rollback unless it was committed.
type fakeStore struct {
	wallets   map[string]domain.Wallet
	ledger    []ledgerRow
	transfers []domain.Transfer
	idem      map[string]domain.IdempotencyRecord // "scope|key"

	nextWallet, nextTx, nextTransfer int

	// Instrumentation for assertions.
	txBegun    int        // number of BeginTx calls
	lockCalls  [][]string // ids passed to each LockBalances call
	walletGets int        // number of Wallets.Get calls

	// Fault injection.
	pingErr error
	// failAt makes one step fail: "lock", "debit", "credit", "transfer", or a
	// ledger entry type ("ADD_MONEY", "TRANSFER_OUT", "TRANSFER_IN").
	failAt string
}

type ledgerRow struct {
	walletID string
	tx       domain.Transaction
}

var errInjected = fmt.Errorf("injected failure")

var fixedNow = time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)

func newFakeStore() *fakeStore {
	return &fakeStore{
		wallets: map[string]domain.Wallet{},
		idem:    map[string]domain.IdempotencyRecord{},
	}
}

var _ service.Store = (*fakeStore)(nil)

func (f *fakeStore) Repos() service.Repositories {
	return service.Repositories{
		Wallets:      fakeWallets{f},
		Transactions: fakeTransactions{f},
		Transfers:    fakeTransfers{f},
		Idempotency:  fakeIdempotency{f},
	}
}

func (f *fakeStore) BeginTx(context.Context) (service.Tx, error) {
	f.txBegun++
	return &fakeTx{f: f, snapshot: f.snapshot()}, nil
}

func (f *fakeStore) Ping(context.Context) error { return f.pingErr }

type fakeTx struct {
	f        *fakeStore
	snapshot fakeState
	done     bool // committed or rolled back
}

func (t *fakeTx) Repos() service.Repositories { return t.f.Repos() }

func (t *fakeTx) Commit(context.Context) error {
	t.done = true
	return nil
}

func (t *fakeTx) Rollback(context.Context) error {
	if !t.done {
		t.f.restore(t.snapshot)
		t.done = true
	}
	return nil
}

type fakeState struct {
	wallets      map[string]domain.Wallet
	ledger       []ledgerRow
	transfers    []domain.Transfer
	idem         map[string]domain.IdempotencyRecord
	nextTx       int
	nextTransfer int
}

func (f *fakeStore) snapshot() fakeState {
	s := fakeState{
		wallets:      map[string]domain.Wallet{},
		ledger:       append([]ledgerRow(nil), f.ledger...),
		transfers:    append([]domain.Transfer(nil), f.transfers...),
		idem:         map[string]domain.IdempotencyRecord{},
		nextTx:       f.nextTx,
		nextTransfer: f.nextTransfer,
	}
	for k, v := range f.wallets {
		s.wallets[k] = v
	}
	for k, v := range f.idem {
		s.idem[k] = v
	}
	return s
}

func (f *fakeStore) restore(s fakeState) {
	f.wallets, f.ledger, f.transfers, f.idem = s.wallets, s.ledger, s.transfers, s.idem
	f.nextTx, f.nextTransfer = s.nextTx, s.nextTransfer
}

// entriesOf returns a wallet's ledger entries, oldest first.
func (f *fakeStore) entriesOf(walletID string) []domain.Transaction {
	var out []domain.Transaction
	for _, row := range f.ledger {
		if row.walletID == walletID {
			out = append(out, row.tx)
		}
	}
	return out
}

// --- wallets ---

type fakeWallets struct{ f *fakeStore }

func (r fakeWallets) Create(_ context.Context, userID, phone string) (domain.Wallet, error) {
	for _, w := range r.f.wallets {
		if w.UserID == userID || w.Phone == phone {
			return domain.Wallet{}, domain.NewError(domain.CodeDuplicateWallet, "duplicate")
		}
	}
	r.f.nextWallet++
	w := domain.Wallet{
		// Sequential ids, so ordering between wallets is predictable.
		ID:        fmt.Sprintf("00000000-0000-4000-8000-%012d", r.f.nextWallet),
		UserID:    userID,
		Phone:     phone,
		Currency:  "BDT",
		CreatedAt: fixedNow,
	}
	r.f.wallets[w.ID] = w
	return w, nil
}

func (r fakeWallets) List(_ context.Context, page, pageSize int) ([]domain.Wallet, int, error) {
	all := make([]domain.Wallet, 0, len(r.f.wallets))
	for _, w := range r.f.wallets {
		all = append(all, w)
	}
	sort.Slice(all, func(i, j int) bool { // newest first, id breaks ties
		if !all[i].CreatedAt.Equal(all[j].CreatedAt) {
			return all[i].CreatedAt.After(all[j].CreatedAt)
		}
		return all[i].ID > all[j].ID
	})
	start := (page - 1) * pageSize
	if start >= len(all) {
		return nil, len(all), nil
	}
	return all[start:min(start+pageSize, len(all))], len(all), nil
}

func (r fakeWallets) Get(_ context.Context, id string) (domain.Wallet, error) {
	r.f.walletGets++
	w, ok := r.f.wallets[id]
	if !ok {
		return domain.Wallet{}, domain.ErrWalletNotFound
	}
	return w, nil
}

func (r fakeWallets) LockBalances(_ context.Context, ids ...string) (map[string]int64, error) {
	r.f.lockCalls = append(r.f.lockCalls, append([]string(nil), ids...))
	if r.f.failAt == "lock" {
		return nil, errInjected
	}
	balances := map[string]int64{}
	for _, id := range ids {
		if w, ok := r.f.wallets[id]; ok {
			balances[id] = w.Balance
		}
	}
	return balances, nil
}

func (r fakeWallets) AdjustBalance(_ context.Context, id string, delta int64) (int64, error) {
	if (delta < 0 && r.f.failAt == "debit") || (delta > 0 && r.f.failAt == "credit") {
		return 0, errInjected
	}
	w, ok := r.f.wallets[id]
	if !ok {
		return 0, domain.ErrWalletNotFound
	}
	if w.Balance+delta < 0 {
		return 0, domain.ErrInsufficientBalance // mirrors the DB CHECK constraint
	}
	w.Balance += delta
	r.f.wallets[id] = w
	return w.Balance, nil
}

// --- transactions (ledger) ---

type fakeTransactions struct{ f *fakeStore }

func (r fakeTransactions) Insert(_ context.Context, walletID string, t *domain.Transaction) error {
	if t.Type == r.f.failAt {
		return errInjected
	}
	if t.Type == domain.TxAddMoney {
		for _, row := range r.f.ledger {
			if row.tx.Type == domain.TxAddMoney && row.tx.ReferenceID == t.ReferenceID {
				return domain.ErrDuplicateGatewayRef // mirrors the unique index
			}
		}
	}
	r.f.nextTx++
	t.ID = int64(r.f.nextTx)
	t.CreatedAt = fixedNow
	r.f.ledger = append(r.f.ledger, ledgerRow{walletID, *t})
	return nil
}

func (r fakeTransactions) List(_ context.Context, f domain.TransactionFilter) ([]domain.Transaction, int, error) {
	var matched []domain.Transaction
	for _, row := range r.f.ledger {
		if row.walletID != f.WalletID || (f.Type != "" && row.tx.Type != f.Type) {
			continue
		}
		matched = append(matched, row.tx)
	}
	sort.Slice(matched, func(i, j int) bool { return matched[i].ID > matched[j].ID }) // newest first

	start := (f.Page - 1) * f.PageSize
	if start >= len(matched) {
		return nil, len(matched), nil
	}
	end := min(start+f.PageSize, len(matched))
	return matched[start:end], len(matched), nil
}

// --- transfers ---

type fakeTransfers struct{ f *fakeStore }

func (r fakeTransfers) Insert(_ context.Context, fromID, toID string, amount int64, note *string) (domain.Transfer, error) {
	if r.f.failAt == "transfer" {
		return domain.Transfer{}, errInjected
	}
	r.f.nextTransfer++
	t := domain.Transfer{
		ID:           fmt.Sprintf("10000000-0000-4000-8000-%012d", r.f.nextTransfer),
		FromWalletID: fromID,
		ToWalletID:   toID,
		Amount:       amount,
		Note:         note,
		Status:       domain.TransferCompleted,
		CreatedAt:    fixedNow,
	}
	r.f.transfers = append(r.f.transfers, t)
	return t, nil
}

// --- idempotency keys ---

type fakeIdempotency struct{ f *fakeStore }

func (r fakeIdempotency) Claim(_ context.Context, scope, key, hash string) (*domain.IdempotencyRecord, error) {
	if rec, ok := r.f.idem[scope+"|"+key]; ok {
		return &rec, nil
	}
	r.f.idem[scope+"|"+key] = domain.IdempotencyRecord{RequestHash: hash}
	return nil, nil
}

func (r fakeIdempotency) SaveResponse(_ context.Context, scope, key string, body []byte) error {
	rec := r.f.idem[scope+"|"+key]
	rec.ResponseBody = body
	r.f.idem[scope+"|"+key] = rec
	return nil
}
