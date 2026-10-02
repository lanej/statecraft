package domain

import (
	"testing"
	"time"
)

func TestProposalSummaryPreservesDecisionMeaning(t *testing.T) {
	now := time.Date(2026, 10, 2, 2, 0, 0, 0, time.UTC)
	input := ProposalSummaryInput{
		HeadSHA: "head", State: "approved", Simulation: true, Now: now, Verification: "not_started",
		Plan:        &PlanSnapshot{ID: "plan-7", CommitSHA: "head"},
		Roots:       []Root{{ID: "api", Status: "planned"}, {ID: "network", Status: "failed"}},
		Policy:      &PolicyEvaluation{ID: "eval-7", PlanSetID: "plan-7", Coverage: "incomplete", Violations: []PolicyViolation{{ID: "violation", PolicyVersion: "1", Blocking: true}}},
		Decisions:   []ReviewDecision{{Actor: "alex", Decision: "approved", PlanSetID: "plan-7", CommitSHA: "head", EvaluationID: "eval-7"}},
		Acceptances: []ViolationAcceptance{{ViolationID: "violation", PlanSetID: "plan-7", EvaluationID: "eval-7", PolicyVersion: "1", Status: "granted", AuthorizedBy: "owner", ExpiresAt: now.Add(time.Hour).Format(time.RFC3339)}},
	}
	summary := SummarizeProposal(input)
	if summary.PlannedRoots != 1 || summary.ExpectedRoots != 2 || summary.PolicyCoverage != "incomplete" || summary.BlockingViolations != 1 || summary.AcceptedViolations != 1 || !summary.CurrentApproval {
		t.Fatal(summary)
	}
	// An existing approval and accepted risk do not turn incomplete evidence into
	// coverage or remove the violation. Summary is informational, not eligibility.
	expired := input
	expired.Now = now.Add(2 * time.Hour)
	summary = SummarizeProposal(expired)
	if summary.AcceptedViolations != 0 || !summary.CurrentApproval || summary.BlockingViolations != 1 {
		t.Fatal("expiry changed approval history or erased violation", summary)
	}
	stale := input
	stale.HeadSHA = "new-head"
	summary = SummarizeProposal(stale)
	if summary.CurrentPlan || summary.CurrentApproval || summary.CommitSHA != "head" {
		t.Fatal("stale evidence misrepresented", summary)
	}
	unplanned := input
	unplanned.Plan = nil
	unplanned.Policy = nil
	summary = SummarizeProposal(unplanned)
	if summary.CurrentPlan || summary.CurrentApproval || summary.PlanID != "" || summary.PolicyCoverage != "missing" {
		t.Fatal("missing evidence invented", summary)
	}
	if input.Plan.ID != "plan-7" || input.Acceptances[0].Status != "granted" || input.Decisions[0].Decision != "approved" {
		t.Fatal("summary mutated evidence")
	}
}
