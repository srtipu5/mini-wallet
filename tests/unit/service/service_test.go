package service_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/srtipu5/mini-wallet/internal/domain"

	"github.com/srtipu5/mini-wallet/internal/dto"
	"github.com/srtipu5/mini-wallet/internal/service"
)

var ctx = context.Background()

const unknownWalletID = "00000000-0000-4000-8000-999999999999"

func newTestService() (*service.Service, *fakeStore) {
	f := newFakeStore()
	return service.New(f), f
}

var phoneSeq int

func newWallet(t *testing.T, svc *service.Service, balance int64) domain.Wallet {
	t.Helper()
	phoneSeq++
	w, err := svc.CreateWallet(ctx, fmt.Sprintf("user_%d", phoneSeq), fmt.Sprintf("0171%07d", phoneSeq))
	if err != nil {
		t.Fatal(err)
	}
	if balance > 0 {
		if _, _, err := svc.AddMoney(ctx, fmt.Sprintf("seed-%d", phoneSeq), dto.AddMoneyInput{
			WalletID: w.ID, Amount: balance, GatewayRef: fmt.Sprintf("GW-seed-%d", phoneSeq),
		}); err != nil {
			t.Fatal(err)
		}
	}
	return w
}

func expectCode(t *testing.T, err error, code string) {
	t.Helper()
	e, ok := domain.AsError(err)
	if !ok || e.Code != code {
		t.Fatalf("error = %v, want code %s", err, code)
	}
}

func balance(t *testing.T, f *fakeStore, id string) int64 {
	t.Helper()
	return f.wallets[id].Balance
}

// ---------------------------------------------------------------- Ping

func TestPing(t *testing.T) {
	svc, f := newTestService()
	if err := svc.Ping(ctx); err != nil {
		t.Fatalf("Ping = %v, want nil", err)
	}
	f.pingErr = errors.New("db down")
	if err := svc.Ping(ctx); !errors.Is(err, f.pingErr) {
		t.Fatalf("Ping = %v, want the store error", err)
	}
}

// ---------------------------------------------------------------- Wallets

func TestCreateWallet(t *testing.T) {
	svc, _ := newTestService()
	w, err := svc.CreateWallet(ctx, "u_1", "01711000001")
	if err != nil {
		t.Fatal(err)
	}
	if w.UserID != "u_1" || w.Phone != "01711000001" || w.Balance != 0 || w.Currency != "BDT" {
		t.Fatalf("unexpected wallet: %+v", w)
	}

	_, err = svc.CreateWallet(ctx, "u_1", "01711000002")
	expectCode(t, err, domain.CodeDuplicateWallet)
}

func TestCreateWallet_ValidationNeverReachesStore(t *testing.T) {
	cases := map[string][2]string{
		"blank user":    {"   ", "01711000001"},
		"empty user":    {"", "01711000001"},
		"user too long": {strings.Repeat("x", domain.MaxUserIDLength+1), "01711000001"},
		"bad phone":     {"u_1", "12345"},
		"intl phone":    {"u_1", "+8801711000001"},
		"empty phone":   {"u_1", ""},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			svc, f := newTestService()
			_, err := svc.CreateWallet(ctx, c[0], c[1])
			expectCode(t, err, domain.CodeValidation)
			if len(f.wallets) != 0 {
				t.Fatal("invalid input must not create a wallet")
			}
		})
	}
}

func TestGetWallet(t *testing.T) {
	svc, f := newTestService()
	w := newWallet(t, svc, 5000)

	got, err := svc.GetWallet(ctx, w.ID)
	if err != nil || got.ID != w.ID || got.Balance != 5000 {
		t.Fatalf("GetWallet = %+v, %v", got, err)
	}

	// Uppercase ids are normalised.
	if _, err := svc.GetWallet(ctx, strings.ToUpper(w.ID)); err != nil {
		t.Fatalf("uppercase id: %v", err)
	}

	_, err = svc.GetWallet(ctx, unknownWalletID)
	expectCode(t, err, domain.CodeWalletNotFound)

	// A malformed id is "not found" and never reaches the repository.
	gets := f.walletGets
	_, err = svc.GetWallet(ctx, "not-a-uuid")
	expectCode(t, err, domain.CodeWalletNotFound)
	if f.walletGets != gets {
		t.Fatal("malformed id must not query the repository")
	}
}

