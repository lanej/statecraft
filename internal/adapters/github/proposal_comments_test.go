package githubadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/lanej/statecraft/internal/adapters/mock"
	"github.com/lanej/statecraft/internal/domain"
)

func TestProposalCommentUsesCoreFactsAndGitHubDestination(t *testing.T) {
	// Only this integration composition supplies a repository and pull request.
	// The pure summary function sees plan/policy evidence, never the destination.
	review, err := mock.SeedReview("review", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	summary := domain.SummarizeProposal(domain.ProposalSummaryInput{HeadSHA: review.HeadSHA, State: review.State, Plan: review.Plan, Policy: review.Policy, Roots: review.Roots, Decisions: review.Decisions, Acceptances: review.Acceptances, Verification: review.Verification, Simulation: review.Demo, Now: time.Now()})
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	source := testSource(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/repos/acme/infra/issues/7/comments" {
			t.Errorf("wrong GitHub discussion endpoint: %s %s", r.Method, r.URL.Path)
		}
		var payload struct {
			Body string `json:"body"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		if !strings.Contains(payload.Body, "Blocking violations: 1 (0 with active acceptance)") || !strings.Contains(payload.Body, "Current plan approval recorded: false") || !strings.Contains(payload.Body, "**Simulation:**") {
			t.Errorf("misleading comment: %s", payload.Body)
		}
		if strings.Contains(payload.Body, "acme/infra") || strings.Contains(payload.Body, "pull request") {
			t.Error("core facts include destination")
		}
		t.Logf("mock GitHub received %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"id":9001,"html_url":"https://github.example/acme/infra/pull/7#issuecomment-9001"}`)
	})
	receipt, err := source.PublishProposalComment(context.Background(), PullRequestTarget{Owner: "acme", Repository: "infra", Number: 7}, summary, logger)
	if err != nil || receipt.GetID() != 9001 {
		t.Fatal(receipt, err)
	}
	t.Log(strings.TrimSpace(logs.String()))
	if len(review.Decisions) != 0 || len(review.Acceptances) != 0 {
		t.Fatal("publication recorded a domain decision")
	}
}

func TestProposalCommentRejectsFailedAndUncertainReceipts(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  int
		body    string
		outcome string
	}{
		{"provider rejection", 403, `{"message":"forbidden"}`, "rejected"},
		{"provider failure", 500, `{"message":"unavailable"}`, "uncertain"},
		{"missing receipt", 201, `{}`, "uncertain"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			source := testSource(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			})
			var logs bytes.Buffer
			receipt, err := source.PublishProposalComment(context.Background(), PullRequestTarget{Owner: "acme", Repository: "infra", Number: 7}, domain.ProposalSummary{}, slog.New(slog.NewJSONHandler(&logs, nil)))
			if err == nil || receipt != nil || calls != 1 || !strings.Contains(logs.String(), `"outcome":"`+tc.outcome+`"`) {
				t.Fatal("publication silently succeeded or retried", receipt, err, calls, logs.String())
			}
		})
	}
}

func TestProposalCommentRequiresExplicitDestination(t *testing.T) {
	source := testSource(t, func(w http.ResponseWriter, r *http.Request) { t.Error("invalid destination reached GitHub") })
	if _, err := source.PublishProposalComment(context.Background(), PullRequestTarget{}, domain.ProposalSummary{}, nil); err == nil {
		t.Fatal("accepted missing target")
	}
}
