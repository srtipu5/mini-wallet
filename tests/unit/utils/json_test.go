package utils_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/srtipu5/mini-wallet/internal/domain"

	"github.com/srtipu5/mini-wallet/internal/utils"
)

type sample struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func TestDecodeJSON_Valid(t *testing.T) {
	var dst sample
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"a","count":2}`))
	if err := utils.DecodeJSON(httptest.NewRecorder(), r, &dst); err != nil {
		t.Fatal(err)
	}
	if dst.Name != "a" || dst.Count != 2 {
		t.Fatalf("decoded %+v", dst)
	}
}

func TestDecodeJSON_Invalid(t *testing.T) {
	cases := map[string]struct {
		body string
		msg  string
	}{
		"empty body":    {``, "request body is required"},
		"malformed":     {`{"name":`, "request body is not valid JSON"},
		"syntax error":  {`{name}`, "request body is not valid JSON"},
		"wrong type":    {`{"count":"two"}`, `field "count" has the wrong type`},
		"not an object": {`[1,2]`, "has the wrong type"},
		"unknown field": {`{"name":"a","admin":true}`, `unknown field "admin"`},
		"trailing data": {`{"name":"a"} {"name":"b"}`, "single JSON object"},
		"trailing junk": {`{"name":"a"} x`, "single JSON object"},
		"too large":     {`{"name":"` + strings.Repeat("x", 1<<20) /* body limit is 1 MB */ + `"}`, "too large"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			var dst sample
			r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(c.body))
			err := utils.DecodeJSON(httptest.NewRecorder(), r, &dst)
			e, ok := domain.AsError(err)
			if !ok || e.Code != domain.CodeValidation {
				t.Fatalf("err = %v, want VALIDATION_ERROR", err)
			}
			if !strings.Contains(e.Message, c.msg) {
				t.Fatalf("message = %q, want it to contain %q", e.Message, c.msg)
			}
		})
	}
}