// ---------------------------------------------------------------- Add money

func TestAddMoney(t *testing.T) {
	svc, f := newTestService()
	w := newWallet(t, svc, 0)

	res, replayed, err := svc.AddMoney(ctx, "k1", dto.AddMoneyInput{WalletID: w.ID, Amount: 50000, GatewayRef: "GW-1"})
	if err != nil || replayed {
		t.Fatalf("AddMoney = %v, replayed=%v", err, replayed)
	}
	if res.WalletID != w.ID || res.Amount != 50000 || res.BalanceAfter != 50000 || res.GatewayRef != "GW-1" || res.TransactionID == 0 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if balance(t, f, w.ID) != 50000 {
		t.Fatal("balance not credited")
	}

	entries := f.entriesOf(w.ID)
	if len(entries) != 1 {
		t.Fatalf("want 1 ledger entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Type != domain.TxAddMoney || e.Amount != 50000 || e.BalanceAfter != 50000 ||
		e.ReferenceID != "GW-1" || e.CounterpartyWalletID != nil {
		t.Fatalf("unexpected ledger entry: %+v", e)
	}
}

func TestAddMoney_AmountBoundaries(t *testing.T) {
	svc, f := newTestService()
	w := newWallet(t, svc, 0)
	for i, amount := range []int64{domain.MinAmount, domain.MaxAmount} {
		if _, _, err := svc.AddMoney(ctx, fmt.Sprint("k", i), dto.AddMoneyInput{
			WalletID: w.ID, Amount: amount, GatewayRef: fmt.Sprint("GW-", i),
		}); err != nil {
			t.Fatalf("amount %d: %v", amount, err)
		}
	}
	if balance(t, f, w.ID) != domain.MinAmount+domain.MaxAmount {
		t.Fatal("boundary amounts must be credited")
	}
}

func TestAddMoney_ValidationNeverOpensTransaction(t *testing.T) {
	cases := []struct {
		name string
		in   func(walletID string) dto.AddMoneyInput
		code string
	}{
		{"malformed wallet id", func(string) dto.AddMoneyInput {
			return dto.AddMoneyInput{WalletID: "garbage", Amount: 1000, GatewayRef: "GW"}
		}, domain.CodeWalletNotFound},
		{"amount below min", func(id string) dto.AddMoneyInput {
			return dto.AddMoneyInput{WalletID: id, Amount: domain.MinAmount - 1, GatewayRef: "GW"}
		}, domain.CodeInvalidAmount},
		{"amount above max", func(id string) dto.AddMoneyInput {
			return dto.AddMoneyInput{WalletID: id, Amount: domain.MaxAmount + 1, GatewayRef: "GW"}
		}, domain.CodeInvalidAmount},
		{"zero amount", func(id string) dto.AddMoneyInput {
			return dto.AddMoneyInput{WalletID: id, Amount: 0, GatewayRef: "GW"}
		}, domain.CodeInvalidAmount},
		{"negative amount", func(id string) dto.AddMoneyInput {
			return dto.AddMoneyInput{WalletID: id, Amount: -500, GatewayRef: "GW"}
		}, domain.CodeInvalidAmount},
		{"missing gateway_ref", func(id string) dto.AddMoneyInput {
			return dto.AddMoneyInput{WalletID: id, Amount: 1000}
		}, domain.CodeValidation},
		{"blank gateway_ref", func(id string) dto.AddMoneyInput {
			return dto.AddMoneyInput{WalletID: id, Amount: 1000, GatewayRef: "   "}
		}, domain.CodeValidation},
		{"gateway_ref too long", func(id string) dto.AddMoneyInput {
			return dto.AddMoneyInput{WalletID: id, Amount: 1000, GatewayRef: strings.Repeat("g", domain.MaxGatewayRefLength+1)}
		}, domain.CodeValidation},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, f := newTestService()
			w := newWallet(t, svc, 0)
			_, _, err := svc.AddMoney(ctx, "k", c.in(w.ID))
			expectCode(t, err, c.code)
			if f.txBegun != 0 || len(f.idem) != 0 {
				t.Fatal("invalid input must be rejected before the transaction and the key claim")
			}
		})
	}
}

