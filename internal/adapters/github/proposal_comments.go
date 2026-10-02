package githubadapter

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	gh "github.com/google/go-github/v92/github"
	"github.com/lanej/statecraft/internal/domain"
)

// PullRequestTarget belongs to the GitHub integration, not the domain. The
// application shell selects this destination from trusted configuration/context.
type PullRequestTarget struct {
	Owner      string
	Repository string
	Number     int
}

// PublishProposalComment is an explicit GitHub capability. It does not expand
// the SourceControl port or pretend a GitHub conversation is a domain approval.
// A failed or uncertain write is returned to the caller; it is never retried here.
func (s *SourceControl) PublishProposalComment(ctx context.Context, target PullRequestTarget, summary domain.ProposalSummary, logger *slog.Logger) (*gh.IssueComment, error) {
	if strings.TrimSpace(target.Owner) == "" || strings.TrimSpace(target.Repository) == "" || target.Number <= 0 {
		return nil, fmt.Errorf("publish GitHub proposal comment: explicit destination required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	attrs := []slog.Attr{slog.String("repository", target.Owner+"/"+target.Repository), slog.Int("pull_request", target.Number), slog.String("plan_id", summary.PlanID)}
	comment, response, err := s.client.Issues.CreateComment(ctx, target.Owner, target.Repository, target.Number, gh.IssueCommentRequest{Body: RenderProposalComment(summary)})
	if err != nil {
		outcome := "uncertain"
		if response != nil {
			attrs = append(attrs, slog.Int("http_status", response.StatusCode))
			if response.StatusCode >= 400 && response.StatusCode < 500 {
				outcome = "rejected"
			}
		}
		logger.LogAttrs(ctx, slog.LevelWarn, "github.comment.completed", append(attrs, slog.String("outcome", outcome))...)
		return nil, fmt.Errorf("publish GitHub proposal comment: %w", err)
	}
	if comment == nil || comment.GetID() == 0 || comment.GetHTMLURL() == "" {
		logger.LogAttrs(ctx, slog.LevelWarn, "github.comment.completed", append(attrs, slog.String("outcome", "uncertain"))...)
		return nil, fmt.Errorf("publish GitHub proposal comment: missing publication receipt; reconcile before retrying")
	}
	logger.LogAttrs(ctx, slog.LevelInfo, "github.comment.completed", append(attrs, slog.String("outcome", "published"), slog.Int64("comment_id", comment.GetID()))...)
	return comment, nil
}

// Markdown is a GitHub presentation choice; the core returns structured facts.
func RenderProposalComment(summary domain.ProposalSummary) string {
	literal := func(value string) string {
		return strings.NewReplacer("\\", "\\\\", "`", "\\`", "*", "\\*", "_", "\\_", "[", "\\[", "]", "\\]", "<", "&lt;", ">", "&gt;", "\r", " ", "\n", " ").Replace(value)
	}
	mode := "Informational evidence summary."
	if summary.Simulation {
		mode = "**Simulation:** synthetic evidence; no live infrastructure was changed."
	}
	body := fmt.Sprintf("### Statecraft proposal update\n\n%s\n\n- Proposal: %s at %s\n- Workflow state: %s\n- Current plan revision: %t\n- Roots planned: %d/%d\n- Policy coverage: %s\n- Blocking violations: %d (%d with active acceptance)\n- Current plan approval recorded: %t\n- Resulting-state verification: %s\n\nThis comment is informational. It does not approve a plan, accept a violation, or authorize execution.", mode, literal(summary.PlanID), literal(summary.CommitSHA), literal(summary.State), summary.CurrentPlan, summary.PlannedRoots, summary.ExpectedRoots, literal(summary.PolicyCoverage), summary.BlockingViolations, summary.AcceptedViolations, summary.CurrentApproval, literal(summary.Verification))
	return body
}
