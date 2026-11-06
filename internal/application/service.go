// Package application holds the business logic: validation, locking order,
// balance checks and idempotency. It knows nothing about HTTP.
package application

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	db *pgxpool.Pool
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

// Ping checks the database connection (used by /health).
func (s *Service) Ping(ctx context.Context) error {
	return s.db.Ping(ctx)
}
