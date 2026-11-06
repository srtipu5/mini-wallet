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

// Transaction history paging limits.
const (
	DefaultPageSize = 20
	MaxPageSize     = 100
	MaxPage         = 100_000 // keeps OFFSET sane
)

// IsValidTransactionType reports whether t is a known history entry type.
func IsValidTransactionType(t string) bool {
	return t == TxAddMoney || t == TxTransferOut || t == TxTransferIn
}

// Bangladeshi mobile number in local format: 01 + operator digit (3-9) + 8 digits.
var phonePattern = regexp.MustCompile(`^01[3-9][0-9]{8}$`)

// uuidPattern matches the canonical 8-4-4-4-12 hex format.
var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// ValidateAmount checks the per-transaction limits. Both cases use the
// INVALID_AMOUNT code; the message says which limit was broken.
func ValidateAmount(amount int64) error {
	if amount < MinAmount {
		return NewError(CodeInvalidAmount,
			fmt.Sprintf("Amount must be at least %d poisha (%d BDT)", MinAmount, MinAmount/100))
	}
	if amount > MaxAmount {
		return NewError(CodeInvalidAmount,
			fmt.Sprintf("Amount must not exceed %d poisha (%d BDT)", MaxAmount, MaxAmount/100))
	}
	return nil
}

func ValidatePhone(phone string) error {
	if !phonePattern.MatchString(phone) {
		return Validation("Phone must be a valid Bangladeshi mobile number.")
	}
	return nil
}

// ValidateText checks a required free-text field (non-blank, max length).
func ValidateText(field, value string, maxLen int) error {
	if strings.TrimSpace(value) == "" {
		return Validation("The " + field + " field is required")
	}
	if utf8.RuneCountInString(value) > maxLen {
		return Validation(fmt.Sprintf("The %s field must be at most %d characters", field, maxLen))
	}
	return nil
}

// IsValidWalletID reports whether id is a well-formed wallet id.
// A malformed id can never match a wallet, so callers treat it as "not found".
func IsValidWalletID(id string) bool { return uuidPattern.MatchString(id) }