func TestAddMoney_UnknownWallet(t *testing.T) {
	svc, f := newTestService()
	_, _, err := svc.AddMoney(ctx, "k", dto.AddMoneyInput{WalletID: unknownWalletID, Amount: 1000, GatewayRef: "GW"})
	expectCode(t, err, domain.CodeWalletNotFound)
	if len(f.idem) != 0 || len(f.ledger) != 0 {
		t.Fatal("failed add-money must roll back the key and the ledger")
	}
}

func TestAddMoney_DuplicateGatewayRefRollsBack(t *testing.T) {
	svc, f := newTestService()
	a := newWallet(t, svc, 0)
	b := newWallet(t, svc, 0)

	if _, _, err := svc.AddMoney(ctx, "k1", dto.AddMoneyInput{WalletID: a.ID, Amount: 1000, GatewayRef: "GW-1"}); err != nil {
		t.Fatal(err)
	}
	// Same payment again: under a new key, and into another wallet.
	_, _, err := svc.AddMoney(ctx, "k2", dto.AddMoneyInput{WalletID: a.ID, Amount: 1000, GatewayRef: "GW-1"})
	expectCode(t, err, domain.CodeDuplicateGatewayRef)
	_, _, err = svc.AddMoney(ctx, "k3", dto.AddMoneyInput{WalletID: b.ID, Amount: 1000, GatewayRef: "GW-1"})
	expectCode(t, err, domain.CodeDuplicateGatewayRef)

	// The balance change made before the duplicate was detected is rolled back.
	if balance(t, f, a.ID) != 1000 || balance(t, f, b.ID) != 0 {
		t.Fatalf("balances a=%d b=%d, want 1000 and 0", balance(t, f, a.ID), balance(t, f, b.ID))
	}
}

func TestAddMoney_Idempotency(t *testing.T) {
	svc, f := newTestService()
	w := newWallet(t, svc, 0)
	in := dto.AddMoneyInput{WalletID: w.ID, Amount: 1000, GatewayRef: "GW-1"}

	first, replayed, err := svc.AddMoney(ctx, "same", in)
	if err != nil || replayed {
		t.Fatalf("first call: %v replayed=%v", err, replayed)
	}

	// Same key + same request (wallet id in another case) -> stored response.
	upper := in
	upper.WalletID = strings.ToUpper(in.WalletID)
	second, replayed, err := svc.AddMoney(ctx, "same", upper)
	if err != nil || !replayed || second != first {
		t.Fatalf("replay = %+v, replayed=%v, err=%v; want %+v", second, replayed, err, first)
	}

	// Same key + different request -> rejected.
	changed := in
	changed.Amount = 2000
	_, _, err = svc.AddMoney(ctx, "same", changed)
	expectCode(t, err, domain.CodeIdempotencyKeyReused)

	if balance(t, f, w.ID) != 1000 || len(f.entriesOf(w.ID)) != 1 {
		t.Fatal("add-money must execute exactly once")
	}
}

