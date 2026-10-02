package connectapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/lanej/statecraft/gen/statecraft/v1"
	"github.com/lanej/statecraft/gen/statecraft/v1/statecraftv1connect"
	"github.com/lanej/statecraft/internal/adapters/connectapi"
	"github.com/lanej/statecraft/internal/adapters/mock"
	"github.com/lanej/statecraft/internal/service"
)

func TestWorkflowLogsCommitAndRejectionWithoutBody(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	store := mock.NewReviewStore()
	workflow := service.NewDemoWorkflow(store, mock.Planner{}, mock.Policies{}, mock.Policies{}, time.Now)
	server := httptest.NewServer(connectapi.Handler(service.NewReviews(store), workflow, mock.SeedReview, connect.WithInterceptors(connectapi.Logging(logger))))
	defer server.Close()
	client := statecraftv1connect.NewDemoWorkflowServiceClient(server.Client(), server.URL)
	ctx := context.Background()
	response, err := client.CreateDemo(ctx, connect.NewRequest(&v1.CreateDemoRequest{Scenario: "ready"}))
	if err != nil {
		t.Fatal(err)
	}
	const private = "DO_NOT_LOG_RATIONALE_OR_RECOVERY_EVIDENCE"
	command := &v1.ActOnDemoRequest{ReviewId: response.Msg.Review.Id, Command: &v1.WorkflowCommand{Action: "approve", ExpectedVersion: response.Msg.Review.Version, Reason: private, Evidence: private}}
	if _, err = client.ActOnDemo(ctx, connect.NewRequest(command)); err != nil {
		t.Fatal(err)
	}
	if _, err = client.ActOnDemo(ctx, connect.NewRequest(command)); connect.CodeOf(err) != connect.CodeAborted {
		t.Fatal(err)
	}
	if strings.Contains(logs.String(), private) {
		t.Fatal("private command body leaked")
	}
	lines := strings.Split(strings.TrimSpace(logs.String()), "\n")
	if len(lines) != 3 {
		t.Fatal(logs.String())
	}
	var approved, rejected map[string]any
	if err = json.Unmarshal([]byte(lines[1]), &approved); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal([]byte(lines[2]), &rejected); err != nil {
		t.Fatal(err)
	}
	if approved["code"] != "ok" || approved["state"] != "approved" || approved["version"] != float64(2) || approved["plan_id"] != "plan-7" {
		t.Fatal(approved)
	}
	if rejected["code"] != "aborted" || rejected["action"] != "approve" {
		t.Fatal(rejected)
	}
	if _, exists := rejected["state"]; exists {
		t.Fatal("rejection logged a committed state", rejected)
	}
}
