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

// withIdempotency runs fn at most once per (scope, key) and replays the saved
// response on retries. It must run inside the caller's transaction so the key
// and the work commit or roll back together.
func withIdempotency[T any](
	ctx context.Context,
	repos Repositories,
	scope, key string,
	request any,
	fn func() (T, error),
) (result T, replayed bool, err error) {
	hash, err := hashRequest(request)
	if err != nil {
		return result, false, err
	}

	// Blocks while another in-flight request holds the same key.
	existing, err := repos.Idempotency.Claim(ctx, scope, key, hash)
	if err != nil {
		return result, false, err
	}
	if existing != nil {
		if existing.RequestHash != hash {
			return result, false, domain.ErrIdempotencyKeyReused
		}
		if err := json.Unmarshal(existing.ResponseBody, &result); err != nil {
			return result, false, err
		}
		return result, true, nil
	}

	if result, err = fn(); err != nil {
		return result, false, err
	}

	body, err := json.Marshal(result)
	if err != nil {
		return result, false, err
	}
	return result, false, repos.Idempotency.SaveResponse(ctx, scope, key, body)
}

func hashRequest(request any) (string, error) {
	b, err := json.Marshal(request)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
