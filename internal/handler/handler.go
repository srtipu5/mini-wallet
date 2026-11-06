// Package handler turns HTTP requests into service calls: parse the request,
// call the service, write the response. No business rules live here.
//
// One file per resource. Routes are registered in internal/routes, request
// and response shapes live in internal/dto, shared helpers in internal/utils.
package handler

import "github.com/srtipu5/mini-wallet/internal/service"

type Handler struct {
	svc *service.Service
}

func New(svc *service.Service) *Handler {
	return &Handler{svc: svc}
}
