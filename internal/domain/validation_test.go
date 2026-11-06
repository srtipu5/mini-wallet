package domain

import "testing"

func TestValidateAmount(t *testing.T) {
	cases := []struct {
		amount int64
		ok     bool
	}{
		{-100, false},
		{0, false},
		{99, false},
		{100, true},       // minimum
		{2_500_000, true}, // maximum
		{2_500_001, false},
	}
	for _, c := range cases {
		err := ValidateAmount(c.amount)
		if (err == nil) != c.ok {
			t.Errorf("ValidateAmount(%d): got err=%v, want ok=%v", c.amount, err, c.ok)
		}
		if err != nil {
			if e, _ := AsError(err); e.Code != CodeInvalidAmount {
				t.Errorf("ValidateAmount(%d): code %s, want %s", c.amount, e.Code, CodeInvalidAmount)
			}
		}
	}
}

func TestValidatePhone(t *testing.T) {
	valid := []string{"01711000001", "01311111111", "01999999999"}
	invalid := []string{"", "0171100000", "017110000012", "01211000001", "+8801711000001", "0171100000a"}

	for _, p := range valid {
		if err := ValidatePhone(p); err != nil {
			t.Errorf("ValidatePhone(%q) should pass: %v", p, err)
		}
	}
	for _, p := range invalid {
		if err := ValidatePhone(p); err == nil {
			t.Errorf("ValidatePhone(%q) should fail", p)
		}
	}
}

func TestIsValidWalletID(t *testing.T) {
	if !IsValidWalletID("3f2b8c4e-1a2b-4c3d-8e9f-0a1b2c3d4e5f") {
		t.Error("valid uuid rejected")
	}
	for _, id := range []string{"", "abc", "3f2b8c4e1a2b4c3d8e9f0a1b2c3d4e5f", "' OR 1=1 --"} {
		if IsValidWalletID(id) {
			t.Errorf("IsValidWalletID(%q) should be false", id)
		}
	}
}
