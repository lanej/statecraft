package service

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/lanej/statecraft/internal/domain"
	"github.com/lanej/statecraft/internal/ports"
)

// ProposedPlan retains source identity and command evidence together. Complete
// means all configured roots produced plans for an unchanged source revision;
// it does not establish artifact identity, policy compliance, or approval.
type ProposedPlan struct {
	Source domain.SourceReviewSnapshot
	PlannedHeadSHA string
	Run domain.PlanRun
	Complete bool
	Stale bool
}

type ProposedPlans struct {
	source ports.SourceControl
	planner ports.Planner
}

func NewProposedPlans(source ports.SourceControl, planner ports.Planner) *ProposedPlans {
	return &ProposedPlans{source: source, planner: planner}
}

// Plan uses explicit roots from trusted configuration; changed filenames alone
// cannot prove the complete infrastructure scope. No execution capability is used.
func (s *ProposedPlans) Plan(ctx context.Context, repo domain.RepositoryRef, number int64, roots []domain.RootSelector) (ProposedPlan, error) {
	if len(roots) == 0 {
		return ProposedPlan{}, fmt.Errorf("plan proposed change: configured roots are required")
	}
	ids := make(map[string]bool, len(roots))
	for _, root := range roots {
		if strings.TrimSpace(root.ID) == "" || ids[root.ID] {
			return ProposedPlan{}, fmt.Errorf("plan proposed change: distinct configured root IDs are required")
		}
		ids[root.ID] = true
	}
	snapshot, err := NewSourceReviews(s.source).Load(ctx, repo, number)
	if err != nil {
		return ProposedPlan{}, err
	}
	change := snapshot.Change
	if change.State != "open" || strings.TrimSpace(change.HeadSHA) == "" || strings.TrimSpace(change.BaseRef) == "" {
		return ProposedPlan{}, fmt.Errorf("plan proposed change: open source change with head SHA and base ref required")
	}
	result := ProposedPlan{Source: snapshot, PlannedHeadSHA: change.HeadSHA}
	run, err := s.planner.Plan(ctx, domain.PlanRequest{
		Repository: repo, PullRequest: number, Ref: change.HeadSHA,
		BaseBranch: change.BaseRef, Roots: slices.Clone(roots),
	})
	if err != nil {
		return result, fmt.Errorf("plan proposed change: %w", err)
	}
	result.Run = run
	current, err := s.source.GetChange(ctx, repo, number)
	if err != nil {
		result.Stale = true
		return result, fmt.Errorf("recheck planned source change: %w", err)
	}
	result.Stale = current.HeadSHA != change.HeadSHA || current.BaseRef != change.BaseRef || current.State != "open"
	result.Complete = !result.Stale && completePlan(run, roots)
	return result, nil
}

func completePlan(run domain.PlanRun, roots []domain.RootSelector) bool {
	if run.ErrorPresent || run.Failure != "" || run.PlansDiscarded {
		return false
	}
	expected := make(map[string]bool, len(roots))
	for _, root := range roots {
		if expected[root.StableID()] { return false }
		expected[root.StableID()] = true
	}
	seen := make(map[string]bool, len(roots))
	for _, attempt := range run.Attempts {
		if !expected[attempt.RootID] || attempt.Status != "succeeded" || attempt.ErrorPresent || attempt.Failure != "" {
			return false
		}
		if attempt.Phase == "policy_check" { continue }
		if attempt.Phase != "plan" || seen[attempt.RootID] { return false }
		seen[attempt.RootID] = true
	}
	return len(seen) == len(expected)
}
