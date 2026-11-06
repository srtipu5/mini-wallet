package utils_test

import (
	"testing"

	"github.com/srtipu5/mini-wallet/internal/utils"
)

func TestGetenv(t *testing.T) {
	t.Setenv("MW_TEST_SET", "value")
	t.Setenv("MW_TEST_EMPTY", "")

	if got := utils.Getenv("MW_TEST_SET", "fallback"); got != "value" {
		t.Errorf("set: got %q", got)
	}
	if got := utils.Getenv("MW_TEST_EMPTY", "fallback"); got != "fallback" {
		t.Errorf("empty: got %q, want fallback", got)
	}
	if got := utils.Getenv("MW_TEST_DOES_NOT_EXIST", "fallback"); got != "fallback" {
		t.Errorf("unset: got %q, want fallback", got)
	}
}
