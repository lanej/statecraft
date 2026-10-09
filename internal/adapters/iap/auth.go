// Package iap validates the signed assertion; editable identity headers are not
// authentication. Native Cloud Run IAP and its IAM policy remain the access gate.
package iap

import (
	"context"
	"net/http"
	"time"

	"google.golang.org/api/idtoken"
)

type Validate func(context.Context, string, string) (*idtoken.Payload, error)

func Protect(audience string, next http.Handler) http.Handler {
	return protect(audience, idtoken.Validate, next)
}

func protect(audience string, validate Validate, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		assertion := r.Header.Get("X-Goog-Iap-Jwt-Assertion")
		if assertion == "" || audience == "" {
			http.Error(w, "IAP authentication required", http.StatusUnauthorized)
			return
		}
		payload, err := validate(ctx, assertion, audience)
		if err != nil || payload == nil || payload.Issuer != "https://cloud.google.com/iap" || payload.Subject == "" ||
			payload.Audience != audience || payload.Expires <= time.Now().Unix() || payload.IssuedAt > time.Now().Unix()+30 {
			http.Error(w, "IAP authentication required", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
