package connectapi_test

import (
	"context"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/lanej/statecraft/gen/statecraft/v1"
	"github.com/lanej/statecraft/gen/statecraft/v1/statecraftv1connect"
	"github.com/lanej/statecraft/internal/adapters/connectapi"
	"github.com/lanej/statecraft/internal/adapters/mock"
	"github.com/lanej/statecraft/internal/domain"
	"github.com/lanej/statecraft/internal/ports"
	"github.com/lanej/statecraft/internal/service"
	"google.golang.org/protobuf/encoding/protojson"
)

type readStore struct {
	review domain.Review
	err    error
}

func (s readStore) GetReview(context.Context, string) (domain.Review, error) { return s.review, s.err }

func TestReadBoundary(t *testing.T) {
	workflow := service.NewDemoWorkflow(mock.NewReviewStore(), mock.Planner{}, mock.Policies{}, mock.Policies{}, time.Now)
	for _, codec := range []string{"protobuf", "json"} {
		t.Run(codec, func(t *testing.T) {
			r := domain.Review{ID: "exact", Version: math.MaxUint64, PullRequest: math.MaxInt64, SourceDecisions: []domain.ExternalReviewDecision{{Actor: "source-only-reviewer"}}}
			server := httptest.NewServer(connectapi.Handler(service.NewReviews(readStore{review: r}), workflow, mock.SeedReview))
			defer server.Close()
			opts := []connect.ClientOption{}
			if codec == "json" {
				opts = append(opts, connect.WithProtoJSON())
			}
			client := statecraftv1connect.NewReviewServiceClient(server.Client(), server.URL, opts...)
			response, err := client.GetReview(context.Background(), connect.NewRequest(&v1.GetReviewRequest{ReviewId: "exact"}))
			if err != nil {
				t.Fatal(err)
			}
			if response.Msg.Review.Version != math.MaxUint64 || response.Msg.Review.PullRequest != math.MaxInt64 || len(response.Msg.Review.Decisions) != 0 {
				t.Fatal("lost exact integers or turned source evidence into approval")
			}
		})
	}
	for _, tc := range []struct {
		err     error
		code    connect.Code
		message string
	}{
		{ports.ErrCapacity, connect.CodeUnavailable, ports.ErrCapacity.Error()},
		{context.Canceled, connect.CodeCanceled, "The request was canceled."},
		{context.DeadlineExceeded, connect.CodeDeadlineExceeded, "The request timed out."},
		{errors.New("private provider credential"), connect.CodeInternal, "The demo could not complete this action."},
	} {
		t.Run(tc.code.String(), func(t *testing.T) {
			server := httptest.NewServer(connectapi.Handler(service.NewReviews(readStore{err: tc.err}), workflow, mock.SeedReview))
			defer server.Close()
			client := statecraftv1connect.NewReviewServiceClient(server.Client(), server.URL)
			_, err := client.GetReview(context.Background(), connect.NewRequest(&v1.GetReviewRequest{ReviewId: "review"}))
			var rpc *connect.Error
			if !errors.As(err, &rpc) || rpc.Code() != tc.code || rpc.Message() != tc.message {
				t.Fatal(err)
			}
		})
	}
}

func TestRPCRequestBoundary(t *testing.T) {
	store := mock.NewReviewStore()
	workflow := service.NewDemoWorkflow(store, mock.Planner{}, mock.Policies{}, mock.Policies{}, time.Now)
	handler := connectapi.Handler(service.NewReviews(store), workflow, mock.SeedReview)
	path := statecraftv1connect.DemoWorkflowServiceCreateDemoProcedure
	for _, tc := range []struct {
		method, path, body, content, site string
		status                            int
	}{
		{"POST", path, `{"scenario":"ready"}`, "application/json", "cross-site", 403},
		{"POST", path, `{`, "application/json", "same-origin", 400},
		{"POST", path, `{"scenario":"ready"} {}`, "application/json", "same-origin", 400},
		{"POST", path, `scenario=ready`, "application/x-www-form-urlencoded", "same-origin", 415},
		{"DELETE", path, ``, "application/json", "same-origin", 405},
		{"POST", "/api/demos", `{"scenario":"ready"}`, "application/json", "same-origin", 404},
	} {
		t.Run(tc.method+tc.path+tc.site+tc.body, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			request.Header.Set("Content-Type", tc.content)
			request.Header.Set("Sec-Fetch-Site", tc.site)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != tc.status || response.Header().Get("Cache-Control") != "no-store" {
				t.Fatal(response.Code, response.Body.String())
			}
		})
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	client := statecraftv1connect.NewDemoWorkflowServiceClient(server.Client(), server.URL)
	_, err := client.CreateDemo(context.Background(), connect.NewRequest(&v1.CreateDemoRequest{Scenario: strings.Repeat("x", 20*1024)}))
	if connect.CodeOf(err) != connect.CodeResourceExhausted {
		t.Fatal("unbounded request", err)
	}
	// A client cannot claim actor authority; protobuf ignores unknown fields and
	// decisions still use the backend's fixed demo identity.
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"scenario":"ready","actor":"admin"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body.String())
	}
	var created v1.CreateDemoResponse
	if err := protojson.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	body := `{"reviewId":"` + created.Review.Id + `","command":{"action":"approve","expectedVersion":"1","actor":"admin","authorizedBy":"admin"}}`
	request = httptest.NewRequest(http.MethodPost, statecraftv1connect.DemoWorkflowServiceActOnDemoProcedure, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body.String())
	}
	var changed v1.ActOnDemoResponse
	if err := protojson.Unmarshal(response.Body.Bytes(), &changed); err != nil {
		t.Fatal(err)
	}
	if len(changed.Review.Decisions) != 1 || changed.Review.Decisions[0].Actor != "alex" {
		t.Fatal("untrusted actor claim changed identity")
	}
}
