// Package handler is the HTTP delivery layer: parse the request, call the
// application service, write the response. No business rules live here.
//
//	routes.go               every route in one place
//	dto.go                  request bodies
//	*_handler.go            one file per resource
//	internal/utils          shared JSON / query / response helpers
package handler

import "github.com/srtipu5/mini-wallet/internal/application"

type Handler struct {
	svc *application.Service
}

func New(svc *application.Service) *Handler {
	return &Handler{svc: svc}
}
