package connectapi

import (
	statecraftv1 "github.com/lanej/statecraft/gen/statecraft/v1"
	"github.com/lanej/statecraft/internal/domain"
)

// Explicit mapping keeps protobuf and other transport types out of the domain.
// SourceDecisions are deliberately private source evidence, not plan approvals.
func reviewMessage(r domain.Review) *statecraftv1.Review {
	m := &statecraftv1.Review{
		Id:           r.ID,
		Repository:   r.Repository,
		PullRequest:  r.PullRequest,
		Title:        r.Title,
		HeadSha:      r.HeadSHA,
		State:        r.State,
		Demo:         r.Demo,
		Version:      r.Version,
		Scenario:     r.Scenario,
		Verification: r.Verification,
	}
	if r.Plan != nil {
		m.Plan = planSnapshotMessage(*r.Plan)
	}
	if r.Policy != nil {
		m.Policy = policyEvaluationMessage(*r.Policy)
	}
	for _, v := range r.Roots {
		m.Roots = append(m.Roots, rootMessage(v))
	}
	for _, v := range r.Changes {
		m.Changes = append(m.Changes, changeMessage(v))
	}
	for _, v := range r.Findings {
		m.Findings = append(m.Findings, findingMessage(v))
	}
	for _, v := range r.Decisions {
		m.Decisions = append(m.Decisions, reviewDecisionMessage(v))
	}
	for _, v := range r.Acceptances {
		m.Acceptances = append(m.Acceptances, violationAcceptanceMessage(v))
	}
	for _, v := range r.Attempts {
		m.Attempts = append(m.Attempts, executionRecordMessage(v))
	}
	for _, v := range r.History {
		m.History = append(m.History, reviewEventMessage(v))
	}
	for _, v := range r.PlanHistory {
		m.PlanHistory = append(m.PlanHistory, planSnapshotMessage(v))
	}
	for _, v := range r.Actions {
		m.Actions = append(m.Actions, actionDecisionMessage(v))
	}
	return m
}

func rootMessage(v domain.Root) *statecraftv1.Root {
	return &statecraftv1.Root{
		Id:          v.ID,
		Name:        v.Name,
		Status:      v.Status,
		ApplyStatus: v.ApplyStatus,
		Log:         v.Log,
	}
}

func findingMessage(v domain.Finding) *statecraftv1.Finding {
	return &statecraftv1.Finding{
		Id:              v.ID,
		Severity:        v.Severity,
		Category:        v.Category,
		Title:           v.Title,
		ResourceAddress: v.ResourceAddress,
		Blocking:        v.Blocking,
	}
}

func reviewDecisionMessage(v domain.ReviewDecision) *statecraftv1.ReviewDecision {
	return &statecraftv1.ReviewDecision{
		Actor:        v.Actor,
		Decision:     v.Decision,
		PlanSetId:    v.PlanSetID,
		CreatedAt:    v.CreatedAt,
		CommitSha:    v.CommitSHA,
		ExternalId:   v.ExternalID,
		Source:       v.Source,
		EvaluationId: v.EvaluationID,
		Message:      v.Message,
	}
}

func propertyChangeMessage(v domain.PropertyChange) *statecraftv1.PropertyChange {
	return &statecraftv1.PropertyChange{
		Name:   v.Name,
		Before: v.Before,
		After:  v.After,
	}
}

func resourceRelationshipMessage(v domain.ResourceRelationship) *statecraftv1.ResourceRelationship {
	return &statecraftv1.ResourceRelationship{
		ResourceId: v.ResourceID,
		Label:      v.Label,
		Kind:       v.Kind,
		Evidence:   v.Evidence,
	}
}

func planSnapshotMessage(v domain.PlanSnapshot) *statecraftv1.PlanSnapshot {
	return &statecraftv1.PlanSnapshot{
		Id:        v.ID,
		Number:    int32(v.Number),
		CommitSha: v.CommitSHA,
		Digest:    v.Digest,
		RootIds:   append([]string(nil), v.RootIDs...),
		CreatedAt: v.CreatedAt,
		ChangeIds: append([]string(nil), v.ChangeIDs...),
	}
}

