package utils_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/srtipu5/mini-wallet/internal/domain"

	"github.com/srtipu5/mini-wallet/internal/utils"
)

func decodeError(t *testing.T, rec *httptest.ResponseRecorder) utils.ErrorDetail {
	t.Helper()
	var body utils.ErrorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid error body %q: %v", rec.Body, err)
	}
	return body.Error
}

func TestSendData(t *testing.T) {
	rec := httptest.NewRecorder()
	utils.SendData(rec, http.StatusTeapot, map[string]int{"a": 1})
	if rec.Code != http.StatusTeapot || rec.Header().Get("Content-Type") != "application/json" ||
		strings.TrimSpace(rec.Body.String()) != `{"a":1}` {
		t.Fatalf("got %d %q %q", rec.Code, rec.Header().Get("Content-Type"), rec.Body)
	}

	// An unencodable value is logged; the status is already sent.
	rec = httptest.NewRecorder()
	utils.SendData(rec, http.StatusOK, make(chan int))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
}

// Every business error code must map to the documented HTTP status.
func TestSendError_StatusForEveryCode(t *testing.T) {
	want := map[string]int{
		domain.CodeValidation:            400,
		domain.CodeInvalidAmount:         400,
		domain.CodeMissingIdempotencyKey: 400,
		domain.CodeSameWalletTransfer:    400,
		domain.CodeWalletNotFound:        404,
		domain.CodeNotFound:              404,
		domain.CodeDuplicateWallet:       409,
		domain.CodeDuplicateGatewayRef:   409,
		domain.CodeIdempotencyKeyReused:  422,
		domain.CodeInsufficientBalance:   422,
	}
	for code, status := range want {
		rec := httptest.NewRecorder()
		utils.SendError(rec, httptest.NewRequest("GET", "/", nil), domain.NewError(code, "msg "+code))
		if rec.Code != status {
			t.Errorf("%s: status = %d, want %d", code, rec.Code, status)
		}
		if d := decodeError(t, rec); d.Code != code || d.Message != "msg "+code {
			t.Errorf("%s: body = %+v", code, d)
		}
	}
}

func TestSendError_WrappedDomainError(t *testing.T) {
	rec := httptest.NewRecorder()
	utils.SendError(rec, httptest.NewRequest("GET", "/", nil), fmt.Errorf("context: %w", domain.ErrInsufficientBalance))
	if rec.Code != 422 || decodeError(t, rec).Code != domain.CodeInsufficientBalance {
		t.Fatalf("wrapped error: %d %s", rec.Code, rec.Body)
	}
}

func TestSendError_UnmappedCodeIs400(t *testing.T) {
	rec := httptest.NewRecorder()
	utils.SendError(rec, httptest.NewRequest("GET", "/", nil), domain.NewError("SOMETHING_NEW", "x"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// Unexpected errors become a generic 500 and never leak their details.
func TestSendError_InternalErrorHidesDetails(t *testing.T) {
	rec := httptest.NewRecorder()
	utils.SendError(rec, httptest.NewRequest("GET", "/", nil), errors.New("pq: password authentication failed for user wallet"))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	d := decodeError(t, rec)
	if d.Code != domain.CodeInternal || d.Message != "internal server error" || strings.Contains(rec.Body.String(), "password") {
		t.Fatalf("leaked or wrong body: %s", rec.Body)
	}
}

func TestSendInternalError(t *testing.T) {
	rec := httptest.NewRecorder()
	utils.SendInternalError(rec)
	if rec.Code != 500 || decodeError(t, rec).Code != domain.CodeInternal {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
}

func TestSendIdempotent(t *testing.T) {
	rec := httptest.NewRecorder()
	utils.SendIdempotent(rec, false, map[string]string{"id": "1"})
	if rec.Code != http.StatusCreated || rec.Header().Get("Idempotent-Replayed") != "" {
		t.Fatalf("fresh: %d header=%q", rec.Code, rec.Header().Get("Idempotent-Replayed"))
	}

	rec = httptest.NewRecorder()
	utils.SendIdempotent(rec, true, map[string]string{"id": "1"})
	if rec.Code != http.StatusCreated || rec.Header().Get("Idempotent-Replayed") != "true" {
		t.Fatalf("replay: %d header=%q", rec.Code, rec.Header().Get("Idempotent-Replayed"))
	}
}
