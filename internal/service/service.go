// Package service holds the business logic (use cases): validation,
// locking order, balance checks and idempotency. It knows nothing about HTTP
// or about which database is used; it only talks to the ports in ports.go.
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
