package domain

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Money limits, all in poisha (100 poisha = 1 BDT).
const (
	MinAmount int64 = 100       // 1 BDT
	MaxAmount int64 = 2_500_000 // 25,000 BDT
)

const (
	MaxUserIDLength         = 64
	MaxGatewayRefLength     = 128
	MaxNoteLength           = 255
	MaxIdempotencyKeyLength = 255
)

// Bangladeshi mobile number in local format: 01 + operator digit (3-9) + 8 digits.
var phonePattern = regexp.MustCompile(`^01[3-9][0-9]{8}$`)

// uuidPattern matches the canonical 8-4-4-4-12 hex format.
var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func ValidateAmount(amount int64) error {
	if amount < MinAmount || amount > MaxAmount {
		return NewError(CodeInvalidAmount,
			fmt.Sprintf("amount must be an integer between %d and %d poisha", MinAmount, MaxAmount))
	}
	return nil
}

func ValidatePhone(phone string) error {
	if !phonePattern.MatchString(phone) {
		return Validation("phone must be a valid Bangladeshi mobile number like 01711000001")
	}
	return nil
}

// ValidateText checks a required free-text field (non-blank, max length).
func ValidateText(field, value string, maxLen int) error {
	if strings.TrimSpace(value) == "" {
		return Validation(field + " is required")
	}
	if utf8.RuneCountInString(value) > maxLen {
		return Validation(fmt.Sprintf("%s must be at most %d characters", field, maxLen))
	}
	return nil
}

// IsValidWalletID reports whether id is a well-formed wallet id.
// A malformed id can never match a wallet, so callers treat it as "not found".
func IsValidWalletID(id string) bool { return uuidPattern.MatchString(id) }
