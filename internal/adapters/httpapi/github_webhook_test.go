package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lanej/statecraft/internal/domain"
	"github.com/lanej/statecraft/internal/service"
)

type failingProposedPlanner struct{ calls int }

func (p *failingProposedPlanner) Plan(context.Context, domain.RepositoryRef, int64, []domain.RootSelector) (service.ProposedPlan, error) {
	p.calls++
	return service.ProposedPlan{}, errors.New("uncertain transport result")
}

func TestWebhookValidationAndUncertainDelivery(t *testing.T) {
	planner := &failingProposedPlanner{}
	handler := NewGitHubWebhook("secret", planner, map[string][]domain.RootSelector{"acme/infra": {{ID: "root", PlannerRef: "sample"}}})
	send := func(body, delivery string) int {
		request := httptest.NewRequest("POST", "/webhooks/github", strings.NewReader(body))
		mac := hmac.New(sha256.New, []byte("secret"))
		mac.Write([]byte(body))
		request.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
		request.Header.Set("X-GitHub-Event", "pull_request")
		request.Header.Set("X-GitHub-Delivery", delivery)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		return recorder.Code
	}
	for _, tc := range []struct {
		body, id string
		status   int
	}{
		{`invalid JSON`, "malformed", 400},
		{`{"action":"opened","number":42,"repository":{"full_name":"other/infra"}}`, "unknown-repo", 403},
		{`{"action":"closed","number":42,"repository":{"full_name":"acme/infra"}}`, "closed", 204},
		{`{"action":"opened","number":42,"repository":{"full_name":"acme/infra"}}`, "", 400},
	} {
		if got := send(tc.body, tc.id); got != tc.status {
			t.Errorf("status = %d, want %d", got, tc.status)
		}
	}
	if planner.calls != 0 {
		t.Fatal("invalid or ignored events reached planner")
	}
	body := `{"action":"opened","number":42,"repository":{"full_name":"acme/infra"}}`
	if got := send(body, "uncertain"); got != 502 {
		t.Fatalf("failed dispatch status = %d", got)
	}
	if got := send(body, "uncertain"); got != 204 {
		t.Fatalf("duplicate uncertain status = %d", got)
	}
	if planner.calls != 1 {
		t.Fatalf("uncertain delivery replayed %d times", planner.calls)
	}
}
