package service

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
// (scope, key):
//
//	BEGIN
//	  1. claim the key      INSERT INTO idempotency_keys ... ON CONFLICT DO NOTHING
//	  2. key already used?  same request -> return the saved response (replay)
//	                        other request -> IDEMPOTENCY_KEY_REUSED
//	  3. do the work        e.g. the transfer (all its SQL uses the same tx)
//	  4. save the response  UPDATE idempotency_keys SET response_body = ...
//	COMMIT    -- or ROLLBACK if any step returns an error
//
// The key and the money movement commit together, so a crash can never leave
// "money moved but key not saved", or the other way round. If the work fails,
// the key is rolled back too, so the client can safely retry with it.
func runIdempotent[T any](
	ctx context.Context,
	store Store,
	scope, key string,
	request any,
	work func(tx Repositories) (T, error),
) (result T, replayed bool, err error) {
	hash, err := hashRequest(request)
	if err != nil {
		return result, false, err
	}

	err = store.WithinTx(ctx, func(tx Repositories) error {
		// 1. Claim the key. A concurrent request with the same key waits here
		//    until our transaction finishes.
		existing, err := tx.Idempotency.Claim(ctx, scope, key, hash)
		if err != nil {
			return err
		}

		// 2. Key already used: reject a different request, replay the same one.
		if existing != nil {
			if existing.RequestHash != hash {
				return domain.ErrIdempotencyKeyReused
			}
			replayed = true
			return json.Unmarshal(existing.ResponseBody, &result)
		}

		// 3. Do the work.
		result, err = work(tx)
		if err != nil {
			return err
		}

		// 4. Save the response, so a retry gets exactly the same answer.
		body, err := json.Marshal(result)
		if err != nil {
			return err
		}
		return tx.Idempotency.SaveResponse(ctx, scope, key, body)
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
