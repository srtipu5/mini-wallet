package tests

import (
	"net/http"
	"testing"
)

func TestHealth(t *testing.T) {
	r := call(t, "GET", "/health", nil, "")
	expectStatus(t, r, http.StatusOK)
	if r.JSON(t)["status"] != "ok" {
		t.Fatalf("unexpected body: %s", r.Body)
	}
}
