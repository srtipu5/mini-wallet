// Package handler is the HTTP delivery layer: parse the request, call the
// service layer, write the response. No business rules live here.
// One file per resource; routes are registered in ../routes, request bodies
// live in internal/dto, and shared helpers in internal/utils.
package handler

import "github.com/srtipu5/mini-wallet/internal/service"

type Handler struct {
	svc *service.Service
}

func New(svc *service.Service) *Handler {
	return &Handler{svc: svc}
}
