# Mock review workbench

The runnable TypeScript → Go thread lets a reviewer inspect a multi-root proposal
and exercise plan, policy acceptance, approval, apply, and verification decisions.
The scenario selector creates a new isolated session; reset starts that scenario
over. Reloading the same browser tab resumes its session while the server remains
running. See the [handoff](handoff.md) for code locations and [roadmap](roadmap.md)
for the next implementation slices.

## Run

Use `make api` in one terminal, then `cd web && npm ci && npm run dev` in another.
The API binds to `127.0.0.1:8081`. To avoid a port collision, set
`STATECRAFT_PORT=18481` for the API and
`STATECRAFT_API_URL=http://127.0.0.1:18481` for Vite. Open the Vite address displayed
in the terminal.

All evidence, reviewer identities, policy evaluation, and execution are simulated.
The runtime contains no credentials or live execution adapters. Sessions and audit
history are in memory, with a 512-session limit; restarting clears them. It is a
local development demo without authentication, not a deployable approval service.

## Boundaries

```text
TypeScript workbench → generated Connect RPC → DemoWorkflow
                                            ├─ WorkflowStore → in-memory sessions
                                            ├─ DemoPlanner → synthetic changes
                                            ├─ PolicyEvaluator → sample policy rules
                                            └─ ActionPolicy → sample workflow rules
```

`internal/domain` and the frontend contract contain Statecraft models only. The
existing `SourceControl` → GitHub and `Planner`/`Executor` → Atlantis adapters remain
separate and are not composed into `DemoWorkflow`. The simulation-only planner
rebuilds evidence after a failed plan and excludes already-applied roots during
recovery. It cannot issue infrastructure commands.

The policy adapter returns evaluation coverage, versioned policy outcomes,
violations, and action eligibility. The application checks eligibility and the
expected review version atomically before each mutation. Concurrent/stale commands
return a conflict; partial mutations do not commit. The browser displays server
reasons and refreshes after a rejected stale decision.

## Evidence and decisions

- **Inspect:** roots → consequential resources → before/after properties, illustrative
  source and plan text, relationships, and policy evidence. Failed roots contribute
  no current resource evidence; unchanged roots remain in the expected scope.
- **Accept:** rationale and recovery evidence create a request. A separate simulated
  database-owner decision grants acceptance for one hour, scoped to the violation,
  policy version, assessment, and plan. The violation remains violated.
- **Approve:** Alex's decision binds to the exact plan, commit, and assessment.
  Requesting changes supersedes Alex's earlier approval for that proposal.
- **Re-plan:** a new plan invalidates old approvals and acceptances, even on the same
  commit. History retains the old scope and human decisions.
- **Apply:** current evidence, complete required policies, unexpired acceptance,
  and a current approval are rechecked on dispatch. Partial execution requires a
  new plan of remaining work; already-applied roots are not blindly retried.
- **Verify:** simulated resulting-state agreement is recorded separately from
  command success. It does not establish operational health or data recovery.

The seven entry scenarios are policy review, ready to plan, routine change,
incomplete planning, stale approval, expired acceptance, and partial apply failure.
They support repeatable investigation of both the normal path and recovery.

## Generated Connect boundary

`DemoWorkflowService` provides `CreateDemo`, `GetDemo`, and `ActOnDemo` under
`/statecraft.v1.DemoWorkflowService/`. The original read-only fixture `pr-1842`
remains available through `/statecraft.v1.ReviewService/GetReview`. The former
`/api/demos` and `/api/reviews` JSON routes are removed.

Go handlers/clients and TypeScript messages/service descriptors are generated from
`proto/statecraft/v1/review.proto`. The browser uses a generated client; rendering
uses the generated review types. Commands cannot supply reviewer/authorizer
identity; the demo uses fixed personas, not authentication. See [transport](transport.md)
for error codes, request limits, generation, and CI enforcement.

## Production work still required

The policy models here are the first runnable subset of [policies.md](./policies.md).
Production evaluation needs trusted actor/role context, policy/reference-data and
input digests, validity boundaries, revocation, durable immutable assessments, and
an OPA/Rego adapter. Current `PlanPolicyInput` and `ActionPolicyInput` contain a
normalized review and explicit time; they do not yet model those production facts.
Plan digests are labeled synthetic, and history retains plan identities/change IDs
rather than full historical evidence snapshots.

Live execution additionally needs ingestion and reconciliation of exact artifacts,
persistent jobs/logs, source authentication and synchronization, and enforcement
across all execution paths. The [Atlantis apply limitation](./integrations.md#apply-is-not-approval-enforcement)
still applies. None of the mock readiness signals authorize a real apply.
