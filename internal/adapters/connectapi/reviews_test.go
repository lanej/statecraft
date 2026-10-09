package connectapi_test

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/lanej/statecraft/gen/statecraft/v1"
	"github.com/lanej/statecraft/gen/statecraft/v1/statecraftv1connect"
	"github.com/lanej/statecraft/internal/adapters/connectapi"
	"github.com/lanej/statecraft/internal/adapters/mock"
	"github.com/lanej/statecraft/internal/service"
)

func TestGeneratedWorkflowClient(t *testing.T) {
	for _, codec := range []string{"protobuf", "json"} {
		t.Run(codec, func(t *testing.T) {
			store := mock.NewReviewStore()
			workflow := service.NewDemoWorkflow(store, mock.Planner{}, mock.Policies{}, mock.Policies{}, time.Now)
			server := httptest.NewServer(connectapi.Handler(service.NewReviews(store), workflow, mock.SeedReview))
			defer server.Close()
			opts := []connect.ClientOption{}
			if codec == "json" {
				opts = append(opts, connect.WithProtoJSON())
			}
			client := statecraftv1connect.NewDemoWorkflowServiceClient(server.Client(), server.URL, opts...)
			ctx := context.Background()
			create := func(scenario string) *v1.Review {
				t.Helper()
				response, err := client.CreateDemo(ctx, connect.NewRequest(&v1.CreateDemoRequest{Scenario: scenario}))
				if err != nil {
					t.Fatal(err)
				}
				r := response.Msg.Review
				if r == nil || !r.Demo || r.Version != 1 || r.Id == "pr-1842" {
					t.Fatal("session was not isolated", r)
				}
				if response.Header().Get("Cache-Control") != "no-store" {
					t.Fatal("RPC response may be cached")
				}
				return r
			}
			act := func(r *v1.Review, action, violation, reason, evidence string) (*v1.Review, error) {
				response, err := client.ActOnDemo(ctx, connect.NewRequest(&v1.ActOnDemoRequest{ReviewId: r.Id, Command: &v1.WorkflowCommand{Action: action, ExpectedVersion: r.Version, ViolationId: violation, Reason: reason, Evidence: evidence}}))
				if err != nil {
					return nil, err
				}
				return response.Msg.Review, nil
			}
			expectCode := func(err error, code connect.Code) {
				t.Helper()
				if err == nil || connect.CodeOf(err) != code {
					t.Fatalf("want %s, got %v", code, err)
				}
			}
			_, err := client.CreateDemo(ctx, connect.NewRequest(&v1.CreateDemoRequest{Scenario: "unknown"}))
			expectCode(err, connect.CodeInvalidArgument)
			_, err = client.GetDemo(ctx, connect.NewRequest(&v1.GetDemoRequest{ReviewId: "missing"}))
			expectCode(err, connect.CodeNotFound)
			_, err = client.GetDemo(ctx, connect.NewRequest(&v1.GetDemoRequest{}))
			expectCode(err, connect.CodeInvalidArgument)
			_, err = client.ActOnDemo(ctx, connect.NewRequest(&v1.ActOnDemoRequest{ReviewId: "missing"}))
			expectCode(err, connect.CodeInvalidArgument)
			_, err = client.ActOnDemo(ctx, connect.NewRequest(&v1.ActOnDemoRequest{ReviewId: "missing", Command: &v1.WorkflowCommand{Action: "invented", ExpectedVersion: 1}}))
			expectCode(err, connect.CodeInvalidArgument)

			r := create("review")
			if r.Plan == nil || r.Policy == nil || len(r.Changes) == 0 || len(r.Policy.Violations) == 0 || len(r.Changes[0].Properties) == 0 {
				t.Fatal("missing review evidence", r)
			}
			_, err = act(r, "approve", "", "", "")
			expectCode(err, connect.CodeFailedPrecondition)
			response, err := client.GetDemo(ctx, connect.NewRequest(&v1.GetDemoRequest{ReviewId: r.Id}))
			if err != nil || response.Msg.Review.Version != r.Version || len(response.Msg.Review.Decisions) != 0 {
				t.Fatal("denied action committed", err)
			}
			violation := r.Policy.Violations[0].Id
			r, err = act(r, "request_acceptance", violation, "Reviewed replacement", "Recovery drill")
			if err != nil {
				t.Fatal(err)
			}
			r, err = act(r, "grant_acceptance", violation, "", "")
			if err != nil {
				t.Fatal(err)
			}
			if r.Acceptances[0].Status != "granted" || r.Policy.Results[0].Outcome != "violated" || len(r.Decisions) != 0 {
				t.Fatal("acceptance became approval or compliance")
			}
			previous := r
			r, err = act(r, "approve", "", "Reviewed exact proposal", "")
			if err != nil {
				t.Fatal(err)
			}
			_, err = act(previous, "approve", "", "", "")
			expectCode(err, connect.CodeAborted)
			response, err = client.GetDemo(ctx, connect.NewRequest(&v1.GetDemoRequest{ReviewId: r.Id}))
			if err != nil || response.Msg.Review.Version != r.Version || len(response.Msg.Review.Decisions) != 1 {
				t.Fatal("stale action changed state", err)
			}
			r, err = act(r, "apply", "", "", "")
			if err != nil {
				t.Fatal(err)
			}
			if r.State != "applied" || r.Verification != "pending" {
				t.Fatal("execution treated as verification")
			}
			r, err = act(r, "verify", "", "", "")
			if err != nil || r.State != "verified" {
				t.Fatal("verification", err)
			}

			for _, scenario := range []string{"unplanned", "incomplete", "stale", "partial"} {
				r = create(scenario)
				_, err = act(r, "approve", "", "", "")
				expectCode(err, connect.CodeFailedPrecondition)
				before := r.Plan
				r, err = act(r, "plan", "", "", "")
				if err != nil || r.State != "ready_for_review" || r.Policy.Coverage != "complete" {
					t.Fatal("replanning", scenario, err)
				}
				if before != nil && (len(r.PlanHistory) == 0 || r.Plan.Id == before.Id) {
					t.Fatal("lost plan history", scenario)
				}
				for _, root := range r.Roots {
					if root.Status != "planned" {
						t.Fatal("incomplete scope", scenario)
					}
				}
				if scenario == "partial" {
					for _, c := range r.Changes {
						if c.RootId == "api" {
							t.Fatal("replanned applied root")
						}
					}
				}
			}
			r = create("expired")
			_, err = act(r, "apply", "", "", "")
			expectCode(err, connect.CodeFailedPrecondition)
			if len(r.Decisions) == 0 || r.Acceptances[0].Status != "expired" {
				t.Fatal("expiry lost original decision")
			}

			original := statecraftv1connect.NewReviewServiceClient(server.Client(), server.URL, opts...)
			fixture, err := original.GetReview(ctx, connect.NewRequest(&v1.GetReviewRequest{ReviewId: "pr-1842"}))
			if err != nil || fixture.Msg.Review.Id != "pr-1842" || fixture.Msg.Review.Demo {
				t.Fatal("legacy read-only fixture", err)
			}
		})
	}
}
