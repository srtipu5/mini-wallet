// Package service contains the business logic. It talks to storage only
// through the interfaces in repository.go.
package service

import "context"

type Service struct {
	store Store
}

func New(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) Ping(ctx context.Context) error {
	return s.store.Ping(ctx)
}
