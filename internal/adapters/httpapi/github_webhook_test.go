package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	return service.ProposedPlan{PlannedHeadSHA: "head", Stale: true, Run: domain.PlanRun{Attempts: []domain.PlanAttempt{{RootID: "root", Phase: "plan", Status: "succeeded", Output: "retained evidence"}}}}, errors.New("uncertain transport result")
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
		if delivery == "uncertain" {
			var receipt PlanningReceipt
			if err := json.Unmarshal(recorder.Body.Bytes(), &receipt); err != nil {
				t.Fatal(err)
			}
			if receipt.Result == nil || len(receipt.Result.Run.Attempts) != 1 || receipt.Result.Run.Attempts[0].Output != "retained evidence" || receipt.Failure == "" || receipt.State != "failed" {
				t.Fatalf("partial evidence or failure lost: %#v", receipt)
			}
		}
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
	if got := send(body, "uncertain"); got != 502 {
		t.Fatalf("duplicate uncertain status = %d", got)
	}
	if planner.calls != 1 {
		t.Fatalf("uncertain delivery replayed %d times", planner.calls)
	}
}

type blockedPlanner struct {
	started chan struct{}
	release chan struct{}
}

func (p *blockedPlanner) Plan(context.Context, domain.RepositoryRef, int64, []domain.RootSelector) (service.ProposedPlan, error) {
	close(p.started)
	<-p.release
	return service.ProposedPlan{PlannedHeadSHA: "head"}, nil
}

func TestDuplicateInFlightReturnsPendingThenOriginalReceipt(t *testing.T) {
	planner := &blockedPlanner{started: make(chan struct{}), release: make(chan struct{})}
	handler := NewGitHubWebhook("secret", planner, map[string][]domain.RootSelector{"acme/infra": {{ID: "root"}}})
	body := `{"action":"opened","number":42,"repository":{"full_name":"acme/infra"}}`
	send := func() *httptest.ResponseRecorder {
		request := httptest.NewRequest("POST", "/webhooks/github", strings.NewReader(body))
		mac := hmac.New(sha256.New, []byte("secret"))
		mac.Write([]byte(body))
		request.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
		request.Header.Set("X-GitHub-Event", "pull_request")
		request.Header.Set("X-GitHub-Delivery", "inflight")
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		return recorder
	}
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- send() }()
	<-planner.started
	pending := send()
	// Release the provider before assertions so failure cannot leave a blocked worker.
	close(planner.release)
	original := <-done
	var receipt PlanningReceipt
	if err := json.Unmarshal(pending.Body.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	if pending.Code != 202 || receipt.State != "pending" || receipt.Result != nil {
		t.Fatalf("inflight receipt = %d %#v", pending.Code, receipt)
	}
	duplicate := send()
	if original.Code != 200 || duplicate.Code != original.Code || duplicate.Body.String() != original.Body.String() {
		t.Fatalf("completed receipt changed: %d %s", duplicate.Code, duplicate.Body.String())
	}
}
