// Package handler parses HTTP requests, calls the service and writes responses.
package handler

import "github.com/srtipu5/mini-wallet/internal/service"

type Handler struct {
	svc *service.Service
}

func New(svc *service.Service) *Handler {
	return &Handler{svc: svc}
}
