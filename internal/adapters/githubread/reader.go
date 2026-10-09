// Package githubread composes only GET operations from GitHub's official spec.
package githubread

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/lanej/statecraft/internal/adapters/githubread/generated"
	"github.com/lanej/statecraft/internal/domain"
	"github.com/lanej/statecraft/internal/ports"
)

var (
	ErrUnavailable = errors.New("GitHub source evidence is unavailable")
	ErrChanged     = errors.New("source changed during capture; refresh to try again")
	ErrIncomplete  = errors.New("GitHub source evidence exceeds the supported capture limit")
)

const maxPages = 30
const pageSize = 100

type Reader struct {
	client *generated.ClientWithResponses
	repo   domain.RepositoryRef
}

var _ ports.SourceEvidenceReader = (*Reader)(nil)

func New(token string) (*Reader, error) {
	return newReader("https://api.github.com", token, domain.RepositoryRef{Owner: "easypost", Name: "platform-infra"})
}

func newReader(base, token string, repo domain.RepositoryRef) (*Reader, error) {
	if strings.TrimSpace(token) == "" {
		return nil, errors.New("a repository-scoped read-only GitHub token is required")
	}
	httpClient := &http.Client{
		Timeout:       15 * time.Second,
		Transport:     readTransport{http.DefaultTransport},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	client, err := generated.NewClientWithResponses(base, generated.WithHTTPClient(httpClient), generated.WithRequestEditorFn(
		func(_ context.Context, req *http.Request) error {
			req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
			req.Header.Set("Accept", "application/vnd.github+json")
			req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
			return nil
		}))
	if err != nil {
		return nil, err
	}
	return &Reader{client: client, repo: repo}, nil
}

// A second boundary rejects mutation even if a future generated spec expands.
type readTransport struct{ next http.RoundTripper }

func (t readTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method != http.MethodGet {
		return nil, errors.New("GitHub writes are disabled")
	}
	response, err := t.next.RoundTrip(req)
	if err == nil {
		response.Body = limitedBody{Reader: io.LimitReader(response.Body, 8*1024*1024), Closer: response.Body}
	}
	return response, err
}

type limitedBody struct {
	io.Reader
	io.Closer
}

func ptr[T any](v T) *T { return &v }
func value[T any](v *T) (zero T) {
	if v != nil {
		return *v
	}
	return zero
}
func hasNext(response *http.Response) bool {
	return response != nil && strings.Contains(response.Header.Get("Link"), `rel="next"`)
}

func (r *Reader) ListChanges(ctx context.Context) (domain.SourceChangeList, error) {
	result := domain.SourceChangeList{Repository: r.repo}
	// A bounded recent list; explicitly disclose truncation.
	for page := 1; page <= 5; page++ {
		response, err := r.client.PullslistWithResponse(ctx, r.repo.Owner, r.repo.Name, &generated.PullslistParams{
			State:     ptr(generated.PullslistParamsState("open")),
			Sort:      ptr(generated.PullslistParamsSort("updated")),
			Direction: ptr(generated.PullslistParamsDirection("desc")),
			PerPage:   ptr(pageSize), Page: ptr(page),
		})
		if err != nil || response.JSON200 == nil {
			return domain.SourceChangeList{}, ErrUnavailable
		}
		for _, pr := range *response.JSON200 {
			author := ""
			if pr.User != nil {
				author = pr.User.Login
			}
			result.Changes = append(result.Changes, domain.SourceChange{
				Repository: r.repo, Number: int64(pr.Number), Title: pr.Title,
				Author: author, State: pr.State, Draft: value(pr.Draft),
				HeadSHA: pr.Head.Sha, BaseRef: pr.Base.Ref, URL: pr.HtmlUrl,
			})
		}
		result.Truncated = hasNext(response.HTTPResponse)
		if !result.Truncated {
			break
		}
	}
	result.CapturedAt = time.Now().UTC()
	return result, nil
}

func (r *Reader) pull(ctx context.Context, number int64) (*generated.PullRequest, error) {
	response, err := r.client.PullsgetWithResponse(ctx, r.repo.Owner, r.repo.Name, int(number))
	if err != nil || response.JSON200 == nil {
		return nil, ErrUnavailable
	}
	if response.JSON200.Number != int(number) || response.JSON200.Head.Sha == "" {
		return nil, ErrUnavailable
	}
	return response.JSON200, nil
}

func (r *Reader) GetSourceEvidence(ctx context.Context, number int64) (domain.SourceEvidence, error) {
	pr, err := r.pull(ctx, number)
	if err != nil {
		return domain.SourceEvidence{}, err
	}
	if pr.ChangedFiles > pageSize*maxPages {
		return domain.SourceEvidence{}, ErrIncomplete
	}
	state := string(pr.State)
	if pr.Merged {
		state = "merged"
	}
	result := domain.SourceEvidence{Change: domain.SourceChange{
		Repository: r.repo, Number: int64(pr.Number), Title: pr.Title, Author: pr.User.Login,
		State: state, Draft: value(pr.Draft), HeadSHA: pr.Head.Sha, BaseRef: pr.Base.Ref, URL: pr.HtmlUrl,
	}}
	capturedBytes := 0
	for page := 1; page <= maxPages; page++ {
		response, err := r.client.PullslistFilesWithResponse(ctx, r.repo.Owner, r.repo.Name, int(number),
			&generated.PullslistFilesParams{PerPage: ptr(pageSize), Page: ptr(page)})
		if err != nil || response.JSON200 == nil {
			return domain.SourceEvidence{}, ErrUnavailable
		}
		for _, file := range *response.JSON200 {
			capturedBytes += len(file.Filename) + len(value(file.PreviousFilename)) + len(value(file.Patch))
			if capturedBytes > 4*1024*1024 {
				return domain.SourceEvidence{}, ErrIncomplete
			}
			result.Files = append(result.Files, domain.SourceFile{
				ChangedFile: domain.ChangedFile{Path: file.Filename, PreviousPath: value(file.PreviousFilename),
					Status: string(file.Status), Additions: file.Additions, Deletions: file.Deletions},
				Patch: value(file.Patch), PatchAvailable: file.Patch != nil,
			})
		}
		if !hasNext(response.HTTPResponse) {
			break
		}
		if page == maxPages {
			return domain.SourceEvidence{}, ErrIncomplete
		}
	}
	if len(result.Files) != pr.ChangedFiles {
		return domain.SourceEvidence{}, ErrChanged
	}
	for page := 1; page <= maxPages; page++ {
		response, err := r.client.PullslistReviewsWithResponse(ctx, r.repo.Owner, r.repo.Name, int(number),
			&generated.PullslistReviewsParams{PerPage: ptr(pageSize), Page: ptr(page)})
		if err != nil || response.JSON200 == nil {
			return domain.SourceEvidence{}, ErrUnavailable
		}
		for _, review := range *response.JSON200 {
			if review.SubmittedAt == nil || review.State == "PENDING" {
				continue
			}
			actor := ""
			if review.User != nil {
				actor = review.User.Login
			}
			result.Reviews = append(result.Reviews, domain.ExternalReviewDecision{
				ID: fmt.Sprint(review.Id), Actor: actor, Decision: strings.ToLower(review.State),
				CommitSHA: value(review.CommitId), CreatedAt: *review.SubmittedAt, URL: review.HtmlUrl, Source: "github",
			})
		}
		if !hasNext(response.HTTPResponse) {
			break
		}
		if page == maxPages {
			return domain.SourceEvidence{}, ErrIncomplete
		}
	}
	checks, err := r.client.CheckslistForRefWithResponse(ctx, r.repo.Owner, r.repo.Name, pr.Head.Sha,
		&generated.CheckslistForRefParams{PerPage: ptr(pageSize), Filter: ptr(generated.CheckslistForRefParamsFilter("latest"))})
	if err != nil || checks.JSON200 == nil || checks.JSON200.CheckRuns == nil {
		return domain.SourceEvidence{}, ErrUnavailable
	}
	for _, check := range checks.JSON200.CheckRuns {
		if check.HeadSha != pr.Head.Sha {
			return domain.SourceEvidence{}, ErrChanged
		}
		result.Checks = append(result.Checks, domain.SourceCheck{
			Name: check.Name, Status: string(check.Status), Conclusion: string(value(check.Conclusion)),
			URL: value(check.HtmlUrl), EvidenceSource: "GitHub check run",
		})
	}
	result.ChecksTruncated = hasNext(checks.HTTPResponse) || checks.JSON200.TotalCount > len(result.Checks)
	statuses, err := r.client.ReposgetCombinedStatusForRefWithResponse(ctx, r.repo.Owner, r.repo.Name, pr.Head.Sha,
		&generated.ReposgetCombinedStatusForRefParams{PerPage: ptr(pageSize)})
	if err != nil || statuses.JSON200 == nil || statuses.JSON200.Statuses == nil {
		return domain.SourceEvidence{}, ErrUnavailable
	}
	if statuses.JSON200.Sha != pr.Head.Sha {
		return domain.SourceEvidence{}, ErrChanged
	}
	for _, status := range statuses.JSON200.Statuses {
		result.Checks = append(result.Checks, domain.SourceCheck{
			Name: status.Context, Status: status.State, URL: value(status.TargetUrl), EvidenceSource: "GitHub commit status",
		})
	}
	result.ChecksTruncated = result.ChecksTruncated || hasNext(statuses.HTTPResponse) || statuses.JSON200.TotalCount > len(statuses.JSON200.Statuses)
	current, err := r.pull(ctx, number)
	if err != nil {
		return domain.SourceEvidence{}, err
	}
	// Both sides matter: files can change when the base branch advances.
	if current.Head.Sha != pr.Head.Sha || current.Base.Sha != pr.Base.Sha ||
		current.ChangedFiles != pr.ChangedFiles || current.State != pr.State || current.Merged != pr.Merged {
		return domain.SourceEvidence{}, ErrChanged
	}
	result.CapturedAt = time.Now().UTC()
	return result, nil
}
