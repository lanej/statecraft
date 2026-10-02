// Package connectapi maps Statecraft models to the generated public RPC contract.
package connectapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"
	statecraftv1 "github.com/lanej/statecraft/gen/statecraft/v1"
	"github.com/lanej/statecraft/gen/statecraft/v1/statecraftv1connect"
	"github.com/lanej/statecraft/internal/domain"
	"github.com/lanej/statecraft/internal/ports"
	"github.com/lanej/statecraft/internal/service"
)

type reviewServer struct{ reviews *service.Reviews }
type demoServer struct {
	workflow *service.DemoWorkflow
	seed     func(string, time.Time) (domain.Review, error)
}

// Handler composes only the read service and isolated demo workflow. It has no
// source-control credentials or external planning/execution capability.
func Handler(reviews *service.Reviews, workflow *service.DemoWorkflow, seed func(string, time.Time) (domain.Review, error)) http.Handler {
	mux := http.NewServeMux()
	opts := []connect.HandlerOption{connect.WithReadMaxBytes(16 * 1024)}
	path, handler := statecraftv1connect.NewReviewServiceHandler(&reviewServer{reviews}, opts...)
	mux.Handle(path, handler)
	path, handler = statecraftv1connect.NewDemoWorkflowServiceHandler(&demoServer{workflow, seed}, opts...)
	mux.Handle(path, handler)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		// Keep the unauthenticated demo loopback-only and do not enable CORS.
		if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
			http.Error(w, "cross-site demo request rejected", http.StatusForbidden)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func validID(id string) bool {
	return id != "" && strings.TrimSpace(id) == id && len(id) <= 256
}

func (s *reviewServer) GetReview(ctx context.Context, req *connect.Request[statecraftv1.GetReviewRequest]) (*connect.Response[statecraftv1.GetReviewResponse], error) {
	if !validID(req.Msg.ReviewId) {
		return nil, rpcError(service.ErrInvalid)
	}
	r, err := s.reviews.Get(ctx, req.Msg.ReviewId)
	if err != nil {
		return nil, rpcError(err)
	}
	return connect.NewResponse(&statecraftv1.GetReviewResponse{Review: reviewMessage(r)}), nil
}

func (s *demoServer) CreateDemo(ctx context.Context, req *connect.Request[statecraftv1.CreateDemoRequest]) (*connect.Response[statecraftv1.CreateDemoResponse], error) {
	r, err := s.seed(req.Msg.Scenario, time.Now())
	if err != nil {
		return nil, rpcError(service.ErrInvalid)
	}
	r, err = s.workflow.Create(ctx, r)
	if err != nil {
		return nil, rpcError(err)
	}
	return connect.NewResponse(&statecraftv1.CreateDemoResponse{Review: reviewMessage(r)}), nil
}

func (s *demoServer) GetDemo(ctx context.Context, req *connect.Request[statecraftv1.GetDemoRequest]) (*connect.Response[statecraftv1.GetDemoResponse], error) {
	if !validID(req.Msg.ReviewId) {
		return nil, rpcError(service.ErrInvalid)
	}
	r, err := s.workflow.Get(ctx, req.Msg.ReviewId)
	if err != nil {
		return nil, rpcError(err)
	}
	return connect.NewResponse(&statecraftv1.GetDemoResponse{Review: reviewMessage(r)}), nil
}

func (s *demoServer) ActOnDemo(ctx context.Context, req *connect.Request[statecraftv1.ActOnDemoRequest]) (*connect.Response[statecraftv1.ActOnDemoResponse], error) {
	c := req.Msg.Command
	if !validID(req.Msg.ReviewId) || c == nil || c.ExpectedVersion == 0 {
		return nil, rpcError(service.ErrInvalid)
	}
	action := domain.ReviewAction(c.Action)
	switch action {
	case domain.ActionPlan, domain.ActionApprove, domain.ActionRequestChanges, domain.ActionRequestAcceptance, domain.ActionGrantAcceptance, domain.ActionApply, domain.ActionVerify:
	default:
		return nil, rpcError(service.ErrInvalid)
	}
	r, err := s.workflow.Act(ctx, req.Msg.ReviewId, domain.WorkflowCommand{
		Action: action, ExpectedVersion: c.ExpectedVersion, ViolationID: c.ViolationId, Reason: c.Reason, Evidence: c.Evidence,
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return connect.NewResponse(&statecraftv1.ActOnDemoResponse{Review: reviewMessage(r)}), nil
}

func rpcError(err error) error {
	code, message := connect.CodeInternal, "The demo could not complete this action."
	switch {
	case errors.Is(err, ports.ErrNotFound):
		code, message = connect.CodeNotFound, err.Error()
	case errors.Is(err, ports.ErrConflict):
		code, message = connect.CodeAborted, err.Error()
	case errors.Is(err, service.ErrInvalid):
		code, message = connect.CodeInvalidArgument, err.Error()
	case errors.Is(err, service.ErrDenied):
		code, message = connect.CodeFailedPrecondition, err.Error()
	case errors.Is(err, ports.ErrCapacity):
		code, message = connect.CodeUnavailable, err.Error()
	case errors.Is(err, context.Canceled):
		code, message = connect.CodeCanceled, "The request was canceled."
	case errors.Is(err, context.DeadlineExceeded):
		code, message = connect.CodeDeadlineExceeded, "The request timed out."
	}
	return connect.NewError(code, errors.New(message))
}
