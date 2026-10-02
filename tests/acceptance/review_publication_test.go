package acceptance_test

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	gh "github.com/google/go-github/v92/github"
	v1 "github.com/lanej/statecraft/gen/statecraft/v1"
	"github.com/lanej/statecraft/gen/statecraft/v1/statecraftv1connect"
	"github.com/lanej/statecraft/internal/adapters/connectapi"
	githubadapter "github.com/lanej/statecraft/internal/adapters/github"
	"github.com/lanej/statecraft/internal/adapters/mock"
	"github.com/lanej/statecraft/internal/domain"
	"github.com/lanej/statecraft/internal/service"
)

// The acceptance path crosses generated transport, service gates, pure summary,
// and the actual GitHub SDK. Provider traffic stays on local HTTP test servers.
func TestReviewWorkflowPublishesProviderIndependentSummary(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	store := mock.NewReviewStore()
	workflow := service.NewDemoWorkflow(store, mock.Planner{}, mock.Policies{}, mock.Policies{}, time.Now)
	api := httptest.NewServer(connectapi.Handler(service.NewReviews(store), workflow, mock.SeedReview, connect.WithInterceptors(connectapi.Logging(logger))))
	defer api.Close()
	rpc := statecraftv1connect.NewDemoWorkflowServiceClient(api.Client(), api.URL, connect.WithProtoJSON())
	ctx := context.Background()
	created, err := rpc.CreateDemo(ctx, connect.NewRequest(&v1.CreateDemoRequest{Scenario: "review"}))
	if err != nil {
		t.Fatal(err)
	}
	review := created.Msg.Review
	action := func(action, violation, reason, evidence string, version uint64) error {
		response, err := rpc.ActOnDemo(ctx, connect.NewRequest(&v1.ActOnDemoRequest{ReviewId: review.Id, Command: &v1.WorkflowCommand{Action: action, ExpectedVersion: version, ViolationId: violation, Reason: reason, Evidence: evidence}}))
		if err == nil {
			review = response.Msg.Review
		}
		return err
	}
	if err = action("approve", "", "", "", review.Version); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatal("blocking risk did not deny approval", err)
	}
	violation := review.Policy.Violations[0].Id
	if err = action("request_acceptance", violation, "Reviewed migration", "Recovery drill", review.Version); err != nil {
		t.Fatal(err)
	}
	if err = action("grant_acceptance", violation, "", "", review.Version); err != nil {
		t.Fatal(err)
	}
	if len(review.Decisions) != 0 || review.Policy.Results[0].Outcome != "violated" {
		t.Fatal("acceptance became approval/compliance")
	}
	oldVersion := review.Version
	if err = action("approve", "", "Reviewed exact proposal", "", review.Version); err != nil {
		t.Fatal(err)
	}
	if err = action("approve", "", "", "", oldVersion); connect.CodeOf(err) != connect.CodeAborted {
		t.Fatal("stale command was not rejected", err)
	}
	if err = action("apply", "", "", "", review.Version); err != nil {
		t.Fatal(err)
	}
	if review.State != "applied" || review.Verification != "pending" {
		t.Fatal("execution became verification")
	}
	if err = action("verify", "", "", "", review.Version); err != nil {
		t.Fatal(err)
	}

	// Application shell projects only Statecraft-owned facts for the pure core.
	evidence, err := workflow.Get(ctx, review.Id)
	if err != nil {
		t.Fatal(err)
	}
	summary := domain.SummarizeProposal(domain.ProposalSummaryInput{
		HeadSHA: evidence.HeadSHA, State: evidence.State, Plan: evidence.Plan, Policy: evidence.Policy, Roots: evidence.Roots,
		Decisions: evidence.Decisions, Acceptances: evidence.Acceptances, Verification: evidence.Verification, Simulation: evidence.Demo, Now: time.Now(),
	})
	if !summary.CurrentApproval || summary.BlockingViolations != 1 || summary.AcceptedViolations != 1 || summary.Verification != "state_matches" {
		t.Fatal(summary)
	}

	destination := githubadapter.PullRequestTarget{Owner: "acme", Repository: "infra", Number: 7}
	comments := 0
	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		comments++
		if r.Method != "POST" || r.URL.Path != "/repos/acme/infra/issues/7/comments" {
			t.Errorf("wrong destination: %s %s", r.Method, r.URL.Path)
		}
		logger.Info("mock.github.received", "method", r.Method, "path", r.URL.Path)
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"id":9001,"html_url":"https://github.example/acme/infra/pull/7#issuecomment-9001"}`)
	}))
	defer github.Close()
	sdk, err := gh.NewClient(gh.WithHTTPClient(github.Client()), gh.WithURLs(gh.Ptr(github.URL+"/"), nil))
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := githubadapter.NewSourceControl(sdk).PublishProposalComment(ctx, destination, summary, logger)
	if err != nil || comments != 1 || receipt.GetID() != 9001 {
		t.Fatal("comment publication", comments, err)
	}
	after, err := workflow.Get(ctx, review.Id)
	if err != nil || after.Version != evidence.Version || len(after.Decisions) != len(evidence.Decisions) {
		t.Fatal("publication changed authoritative review state", err)
	}
	t.Log("Actual local acceptance logs:\n" + strings.TrimSpace(logs.String()))
}
