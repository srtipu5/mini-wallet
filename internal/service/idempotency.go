package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/srtipu5/mini-wallet/internal/domain"
)

// Idempotency flow, all inside one DB transaction:
//
//  1. claimIdempotency        – reserve the key, or return the saved response on a retry
//  2. do the work             – only when it is not a retry
//  3. saveIdempotencyResponse – store the response for future retries
//
// If anything fails, the transaction rolls back and the key is released too.

const (
	scopeAddMoney = "add_money"
	scopeTransfer = "transfer"
)

// claimIdempotency reserves the key for this request.
// It returns replayed=true with the saved response if the same request
// already used this key, and ErrIdempotencyKeyReused if a different one did.
func claimIdempotency[T any](ctx context.Context, repos Repositories, scope, key string, request any) (T, bool, error) {
	var saved T

	requestHash, err := hashRequest(request)
	if err != nil {
		return saved, false, err
	}

	// Waits if another request with the same key is still running.
	previousRequest, err := repos.Idempotency.Claim(ctx, scope, key, requestHash)
	if err != nil {
		return saved, false, err
	}

	switch {
	case previousRequest == nil: // new key
		return saved, false, nil
	case previousRequest.RequestHash != requestHash: // same key, different request
		return saved, false, domain.ErrIdempotencyKeyReused
	}

	// Retry of the same request: replay the saved response.
	if err := json.Unmarshal(previousRequest.ResponseBody, &saved); err != nil {
		return saved, false, err
	}
	return saved, true, nil
}

// saveIdempotencyResponse stores the response so retries can replay it.
func saveIdempotencyResponse(ctx context.Context, repos Repositories, scope, key string, response any) error {
	body, err := json.Marshal(response)
	if err != nil {
		return err
	}
	return repos.Idempotency.SaveResponse(ctx, scope, key, body)
}

// hashRequest fingerprints the request body so a reused key can be detected.
func hashRequest(request any) (string, error) {
	b, err := json.Marshal(request)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
