package connectapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/lanej/statecraft/gen/statecraft/v1"
	"github.com/lanej/statecraft/gen/statecraft/v1/statecraftv1connect"
	"github.com/lanej/statecraft/internal/adapters/githubread"
	"github.com/lanej/statecraft/internal/domain"
	"github.com/lanej/statecraft/internal/ports"
)

type sourceServer struct{ reader ports.SourceEvidenceReader }

func SourceHandler(reader ports.SourceEvidenceReader, options ...connect.HandlerOption) (string, http.Handler) {
	options = append(options, connect.WithReadMaxBytes(1024))
	return statecraftv1connect.NewSourceEvidenceServiceHandler(&sourceServer{reader}, options...)
}

func sourceError(err error) error {
	if errors.Is(err, githubread.ErrChanged) {
		return connect.NewError(connect.CodeAborted, err)
	}
	if errors.Is(err, githubread.ErrIncomplete) {
		return connect.NewError(connect.CodeResourceExhausted, err)
	}
	// Never expose provider response bodies or credential/transport diagnostics.
	return connect.NewError(connect.CodeUnavailable, githubread.ErrUnavailable)
}

func (s *sourceServer) ListChanges(ctx context.Context, _ *connect.Request[v1.ListChangesRequest]) (*connect.Response[v1.ListChangesResponse], error) {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	result, err := s.reader.ListChanges(ctx)
	if err != nil {
		return nil, sourceError(err)
	}
	message := &v1.ListChangesResponse{Repository: result.Repository.FullName(), Truncated: result.Truncated, CapturedAt: result.CapturedAt.Format(time.RFC3339)}
	for _, change := range result.Changes {
		message.SourceChanges = append(message.SourceChanges, sourceChangeMessage(change))
	}
	return connect.NewResponse(message), nil
}

func (s *sourceServer) GetSourceEvidence(ctx context.Context, request *connect.Request[v1.GetSourceEvidenceRequest]) (*connect.Response[v1.GetSourceEvidenceResponse], error) {
	if request.Msg.PullRequest < 1 || request.Msg.PullRequest > 2147483647 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("a valid pull request number is required"))
	}
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	result, err := s.reader.GetSourceEvidence(ctx, request.Msg.PullRequest)
	if err != nil {
		return nil, sourceError(err)
	}
	message := &v1.GetSourceEvidenceResponse{SourceChange: sourceChangeMessage(result.Change), CapturedAt: result.CapturedAt.Format(time.RFC3339), ChecksTruncated: result.ChecksTruncated}
	for _, file := range result.Files {
		message.Files = append(message.Files, &v1.SourceFile{Path: file.Path, PreviousPath: file.PreviousPath, Status: file.Status,
			Additions: int64(file.Additions), Deletions: int64(file.Deletions), Patch: file.Patch, PatchAvailable: file.PatchAvailable})
	}
	for _, review := range result.Reviews {
		message.SourceReviews = append(message.SourceReviews, &v1.SourceReview{Actor: review.Actor, Decision: review.Decision,
			CommitSha: review.CommitSHA, CreatedAt: review.CreatedAt.Format(time.RFC3339), Url: review.URL})
	}
	for _, check := range result.Checks {
		message.Checks = append(message.Checks, &v1.SourceCheck{Name: check.Name, Status: check.Status, Conclusion: check.Conclusion, Url: check.URL, EvidenceSource: check.EvidenceSource})
	}
	return connect.NewResponse(message), nil
}

func sourceChangeMessage(change domain.SourceChange) *v1.SourceChange {
	return &v1.SourceChange{Repository: change.Repository.FullName(), Number: change.Number, Title: change.Title, Author: change.Author,
		HeadSha: change.HeadSHA, BaseRef: change.BaseRef, State: change.State, Draft: change.Draft, Url: change.URL}
}
