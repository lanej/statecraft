package domain

import "time"

// ProposalSummaryInput is the Statecraft evidence needed for an informational
// update. It has no repository, source-change number, destination, or SDK type.
type ProposalSummaryInput struct {
	HeadSHA      string
	State        string
	Plan         *PlanSnapshot
	Policy       *PolicyEvaluation
	Roots        []Root
	Decisions    []ReviewDecision
	Acceptances  []ViolationAcceptance
	Verification string
	Simulation   bool
	Now          time.Time
}

type ProposalSummary struct {
	PlanID             string
	CommitSHA          string
	State              string
	PlannedRoots       int
	ExpectedRoots      int
	CurrentPlan        bool
	PolicyCoverage     string
	BlockingViolations int
	AcceptedViolations int
	CurrentApproval    bool
	Verification       string
	Simulation         bool
}

// SummarizeProposal is pure. Its facts do not authorize execution, and publishing
// them does not record an approval or accept a violation.
func SummarizeProposal(input ProposalSummaryInput) ProposalSummary {
	summary := ProposalSummary{
		State: input.State, ExpectedRoots: len(input.Roots), Verification: input.Verification,
		PolicyCoverage: "missing", Simulation: input.Simulation,
	}
	for _, root := range input.Roots {
		if root.Status == "planned" {
			summary.PlannedRoots++
		}
	}
	if input.Plan != nil {
		summary.PlanID, summary.CommitSHA = input.Plan.ID, input.Plan.CommitSHA
		summary.CurrentPlan = input.Plan.ID != "" && input.HeadSHA != "" && input.Plan.CommitSHA == input.HeadSHA && input.State != "stale"
	}
	// Reuse the existing pure decision/acceptance rules over evidence only.
	evidence := Review{HeadSHA: input.HeadSHA, State: input.State, Plan: input.Plan, Policy: input.Policy, Decisions: input.Decisions, Acceptances: input.Acceptances}
	summary.CurrentApproval = evidence.CurrentApproval()
	if input.Policy != nil {
		summary.PolicyCoverage = input.Policy.Coverage
		for _, violation := range input.Policy.Violations {
			if violation.Blocking {
				summary.BlockingViolations++
				if evidence.AcceptanceFor(violation, input.Now) != nil {
					summary.AcceptedViolations++
				}
			}
		}
	}
	return summary
}
