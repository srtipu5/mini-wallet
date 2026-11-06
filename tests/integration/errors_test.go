package tests

import (
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

// Every error the API can return uses the standard envelope and a message
// written as a sentence: capital first letter, full stop at the end.
func TestErrorMessagesAreSentences(t *testing.T) {
	resetDB(t)
	a := createWallet(t, "u_a")
	b := createWallet(t, "u_b")
	addMoney(t, a, 1000)
	expectStatus(t, call(t, "POST", addMoneyPath(a), `{"amount":500,"gateway_ref":"GW-used"}`, "used-key"), 201)
	missing := "00000000-0000-0000-0000-000000000000"
	long := strings.Repeat("x", 300)

	responses := map[string]response{
		"empty body":          call(t, "POST", "/api/v1/wallets", ``, ""),
		"invalid JSON":        call(t, "POST", "/api/v1/wallets", `{"user_id":`, ""),
		"wrong type":          call(t, "POST", "/api/v1/wallets", `{"user_id":1,"phone":"01711000001"}`, ""),
		"unknown field":       call(t, "POST", "/api/v1/wallets", `{"user_id":"u","phone":"01711000001","x":1}`, ""),
		"trailing data":       call(t, "POST", "/api/v1/wallets", `{"user_id":"u","phone":"01711000001"} {}`, ""),
		"missing user_id":     call(t, "POST", "/api/v1/wallets", `{"phone":"01711000001"}`, ""),
		"user_id too long":    call(t, "POST", "/api/v1/wallets", `{"user_id":"`+long+`","phone":"01711000001"}`, ""),
		"bad phone":           call(t, "POST", "/api/v1/wallets", `{"user_id":"u","phone":"1"}`, ""),
		"duplicate user":      call(t, "POST", "/api/v1/wallets", `{"user_id":"u_a","phone":"01799999999"}`, ""),
		"unknown wallet":      call(t, "GET", "/api/v1/wallets/"+missing, nil, ""),
		"missing key":         call(t, "POST", addMoneyPath(a), `{"amount":500,"gateway_ref":"G"}`, ""),
		"key too long":        call(t, "POST", addMoneyPath(a), `{"amount":500,"gateway_ref":"G"}`, long),
		"missing amount":      call(t, "POST", addMoneyPath(a), `{"gateway_ref":"G"}`, "k1"),
		"float amount":        call(t, "POST", addMoneyPath(a), `{"amount":1.5,"gateway_ref":"G"}`, "k2"),
		"amount too small":    call(t, "POST", addMoneyPath(a), `{"amount":1,"gateway_ref":"G"}`, "k3"),
		"amount too large":    call(t, "POST", addMoneyPath(a), `{"amount":2500001,"gateway_ref":"G"}`, "k4"),
		"missing gateway_ref": call(t, "POST", addMoneyPath(a), `{"amount":500}`, "k5"),
		"duplicate gateway":   call(t, "POST", addMoneyPath(a), `{"amount":500,"gateway_ref":"GW-used"}`, "k6"),
		"key reused":          call(t, "POST", addMoneyPath(a), `{"amount":999,"gateway_ref":"GW-used"}`, "used-key"),
		"missing wallet ids":  call(t, "POST", "/api/v1/transfers", `{"amount":500}`, "k7"),
		"same wallet":         transfer(t, a, a, 500, "k8"),
		"unknown sender":      transfer(t, missing, b, 500, "k9"),
		"unknown receiver":    transfer(t, a, missing, 500, "k10"),
		"insufficient":        transfer(t, b, a, 500, "k11"),
		"note too long":       call(t, "POST", "/api/v1/transfers", `{"from_wallet_id":"`+a+`","to_wallet_id":"`+b+`","amount":500,"note":"`+long+`"}`, "k12"),
		"bad type filter":     call(t, "GET", historyPath(a, "?type=X"), nil, ""),
		"bad page_size":       call(t, "GET", historyPath(a, "?page_size=0"), nil, ""),
		"bad date":            call(t, "GET", historyPath(a, "?from=yesterday"), nil, ""),
		"from after to":       call(t, "GET", historyPath(a, "?from=2026-11-05&to=2026-11-01"), nil, ""),
		"unknown route":       call(t, "GET", "/api/v1/nope", nil, ""),
	}
	for name, r := range responses {
		if r.Status < 400 {
			t.Errorf("%s: status %d, want an error", name, r.Status)
			continue
		}
		msg, _ := r.JSON(t)["error"].(map[string]any)["message"].(string)
		first, _ := utf8.DecodeRuneInString(msg)
		if msg == "" || !unicode.IsUpper(first) || !strings.HasSuffix(msg, ".") {
			t.Errorf("%s: message %q must start with a capital letter and end with a full stop", name, msg)
		}
	}
}
