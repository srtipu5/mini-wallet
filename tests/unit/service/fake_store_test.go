package service_test

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/srtipu5/mini-wallet/internal/domain"

	"github.com/srtipu5/mini-wallet/internal/service"
)

// fakeStore is an in-memory implementation of the service.Store interface, so the business
// logic can be tested without Postgres. WithinTx snapshots the state and
// restores it if fn fails, which mimics a database rollback.
type fakeStore struct {
	wallets   map[string]domain.Wallet
	ledger    []ledgerRow
	transfers []domain.Transfer
	idem      map[string]domain.IdempotencyRecord // "scope|key"

	nextWallet, nextTx, nextTransfer int

	// Instrumentation for assertions.
	txBegun    int        // number of WithinTx calls
	lockCalls  [][]string // ids passed to each LockBalances call
	walletGets int        // number of Wallets.Get calls

	// Fault injection.
	pingErr        error
	failInsertType string // Transactions.Insert fails for this entry type
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

func (f *fakeStore) WithinTx(_ context.Context, fn func(r service.Repositories) error) error {
	f.txBegun++
	snapshot := f.snapshot()
	if err := fn(f.Repos()); err != nil {
		f.restore(snapshot)
		return err
	}
	return nil
}

func (f *fakeStore) Ping(context.Context) error { return f.pingErr }

type fakeState struct {
	wallets   map[string]domain.Wallet
	ledger    []ledgerRow
	transfers []domain.Transfer
	idem      map[string]domain.IdempotencyRecord
	nextTx    int
	nextTrf   int
}

func (f *fakeStore) snapshot() fakeState {
	s := fakeState{
		wallets:   map[string]domain.Wallet{},
		ledger:    append([]ledgerRow(nil), f.ledger...),
		transfers: append([]domain.Transfer(nil), f.transfers...),
		idem:      map[string]domain.IdempotencyRecord{},
		nextTx:    f.nextTx,
		nextTrf:   f.nextTransfer,
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
	f.nextTx, f.nextTransfer = s.nextTx, s.nextTrf
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
	balances := map[string]int64{}
	for _, id := range ids {
		if w, ok := r.f.wallets[id]; ok {
			balances[id] = w.Balance
		}
	}
	return balances, nil
}

func (r fakeWallets) ChangeBalance(_ context.Context, id string, delta int64) (int64, error) {
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
	if t.Type == r.f.failInsertType {
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
