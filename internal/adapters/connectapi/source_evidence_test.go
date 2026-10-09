package connectapi

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/lanej/statecraft/gen/statecraft/v1"
	"github.com/lanej/statecraft/gen/statecraft/v1/statecraftv1connect"
	"github.com/lanej/statecraft/internal/adapters/githubread"
	"github.com/lanej/statecraft/internal/domain"
)

type evidenceReader struct {
	calls int
	err   error
}

func (r *evidenceReader) ListChanges(context.Context) (domain.SourceChangeList, error) {
	return domain.SourceChangeList{Repository: domain.RepositoryRef{Owner: "easypost", Name: "platform-infra"}}, r.err
}
func (r *evidenceReader) GetSourceEvidence(context.Context, int64) (domain.SourceEvidence, error) {
	r.calls++
	return domain.SourceEvidence{
		Change:     domain.SourceChange{Number: 12, HeadSHA: "head"},
		Files:      []domain.SourceFile{{ChangedFile: domain.ChangedFile{Path: "main.tf"}, Patch: "source", PatchAvailable: true}},
		Reviews:    []domain.ExternalReviewDecision{{Decision: "approved", CommitSHA: "old"}},
		Checks:     []domain.SourceCheck{{Name: "atlantis/plan", Status: "success", EvidenceSource: "GitHub commit status"}},
		CapturedAt: time.Unix(0, 0).UTC(),
	}, r.err
}
func TestSourceAPIContractAndValidation(t *testing.T) {
	reader := &evidenceReader{}
	_, handler := SourceHandler(reader)
	server := httptest.NewServer(handler)
	defer server.Close()
	client := statecraftv1connect.NewSourceEvidenceServiceClient(server.Client(), server.URL)
	response, err := client.GetSourceEvidence(context.Background(), connect.NewRequest(&v1.GetSourceEvidenceRequest{PullRequest: 12}))
	if err != nil {
		t.Fatal(err)
	}
	if response.Msg.SourceChange.HeadSha != "head" || response.Msg.SourceReviews[0].CommitSha != "old" ||
		response.Msg.Checks[0].EvidenceSource != "GitHub commit status" || !response.Msg.Files[0].PatchAvailable {
		t.Fatalf("mapping: %+v", response.Msg)
	}
	_, err = client.GetSourceEvidence(context.Background(), connect.NewRequest(&v1.GetSourceEvidenceRequest{PullRequest: 0}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument || reader.calls != 1 {
		t.Fatalf("invalid request reached reader: %v, %d", err, reader.calls)
	}
	for _, test := range []struct {
		err  error
		code connect.Code
	}{
		{githubread.ErrChanged, connect.CodeAborted},
		{githubread.ErrIncomplete, connect.CodeResourceExhausted},
		{errors.New("private diagnostic"), connect.CodeUnavailable},
	} {
		reader.err = test.err
		_, err = client.GetSourceEvidence(context.Background(), connect.NewRequest(&v1.GetSourceEvidenceRequest{PullRequest: 12}))
		if connect.CodeOf(err) != test.code {
			t.Fatalf("error mapping: %v", err)
		}
		if test.code == connect.CodeUnavailable && strings.Contains(err.Error(), "private diagnostic") {
			t.Fatal("provider diagnostic leaked")
		}
	}
}
