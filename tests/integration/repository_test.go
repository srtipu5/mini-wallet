package tests

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/srtipu5/mini-wallet/internal/domain"
	"github.com/srtipu5/mini-wallet/internal/repository/postgres"
	"github.com/srtipu5/mini-wallet/internal/service"
)

// These tests call the repositories directly to check database-level
// guarantees the service's validation normally hides.

// inTx commits if fn succeeds and rolls back otherwise.
func inTx(ctx context.Context, store service.Store, fn func(r service.Repositories) error) error {
	tx, err := store.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := fn(tx.Repos()); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func TestRepository_LockBalances(t *testing.T) {
	resetDB(t)
	a := createWallet(t, "u_a")
	b := createWallet(t, "u_b")
	addMoney(t, a, 7000)
	missing := "00000000-0000-0000-0000-000000000000"

	err := inTx(context.Background(), postgres.NewStore(db), func(r service.Repositories) error {
		balances, err := r.Wallets.LockBalances(context.Background(), b, missing, a)
		if err != nil {
			return err
		}
		if len(balances) != 2 || balances[a] != 7000 || balances[b] != 0 {
			t.Errorf("balances = %v; want %s:7000 and %s:0, unknown wallet left out", balances, a, b)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// While one transaction holds the lock, another one that wants the same
// wallet must wait. This is what stops double-spending.
func TestRepository_LockBlocksOtherTransactions(t *testing.T) {
	resetDB(t)
	a := createWallet(t, "u_a")
	store := postgres.NewStore(db)
	ctx := context.Background()

	locked, release := make(chan struct{}), make(chan struct{})
	finished := make(chan struct{})
	var firstErr error
	// Always release the first transaction, even if the test fails early;
	// otherwise its open transaction would block the next test's resetDB.
	releaseLock := sync.OnceFunc(func() { close(release) })
	t.Cleanup(func() { releaseLock(); <-finished })
	go func() {
		defer close(finished)
		firstErr = inTx(ctx, store, func(r service.Repositories) error {
			if _, err := r.Wallets.LockBalances(ctx, a); err != nil {
				return err
			}
			close(locked)
			<-release // hold the lock until the test says so
			return nil
		})
	}()
	select {
	case <-locked:
	case <-finished:
		t.Fatalf("first transaction could not take the lock: %v", firstErr)
	}

	// A second transaction cannot get the lock while the first holds it.
	waitCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	err := inTx(waitCtx, store, func(r service.Repositories) error {
		_, err := r.Wallets.LockBalances(waitCtx, a)
		return err
	})
	cancel()
	if err == nil {
		t.Fatal("second transaction got the lock while the first still held it")
	}

	// Once the first commits, the lock is free again.
	releaseLock()
	err = inTx(ctx, store, func(r service.Repositories) error {
		_, err := r.Wallets.LockBalances(ctx, a)
		return err
	})
	if err != nil {
		t.Fatalf("lock not released after commit: %v", err)
	}
}

// The CHECK (balance >= 0) constraint is the last line of defence: even if
// the service had a bug and skipped its balance check, the database refuses.
func TestRepository_BalanceCannotGoNegative(t *testing.T) {
	resetDB(t)
	a := createWallet(t, "u_a")
	addMoney(t, a, 1000)

	err := inTx(context.Background(), postgres.NewStore(db), func(r service.Repositories) error {
		_, err := r.Wallets.AdjustBalance(context.Background(), a, -1001)
		return err
	})
	if !errors.Is(err, domain.ErrInsufficientBalance) {
		t.Fatalf("err = %v, want ErrInsufficientBalance from the CHECK constraint", err)
	}
	if balanceOf(t, a) != 1000 {
		t.Fatal("balance must be unchanged")
	}
}
