package tests

import (
	"fmt"
	"math/rand"
	"testing"
)

// history returns every ledger entry of a wallet (newest first).
func history(t *testing.T, walletID string) []map[string]any {
	t.Helper()
	r := call(t, "GET", historyPath(walletID, "?page_size=100"), nil, "")
	expectStatus(t, r, 200)
	var out []map[string]any
	for _, e := range r.JSON(t)["data"].([]any) {
		out = append(out, e.(map[string]any))
	}
	return out
}

// checkLedger verifies that a wallet's history explains its balance exactly:
// the newest balance_after equals the balance, and every entry moves the
// running balance by its amount.
func checkLedger(t *testing.T, walletID string) {
	t.Helper()
	entries := history(t, walletID)
	balance := balanceOf(t, walletID)
	if len(entries) == 0 {
		if balance != 0 {
			t.Fatalf("wallet %s: balance %d with no history", walletID, balance)
		}
		return
	}
	if int64(entries[0]["balance_after"].(float64)) != balance {
		t.Fatalf("wallet %s: newest balance_after %v != balance %d", walletID, entries[0]["balance_after"], balance)
	}

	var running int64
	for i := len(entries) - 1; i >= 0; i-- { // oldest first
		e := entries[i]
		amount := int64(e["amount"].(float64))
		switch e["type"] {
		case "ADD_MONEY", "TRANSFER_IN":
			running += amount
		case "TRANSFER_OUT":
			running -= amount
		}
		if running < 0 || int64(e["balance_after"].(float64)) != running {
			t.Fatalf("wallet %s: entry %v breaks the running balance (expected %d)", walletID, e, running)
		}
	}
}

func TestLedger_HistoryExplainsBalance(t *testing.T) {
	resetDB(t)
	a := createWallet(t, "u_a")
	b := createWallet(t, "u_b")
	c := createWallet(t, "u_c")
	addMoney(t, a, 10000)
	addMoney(t, b, 5000)
	expectStatus(t, transfer(t, a, b, 3000, "t1"), 201)
	expectStatus(t, transfer(t, b, c, 7000, "t2"), 201)
	expectStatus(t, transfer(t, c, a, 2500, "t3"), 201)
	expectError(t, transfer(t, a, c, 999999, "t4"), 422, "INSUFFICIENT_BALANCE") // leaves no trace

	for _, w := range []string{a, b, c} {
		checkLedger(t, w)
	}
	if balanceOf(t, a)+balanceOf(t, b)+balanceOf(t, c) != 15000 {
		t.Fatal("transfers must conserve money")
	}
}

// Add-money and transfers between several wallets, all at the same time.
// Afterwards: no negative balance, total money == total successfully added,
// and every wallet's history still explains its balance.
func TestConcurrent_MixedOperationsConserveMoney(t *testing.T) {
	resetDB(t)
	wallets := []string{createWallet(t, "u_1"), createWallet(t, "u_2"), createWallet(t, "u_3"), createWallet(t, "u_4")}
	for _, w := range wallets {
		addMoney(t, w, 5000)
	}

	const n = 80
	added := make([]int64, n)
	runConcurrently(n, func(i int) {
		rng := rand.New(rand.NewSource(int64(i)))
		from := wallets[rng.Intn(len(wallets))]
		if i%4 == 0 {
			r := call(t, "POST", addMoneyPath(from),
				fmt.Sprintf(`{"amount":1000,"gateway_ref":"GW-mix-%d"}`, i), fmt.Sprintf("add-%d", i))
			if r.Status == 201 {
				added[i] = 1000
			}
			return
		}
		to := wallets[(rng.Intn(len(wallets)-1)+1+indexOf(wallets, from))%len(wallets)]
		r := transfer(t, from, to, int64(100+rng.Intn(3000)), fmt.Sprintf("mix-%d", i))
		if r.Status != 201 && r.Status != 422 {
			t.Errorf("transfer %d: unexpected status %d: %s", i, r.Status, r.Body)
		}
	})

	var want, got int64 = 4 * 5000, 0
	for _, a := range added {
		want += a
	}
	for _, w := range wallets {
		b := balanceOf(t, w)
		if b < 0 {
			t.Fatalf("negative balance %d", b)
		}
		got += b
		checkLedger(t, w)
	}
	if got != want {
		t.Fatalf("total money = %d, want %d", got, want)
	}
}

func indexOf(list []string, s string) int {
	for i, v := range list {
		if v == s {
			return i
		}
	}
	return -1
}
