// Package handler is the HTTP delivery layer: parse the request, call the
// application service, write the response. No business rules live here.
// One file per resource; routes are registered in ../routes, request bodies
// live in internal/dto, and shared helpers in internal/utils.
package handler

import "github.com/srtipu5/mini-wallet/internal/application"

type Handler struct {
	svc *application.Service
}

func New(svc *application.Service) *Handler {
	return &Handler{svc: svc}
}
