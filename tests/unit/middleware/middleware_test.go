package middleware_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/srtipu5/mini-wallet/internal/middleware"
)

func TestRecover_PanicBecomes500(t *testing.T) {
	h := middleware.Recover(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	var body struct {
		Error struct{ Code, Message string } `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Error.Code != "INTERNAL_ERROR" {
		t.Fatalf("body = %s (%v)", rec.Body, err)
	}
	if strings.Contains(rec.Body.String(), "boom") {
		t.Fatal("panic details must not leak to the client")
	}
}

func TestRecover_PassesThrough(t *testing.T) {
	h := middleware.Recover(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", rec.Code)
	}
}

func TestLogger_LogsMethodPathStatus(t *testing.T) {
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })

	cases := []struct {
		handler http.HandlerFunc
		status  float64
	}{
		{func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) }, 418},
		{func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) }, 200}, // implicit 200
	}
	for _, c := range cases {
		buf.Reset()
		rec := httptest.NewRecorder()
		middleware.Logger(c.handler).ServeHTTP(rec, httptest.NewRequest("POST", "/api/v1/x", nil))

		var line map[string]any
		if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
			t.Fatalf("log line %q: %v", buf.String(), err)
		}
		if line["msg"] != "request" || line["method"] != "POST" || line["path"] != "/api/v1/x" || line["status"] != c.status {
			t.Fatalf("log line = %v, want status %v", line, c.status)
		}
		if _, ok := line["duration_ms"]; !ok {
			t.Fatal("duration_ms missing")
		}
		if rec.Code != int(c.status) {
			t.Fatalf("response status = %d, want %v", rec.Code, c.status)
		}
	}
}