func TestAddMoney_FailedRequestReleasesKey(t *testing.T) {
	svc, f := newTestService()
	w := newWallet(t, svc, 0)
	if _, _, err := svc.AddMoney(ctx, "k1", dto.AddMoneyInput{WalletID: w.ID, Amount: 1000, GatewayRef: "GW-1"}); err != nil {
		t.Fatal(err)
	}

	// Fails (duplicate gateway_ref), so key k2 is not stored ...
	_, _, err := svc.AddMoney(ctx, "k2", dto.AddMoneyInput{WalletID: w.ID, Amount: 1000, GatewayRef: "GW-1"})
	expectCode(t, err, domain.CodeDuplicateGatewayRef)

	// ... and can be used for a different, valid request.
	if _, replayed, err := svc.AddMoney(ctx, "k2", dto.AddMoneyInput{WalletID: w.ID, Amount: 500, GatewayRef: "GW-2"}); err != nil || replayed {
		t.Fatalf("retry with released key: %v replayed=%v", err, replayed)
	}
	if balance(t, f, w.ID) != 1500 {
		t.Fatalf("balance = %d, want 1500", balance(t, f, w.ID))
	}
}

// ---------------------------------------------------------------- Transfers

func TestTransfer(t *testing.T) {
	svc, f := newTestService()
	a := newWallet(t, svc, 50000)
	b := newWallet(t, svc, 0)
	note := "Lunch"

	tr, replayed, err := svc.Transfer(ctx, "t1", dto.TransferInput{FromWalletID: a.ID, ToWalletID: b.ID, Amount: 10000, Note: &note})
	if err != nil || replayed {
		t.Fatalf("Transfer = %v replayed=%v", err, replayed)
	}
	if tr.ID == "" || tr.FromWalletID != a.ID || tr.ToWalletID != b.ID || tr.Amount != 10000 ||
		tr.Status != domain.TransferCompleted || tr.Note == nil || *tr.Note != "Lunch" {
		t.Fatalf("unexpected transfer: %+v", tr)
	}
	if balance(t, f, a.ID) != 40000 || balance(t, f, b.ID) != 10000 {
		t.Fatalf("balances a=%d b=%d", balance(t, f, a.ID), balance(t, f, b.ID))
	}

	// One ledger entry per side, pointing at each other and at the transfer.
	out := f.entriesOf(a.ID)[1] // [0] is the seed ADD_MONEY
	in := f.entriesOf(b.ID)[0]
	if out.Type != domain.TxTransferOut || out.Amount != 10000 || out.BalanceAfter != 40000 ||
		*out.CounterpartyWalletID != b.ID || out.ReferenceID != tr.ID {
		t.Fatalf("unexpected TRANSFER_OUT entry: %+v", out)
	}
	if in.Type != domain.TxTransferIn || in.Amount != 10000 || in.BalanceAfter != 10000 ||
		*in.CounterpartyWalletID != a.ID || in.ReferenceID != tr.ID {
		t.Fatalf("unexpected TRANSFER_IN entry: %+v", in)
	}
}

func TestTransfer_WholeBalance(t *testing.T) {
	svc, f := newTestService()
	a := newWallet(t, svc, 1000)
	b := newWallet(t, svc, 0)
	if _, _, err := svc.Transfer(ctx, "t1", dto.TransferInput{FromWalletID: a.ID, ToWalletID: b.ID, Amount: 1000}); err != nil {
		t.Fatal(err)
	}
	if balance(t, f, a.ID) != 0 || balance(t, f, b.ID) != 1000 {
		t.Fatal("sending the whole balance must leave exactly 0")
	}
}

func TestTransfer_InsufficientBalance(t *testing.T) {
	svc, f := newTestService()
	a := newWallet(t, svc, 1000)
	b := newWallet(t, svc, 0)

	_, _, err := svc.Transfer(ctx, "t1", dto.TransferInput{FromWalletID: a.ID, ToWalletID: b.ID, Amount: 1001})
	expectCode(t, err, domain.CodeInsufficientBalance)
	if balance(t, f, a.ID) != 1000 || balance(t, f, b.ID) != 0 || len(f.transfers) != 0 {
		t.Fatal("a rejected transfer must change nothing")
	}
	if _, used := f.idem["transfer|t1"]; used {
		t.Fatal("a failed transfer must not keep its idempotency key")
	}
}

