package service

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/lanej/statecraft/internal/domain"
)

func TestPlanningCompletenessSeparatesPolicyOutcomes(t *testing.T) {
	roots := []domain.RootSelector{{ID: "root"}}
	plan := domain.PlanAttempt{RootID: "root", Phase: "plan", Status: "succeeded"}
	policy := domain.PlanAttempt{RootID: "root", Phase: "policy_check", Status: "failed", ErrorPresent: true, Failure: "violated policy"}
	if !completePlan(domain.PlanRun{Attempts: []domain.PlanAttempt{policy, plan}}, roots) {
		t.Fatal("failed policy check hid complete planning evidence")
	}
	if completePlan(domain.PlanRun{Attempts: []domain.PlanAttempt{policy}}, roots) {
		t.Fatal("policy attempt filled a missing plan")
	}
	plan.Status = "failed"
	if completePlan(domain.PlanRun{Attempts: []domain.PlanAttempt{plan}}, roots) {
		t.Fatal("failed plan established completeness")
	}
	if completePlan(domain.PlanRun{Attempts: []domain.PlanAttempt{{RootID: "root", Phase: "plan", Status: "succeeded"}}, ErrorPresent: true}, roots) {
		t.Fatal("unattributed aggregate error established completeness")
	}
}

type partialPlanner struct{ run domain.PlanRun }

func (p partialPlanner) Plan(context.Context, domain.PlanRequest) (domain.PlanRun, error) {
	return p.run, errors.New("unknown operation outcome")
}

func TestProposedPlansRetainsRunWhenPlannerReturnsError(t *testing.T) {
	source := &sourceControlStub{snapshot: domain.SourceReviewSnapshot{Change: domain.SourceChange{State: "open", HeadSHA: "head", BaseRef: "main"}}}
	run := domain.PlanRun{Attempts: []domain.PlanAttempt{{RootID: "root", Phase: "plan", Status: "succeeded", Output: "collected output"}}}
	got, err := NewProposedPlans(source, partialPlanner{run}).Plan(context.Background(), domain.RepositoryRef{Owner: "acme", Name: "infra"}, 42, []domain.RootSelector{{ID: "root"}})
	if err == nil || got.Complete || !reflect.DeepEqual(got.Run, run) {
		t.Fatalf("partial run lost or reported complete: %#v, %v", got, err)
	}
}
