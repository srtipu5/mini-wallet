package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/srtipu5/mini-wallet/internal/domain"
)

const (
	scopeAddMoney = "add_money"
	scopeTransfer = "transfer"
)

// runIdempotent runs work inside ONE database transaction, at most once per
// (scope, key). Steps:
//
//  1. Claim the key (blocks if a concurrent request holds the same key).
//  2. Key already used with a different request  -> IDEMPOTENCY_KEY_REUSED.
//     Key already used with the same request     -> return the stored response.
//  3. Otherwise do the work and store its response in the same transaction.
//
// Because the key and the money movement commit together, a crash can never
// leave "money moved but key not saved" or the other way round.
//
// If work returns an error, the whole transaction (including the key) is
// rolled back: nothing happened, so the client may safely retry with the key.
func runIdempotent[T any](
	ctx context.Context,
	store Store,
	scope, key string,
	request any,
	work func(r Repositories) (T, error),
) (result T, replayed bool, err error) {
	hash, err := hashRequest(request)
	if err != nil {
		return result, false, err
	}

	err = store.WithinTx(ctx, func(r Repositories) error {
		existing, err := r.Idempotency.Claim(ctx, scope, key, hash)
		if err != nil {
			return err
		}

		if existing != nil {
			if existing.RequestHash != hash {
				return domain.ErrIdempotencyKeyReused
			}
			replayed = true
			return json.Unmarshal(existing.ResponseBody, &result)
		}

		result, err = work(r)
		if err != nil {
			return err
		}

		body, err := json.Marshal(result)
		if err != nil {
			return err
		}
		return r.Idempotency.SaveResponse(ctx, scope, key, body)
	})
	return result, replayed, err
}

// hashRequest fingerprints the normalized request. Because we hash the parsed
// struct (not raw bytes), whitespace or key order in the JSON does not matter.
func hashRequest(request any) (string, error) {
	b, err := json.Marshal(request)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
