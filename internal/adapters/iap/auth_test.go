package iap

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"google.golang.org/api/idtoken"
)

func TestAuthenticationFailsClosed(t *testing.T) {
	const audience = "/projects/123/locations/us-east1/services/statecraft"
	for _, test := range []struct {
		name    string
		header  string
		payload *idtoken.Payload
		err     error
		want    int
	}{
		{"missing", "", nil, nil, 401},
		{"bad signature", "forged", nil, errors.New("signature"), 401},
		{"wrong issuer", "token", &idtoken.Payload{Issuer: "attacker", Audience: audience, Subject: "u", Expires: time.Now().Unix() + 60}, nil, 401},
		{"wrong audience", "token", &idtoken.Payload{Issuer: "https://cloud.google.com/iap", Audience: "other", Subject: "u", Expires: time.Now().Unix() + 60}, nil, 401},
		{"expired", "token", &idtoken.Payload{Issuer: "https://cloud.google.com/iap", Audience: audience, Subject: "u", Expires: 1}, nil, 401},
		{"valid", "token", &idtoken.Payload{Issuer: "https://cloud.google.com/iap", Audience: audience, Subject: "u", Expires: time.Now().Unix() + 60}, nil, 204},
	} {
		t.Run(test.name, func(t *testing.T) {
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) })
			handler := protect(audience, func(_ context.Context, _ string, got string) (*idtoken.Payload, error) {
				if got != audience {
					t.Fatal("audience not supplied to signature validation")
				}
				return test.payload, test.err
			}, next)
			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set("X-Goog-Iap-Jwt-Assertion", test.header)
			req.Header.Set("X-Goog-Authenticated-User-Email", "accounts.google.com:admin@easypost.com")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			if w.Code != test.want {
				t.Fatalf("status %d, want %d", w.Code, test.want)
			}
		})
	}
}

func TestRealValidatorRejectsForgedAssertion(t *testing.T) {
	handler := Protect("expected", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) }))
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Goog-Iap-Jwt-Assertion", "forged.assertion.signature")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatalf("forged assertion passed: %d", w.Code)
	}
}
