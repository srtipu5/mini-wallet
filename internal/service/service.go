// Package service holds the business logic: validation, balance checks,
// locking order and idempotency.
//
// It knows nothing about HTTP or about which database is used. It reads and
// writes data only through the interfaces in repository.go.
package service

import "context"

type Service struct {
	store Store
}

func New(store Store) *Service {
	return &Service{store: store}
}

// Ping checks the database connection (used by /health).
func (s *Service) Ping(ctx context.Context) error {
	return s.store.Ping(ctx)
}