// Both wallets must be locked together in one call, so the repository can
// lock them in id order (ORDER BY id ... FOR UPDATE). Locking them one by one
// in transfer direction would deadlock A->B against B->A. The real ordering is
// proven against Postgres by the integration deadlock tests.
func TestTransfer_LocksBothWalletsInOneCall(t *testing.T) {
	svc, f := newTestService()
	a := newWallet(t, svc, 5000)
	b := newWallet(t, svc, 5000)

	for i, dir := range [][2]string{{a.ID, b.ID}, {b.ID, a.ID}} {
		f.lockCalls = nil
		if _, _, err := svc.Transfer(ctx, fmt.Sprint("t", i), dto.TransferInput{FromWalletID: dir[0], ToWalletID: dir[1], Amount: 100}); err != nil {
			t.Fatal(err)
		}
		if len(f.lockCalls) != 1 || len(f.lockCalls[0]) != 2 {
			t.Fatalf("want one LockBalances call with both wallets, got %v", f.lockCalls)
		}
		got := map[string]bool{f.lockCalls[0][0]: true, f.lockCalls[0][1]: true}
		if !got[a.ID] || !got[b.ID] {
			t.Fatalf("locked %v, want both %s and %s", f.lockCalls[0], a.ID, b.ID)
		}
	}
}

func TestTransfer_ValidationNeverOpensTransaction(t *testing.T) {
	long := strings.Repeat("n", domain.MaxNoteLength+1)
	cases := []struct {
		name string
		in   func(a, b string) dto.TransferInput
		code string
		msg  string
	}{
		{"missing from", func(_, b string) dto.TransferInput {
			return dto.TransferInput{ToWalletID: b, Amount: 1000}
		},
			domain.CodeValidation, ""},
		{"missing to", func(a, _ string) dto.TransferInput {
			return dto.TransferInput{FromWalletID: a, Amount: 1000}
		},
			domain.CodeValidation, ""},
		{"same wallet", func(a, _ string) dto.TransferInput {
			return dto.TransferInput{FromWalletID: a, ToWalletID: a, Amount: 1000}
		},
			domain.CodeSameWalletTransfer, ""},
		{"same wallet, different case", func(a, _ string) dto.TransferInput {
			return dto.TransferInput{FromWalletID: a, ToWalletID: strings.ToUpper(a), Amount: 1000}
		}, domain.CodeSameWalletTransfer, ""},
		{"malformed sender", func(_, b string) dto.TransferInput {
			return dto.TransferInput{FromWalletID: "x", ToWalletID: b, Amount: 1000}
		},
			domain.CodeWalletNotFound, "sender wallet not found"},
		{"malformed receiver", func(a, _ string) dto.TransferInput {
			return dto.TransferInput{FromWalletID: a, ToWalletID: "x", Amount: 1000}
		},
			domain.CodeWalletNotFound, "receiver wallet not found"},
		{"amount below min", func(a, b string) dto.TransferInput {
			return dto.TransferInput{FromWalletID: a, ToWalletID: b, Amount: 99}
		},
			domain.CodeInvalidAmount, ""},
		{"amount above max", func(a, b string) dto.TransferInput {
			return dto.TransferInput{FromWalletID: a, ToWalletID: b, Amount: domain.MaxAmount + 1}
		}, domain.CodeInvalidAmount, ""},
		{"negative amount", func(a, b string) dto.TransferInput {
			return dto.TransferInput{FromWalletID: a, ToWalletID: b, Amount: -100}
		},
			domain.CodeInvalidAmount, ""},
		{"note too long", func(a, b string) dto.TransferInput {
			return dto.TransferInput{FromWalletID: a, ToWalletID: b, Amount: 1000, Note: &long}
		}, domain.CodeValidation, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, f := newTestService()
			a := newWallet(t, svc, 5000)
			b := newWallet(t, svc, 0)
			txBefore := f.txBegun

			_, _, err := svc.Transfer(ctx, "k", c.in(a.ID, b.ID))
			expectCode(t, err, c.code)
			if e, _ := domain.AsError(err); c.msg != "" && e.Message != c.msg {
				t.Fatalf("message = %q, want %q", e.Message, c.msg)
			}
			if f.txBegun != txBefore {
				t.Fatal("invalid input must be rejected before opening a transaction")
			}
		})
	}
}