func policyResultMessage(v domain.PolicyResult) *statecraftv1.PolicyResult {
	return &statecraftv1.PolicyResult{
		Id:      v.ID,
		Version: v.Version,
		Title:   v.Title,
		Outcome: v.Outcome,
	}
}

func policyViolationMessage(v domain.PolicyViolation) *statecraftv1.PolicyViolation {
	return &statecraftv1.PolicyViolation{
		Id:                v.ID,
		PolicyId:          v.PolicyID,
		PolicyVersion:     v.PolicyVersion,
		ResourceId:        v.ResourceID,
		Title:             v.Title,
		Severity:          v.Severity,
		Blocking:          v.Blocking,
		AcceptanceAllowed: v.AcceptanceAllowed,
		Explanation:       v.Explanation,
		Consequence:       v.Consequence,
		Unknown:           v.Unknown,
		Evidence:          append([]string(nil), v.Evidence...),
	}
}

func violationAcceptanceMessage(v domain.ViolationAcceptance) *statecraftv1.ViolationAcceptance {
	return &statecraftv1.ViolationAcceptance{
		Id:            v.ID,
		ViolationId:   v.ViolationID,
		PlanSetId:     v.PlanSetID,
		EvaluationId:  v.EvaluationID,
		PolicyVersion: v.PolicyVersion,
		Status:        v.Status,
		RequestedBy:   v.RequestedBy,
		AuthorizedBy:  v.AuthorizedBy,
		Reason:        v.Reason,
		Evidence:      v.Evidence,
		CreatedAt:     v.CreatedAt,
		ExpiresAt:     v.ExpiresAt,
	}
}

func executionRecordMessage(v domain.ExecutionRecord) *statecraftv1.ExecutionRecord {
	return &statecraftv1.ExecutionRecord{
		Id:        v.ID,
		PlanSetId: v.PlanSetID,
		RootId:    v.RootID,
		Operation: v.Operation,
		Status:    v.Status,
		CreatedAt: v.CreatedAt,
		Log:       v.Log,
	}
}

func reviewEventMessage(v domain.ReviewEvent) *statecraftv1.ReviewEvent {
	return &statecraftv1.ReviewEvent{
		Title:     v.Title,
		Detail:    v.Detail,
		CreatedAt: v.CreatedAt,
	}
}

func actionDecisionMessage(v domain.ActionDecision) *statecraftv1.ActionDecision {
	return &statecraftv1.ActionDecision{
		Action:  string(v.Action),
		Outcome: v.Outcome,
		Reason:  v.Reason,
	}
}

func changeMessage(v domain.Change) *statecraftv1.Change {
	m := &statecraftv1.Change{
		Id:           v.ID,
		RootId:       v.RootID,
		Address:      v.Address,
		ResourceType: v.ResourceType,
		Action:       v.Action,
		Risk:         v.Risk,
		Summary:      v.Summary,
		Name:         v.Name,
		PlanText:     v.PlanText,
		SourcePath:   v.SourcePath,
		SourceText:   v.SourceText,
	}
	for _, item := range v.Properties {
		m.Properties = append(m.Properties, propertyChangeMessage(item))
	}
	for _, item := range v.Relationships {
		m.Relationships = append(m.Relationships, resourceRelationshipMessage(item))
	}
	return m
}

func policyEvaluationMessage(v domain.PolicyEvaluation) *statecraftv1.PolicyEvaluation {
	m := &statecraftv1.PolicyEvaluation{
		Id:          v.ID,
		PlanSetId:   v.PlanSetID,
		PolicySetId: v.PolicySetID,
		Status:      v.Status,
		Coverage:    v.Coverage,
		EvaluatedAt: v.EvaluatedAt,
	}
	for _, item := range v.Results {
		m.Results = append(m.Results, policyResultMessage(item))
	}
	for _, item := range v.Violations {
		m.Violations = append(m.Violations, policyViolationMessage(item))
	}
	return m
}
