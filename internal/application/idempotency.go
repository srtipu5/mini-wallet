package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/srtipu5/mini-wallet/internal/domain"
	"github.com/srtipu5/mini-wallet/internal/repository"
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
	db *pgxpool.Pool,
	scope, key string,
	request any,
	work func(tx pgx.Tx) (T, error),
) (result T, replayed bool, err error) {
	hash, err := hashRequest(request)
	if err != nil {
		return result, false, err
	}

	err = pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		existing, err := repository.ClaimIdempotencyKey(ctx, tx, scope, key, hash)
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

		result, err = work(tx)
		if err != nil {
			return err
		}

		body, err := json.Marshal(result)
		if err != nil {
			return err
		}
		return repository.SaveIdempotencyResponse(ctx, tx, scope, key, body)
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