func TestTransfer_UnknownWallets(t *testing.T) {
	svc, f := newTestService()
	a := newWallet(t, svc, 5000)

	_, _, err := svc.Transfer(ctx, "k1", dto.TransferInput{FromWalletID: unknownWalletID, ToWalletID: a.ID, Amount: 1000})
	expectCode(t, err, domain.CodeWalletNotFound)
	if e, _ := domain.AsError(err); e.Message != "sender wallet not found" {
		t.Fatalf("message = %q", e.Message)
	}

	_, _, err = svc.Transfer(ctx, "k2", dto.TransferInput{FromWalletID: a.ID, ToWalletID: unknownWalletID, Amount: 1000})
	expectCode(t, err, domain.CodeWalletNotFound)
	if e, _ := domain.AsError(err); e.Message != "receiver wallet not found" {
		t.Fatalf("message = %q", e.Message)
	}

	if balance(t, f, a.ID) != 5000 || len(f.transfers) != 0 {
		t.Fatal("a transfer to or from an unknown wallet must change nothing")
	}
}

func TestTransfer_NoteHandling(t *testing.T) {
	svc, _ := newTestService()
	a := newWallet(t, svc, 50000)
	b := newWallet(t, svc, 0)

	blank := "   "
	tr, _, err := svc.Transfer(ctx, "k1", dto.TransferInput{FromWalletID: a.ID, ToWalletID: b.ID, Amount: 100, Note: &blank})
	if err != nil || tr.Note != nil {
		t.Fatalf("a blank note must be stored as no note: note=%v err=%v", tr.Note, err)
	}

	tr, _, err = svc.Transfer(ctx, "k2", dto.TransferInput{FromWalletID: a.ID, ToWalletID: b.ID, Amount: 100})
	if err != nil || tr.Note != nil {
		t.Fatalf("no note: note=%v err=%v", tr.Note, err)
	}

	// The limit counts characters, not bytes (Bangla letters are 3 bytes each).
	bangla := strings.Repeat("ট", domain.MaxNoteLength)
	if _, _, err := svc.Transfer(ctx, "k3", dto.TransferInput{FromWalletID: a.ID, ToWalletID: b.ID, Amount: 100, Note: &bangla}); err != nil {
		t.Fatalf("255-character note must be accepted: %v", err)
	}
}

