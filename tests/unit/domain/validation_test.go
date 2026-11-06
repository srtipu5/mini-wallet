package domain_test

import (
	"testing"

	"github.com/srtipu5/mini-wallet/internal/domain"
)

func TestValidateAmount(t *testing.T) {
	const (
		tooSmall = "Amount must be at least 100 poisha (1 BDT)"
		tooLarge = "Amount must not exceed 2500000 poisha (25000 BDT)"
	)
	cases := []struct {
		amount int64
		msg    string // "" = valid
	}{
		{-100, tooSmall},
		{0, tooSmall},
		{99, tooSmall},
		{100, ""},       // minimum
		{2_500_000, ""}, // maximum
		{2_500_001, tooLarge},
		{9_000_000_000, tooLarge},
	}
	for _, c := range cases {
		err := domain.ValidateAmount(c.amount)
		if c.msg == "" {
			if err != nil {
				t.Errorf("ValidateAmount(%d): unexpected error %v", c.amount, err)
			}
			continue
		}
		e, ok := domain.AsError(err)
		if !ok || e.Code != domain.CodeInvalidAmount || e.Message != c.msg {
			t.Errorf("ValidateAmount(%d) = %v; want INVALID_AMOUNT %q", c.amount, err, c.msg)
		}
	}
}

func TestValidatePhone(t *testing.T) {
	valid := []string{"01711000001", "01311111111", "01999999999"}
	invalid := []string{"", "0171100000", "017110000012", "01211000001", "+8801711000001", "0171100000a"}

	for _, p := range valid {
		if err := domain.ValidatePhone(p); err != nil {
			t.Errorf("ValidatePhone(%q) should pass: %v", p, err)
		}
	}
	for _, p := range invalid {
		if err := domain.ValidatePhone(p); err == nil {
			t.Errorf("ValidatePhone(%q) should fail", p)
		}
	}
}

func TestIsValidWalletID(t *testing.T) {
	if !domain.IsValidWalletID("3f2b8c4e-1a2b-4c3d-8e9f-0a1b2c3d4e5f") {
		t.Error("valid uuid rejected")
	}
	for _, id := range []string{"", "abc", "3f2b8c4e1a2b4c3d8e9f0a1b2c3d4e5f", "' OR 1=1 --"} {
		if domain.IsValidWalletID(id) {
			t.Errorf("IsValidWalletID(%q) should be false", id)
		}
	}
}

func TestValidateText(t *testing.T) {
	cases := []struct {
		value string
		ok    bool
	}{
		{"abc", true},
		{"12345", true},    // exactly max
		{"ঢাকাবাস", false}, // 7 characters > 5 (counted as characters, not bytes)
		{"ঢাকা", true},     // 4 characters, 12 bytes
		{"123456", false},  // max + 1
		{"", false},        // empty
		{"   ", false},     // blank
		{"\t\n", false},    // whitespace only
	}
	for _, c := range cases {
		err := domain.ValidateText("field", c.value, 5)
		if (err == nil) != c.ok {
			t.Errorf("ValidateText(%q): err=%v, want ok=%v", c.value, err, c.ok)
		}
		if err != nil {
			if e, _ := domain.AsError(err); e.Code != domain.CodeValidation {
				t.Errorf("ValidateText(%q): code %s", c.value, e.Code)
			}
		}
	}
}

func TestIsValidTransactionType(t *testing.T) {
	for _, ok := range []string{domain.TxAddMoney, domain.TxTransferOut, domain.TxTransferIn} {
		if !domain.IsValidTransactionType(ok) {
			t.Errorf("%s should be valid", ok)
		}
	}
	for _, bad := range []string{"", "REFUND", "add_money", "TRANSFER"} {
		if domain.IsValidTransactionType(bad) {
			t.Errorf("%q should be invalid", bad)
		}
	}
}

func TestValidatePhone_AllOperators(t *testing.T) {
	for d := '3'; d <= '9'; d++ {
		p := "01" + string(d) + "11000001"
		if err := domain.ValidatePhone(p); err != nil {
			t.Errorf("operator 01%c should be valid: %v", d, err)
		}
	}
	for _, p := range []string{"01011000001", "01111000001", "02711000001", "8801711000001", " 01711000001"} {
		if domain.ValidatePhone(p) == nil {
			t.Errorf("ValidatePhone(%q) should fail", p)
		}
	}
}