func TestTransfer_CaseInsensitiveIDs(t *testing.T) {
	svc, f := newTestService()
	a := newWallet(t, svc, 5000)
	b := newWallet(t, svc, 0)

	tr, _, err := svc.Transfer(ctx, "k", dto.TransferInput{
		FromWalletID: strings.ToUpper(a.ID), ToWalletID: strings.ToUpper(b.ID), Amount: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if tr.FromWalletID != a.ID || tr.ToWalletID != b.ID || balance(t, f, b.ID) != 1000 {
		t.Fatalf("ids must be normalised to lowercase: %+v", tr)
	}
}

// Debit and credit must be atomic: whichever step fails, nothing is left
// behind (no half-moved money, no transfer record, no ledger entry, no key).
func TestTransfer_AtomicRollbackAtEveryStep(t *testing.T) {
	steps := []string{"lock", "debit", "credit", "transfer", domain.TxTransferOut, domain.TxTransferIn}
	for _, step := range steps {
		t.Run(step, func(t *testing.T) {
			svc, f := newTestService()
			a := newWallet(t, svc, 5000)
			b := newWallet(t, svc, 0)
			ledgerBefore, keysBefore := len(f.ledger), len(f.idem)

			f.failAt = step
			_, _, err := svc.Transfer(ctx, "k", dto.TransferInput{FromWalletID: a.ID, ToWalletID: b.ID, Amount: 1000})
			if !errors.Is(err, errInjected) {
				t.Fatalf("err = %v, want the injected failure", err)
			}
			if balance(t, f, a.ID) != 5000 || balance(t, f, b.ID) != 0 {
				t.Fatalf("balances a=%d b=%d; must be rolled back to 5000 and 0", balance(t, f, a.ID), balance(t, f, b.ID))
			}
			if len(f.transfers) != 0 || len(f.ledger) != ledgerBefore || len(f.idem) != keysBefore {
				t.Fatal("transfer record, ledger entries and idempotency key must be rolled back")
			}

			// Nothing was used up: the same key works once the fault is gone.
			f.failAt = ""
			if _, replayed, err := svc.Transfer(ctx, "k", dto.TransferInput{FromWalletID: a.ID, ToWalletID: b.ID, Amount: 1000}); err != nil || replayed {
				t.Fatalf("retry after rollback: err=%v replayed=%v", err, replayed)
			}
		})
	}
}

func TestAddMoney_AtomicRollbackAtEveryStep(t *testing.T) {
	for _, step := range []string{"credit", domain.TxAddMoney} {
		t.Run(step, func(t *testing.T) {
			svc, f := newTestService()
			w := newWallet(t, svc, 0)

			f.failAt = step
			_, _, err := svc.AddMoney(ctx, "k", dto.AddMoneyInput{WalletID: w.ID, Amount: 1000, GatewayRef: "GW-1"})
			if !errors.Is(err, errInjected) {
				t.Fatalf("err = %v, want the injected failure", err)
			}
			if balance(t, f, w.ID) != 0 || len(f.entriesOf(w.ID)) != 0 || len(f.idem) != 0 {
				t.Fatal("balance, ledger entry and idempotency key must be rolled back")
			}
		})
	}
}

// When neither wallet exists, the sender is reported.
func TestTransfer_BothWalletsUnknown(t *testing.T) {
	svc, f := newTestService()
	other := "00000000-0000-4000-8000-888888888888"
	_, _, err := svc.Transfer(ctx, "k", dto.TransferInput{FromWalletID: unknownWalletID, ToWalletID: other, Amount: 1000})
	expectCode(t, err, domain.CodeWalletNotFound)
	if e, _ := domain.AsError(err); e.Message != "sender wallet not found" {
		t.Fatalf("message = %q, want sender wallet not found", e.Message)
	}
	if len(f.idem) != 0 {
		t.Fatal("a failed transfer must not keep its key")
	}
}

func TestTransfer_Idempotency(t *testing.T) {
	svc, f := newTestService()
	a := newWallet(t, svc, 5000)
	b := newWallet(t, svc, 0)
	in := dto.TransferInput{FromWalletID: a.ID, ToWalletID: b.ID, Amount: 1000}

	first, replayed, err := svc.Transfer(ctx, "same", in)
	if err != nil || replayed {
		t.Fatalf("first: %v replayed=%v", err, replayed)
	}
	second, replayed, err := svc.Transfer(ctx, "same", in)
	if err != nil || !replayed || second.ID != first.ID {
		t.Fatalf("replay: %+v replayed=%v err=%v", second, replayed, err)
	}

	// Changing any field (here: adding a note) is a different request.
	note := "hi"
	withNote := in
	withNote.Note = &note
	_, _, err = svc.Transfer(ctx, "same", withNote)
	expectCode(t, err, domain.CodeIdempotencyKeyReused)

	if balance(t, f, a.ID) != 4000 || balance(t, f, b.ID) != 1000 || len(f.transfers) != 1 {
		t.Fatal("transfer must execute exactly once")
	}
}

func TestTransfer_KeysAreScopedPerOperation(t *testing.T) {
	svc, f := newTestService()
	a := newWallet(t, svc, 0)
	b := newWallet(t, svc, 0)

	if _, _, err := svc.AddMoney(ctx, "shared", dto.AddMoneyInput{WalletID: a.ID, Amount: 5000, GatewayRef: "GW-1"}); err != nil {
		t.Fatal(err)
	}
	if _, replayed, err := svc.Transfer(ctx, "shared", dto.TransferInput{FromWalletID: a.ID, ToWalletID: b.ID, Amount: 1000}); err != nil || replayed {
		t.Fatalf("the same key in another operation must run normally: %v replayed=%v", err, replayed)
	}
	if balance(t, f, b.ID) != 1000 {
		t.Fatal("transfer did not run")
	}
}

// ---------------------------------------------------------------- History

func TestListTransactions(t *testing.T) {
	svc, _ := newTestService()
	a := newWallet(t, svc, 0)
	b := newWallet(t, svc, 0)
	for i := 0; i < 4; i++ {
		if _, _, err := svc.AddMoney(ctx, fmt.Sprint("k", i), dto.AddMoneyInput{WalletID: a.ID, Amount: 1000, GatewayRef: fmt.Sprint("GW-", i)}); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := svc.Transfer(ctx, "t", dto.TransferInput{FromWalletID: a.ID, ToWalletID: b.ID, Amount: 500}); err != nil {
		t.Fatal(err)
	}

	// 5 entries, page size 2 -> 3 pages; newest (the transfer) first.
	page, err := svc.ListTransactions(ctx, domain.TransactionFilter{WalletID: strings.ToUpper(a.ID), Page: 1, PageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Data) != 2 || page.Data[0].Type != domain.TxTransferOut || page.Data[0].ID < page.Data[1].ID {
		t.Fatalf("page 1 must be newest first: %+v", page.Data)
	}
	if page.Pagination != (dto.Pagination{Page: 1, PageSize: 2, TotalItems: 5, TotalPages: 3}) {
		t.Fatalf("pagination = %+v", page.Pagination)
	}

	last, _ := svc.ListTransactions(ctx, domain.TransactionFilter{WalletID: a.ID, Page: 3, PageSize: 2})
	if len(last.Data) != 1 {
		t.Fatalf("last page has %d items, want 1", len(last.Data))
	}

	onlyAdds, _ := svc.ListTransactions(ctx, domain.TransactionFilter{WalletID: a.ID, Type: domain.TxAddMoney, Page: 1, PageSize: 20})
	if onlyAdds.Pagination.TotalItems != 4 {
		t.Fatalf("type filter: total = %d, want 4", onlyAdds.Pagination.TotalItems)
	}
}

func TestListTransactions_EmptyIsArrayNotNull(t *testing.T) {
	svc, _ := newTestService()
	w := newWallet(t, svc, 0)

	for _, p := range []int{1, 99} { // a new wallet, and a page past the end
		page, err := svc.ListTransactions(ctx, domain.TransactionFilter{WalletID: w.ID, Page: p, PageSize: 20})
		if err != nil {
			t.Fatal(err)
		}
		if page.Data == nil || len(page.Data) != 0 || page.Pagination.TotalPages != 0 {
			t.Fatalf("page %d: want empty non-nil list and 0 pages, got %+v", p, page)
		}
	}
}

func TestListTransactions_UnknownWallet(t *testing.T) {
	svc, _ := newTestService()
	for _, id := range []string{unknownWalletID, "garbage"} {
		_, err := svc.ListTransactions(ctx, domain.TransactionFilter{WalletID: id, Page: 1, PageSize: 20})
		expectCode(t, err, domain.CodeWalletNotFound)
	}
}
