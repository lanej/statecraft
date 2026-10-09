# Agent handoff

This describes the mock workbench and separate read-only platform-infra runtime.
Check the current checkout and PR state before inferring merge or deployment status.

## Read and decide

Start with [AGENTS.md](../AGENTS.md). The [product definition](product.md) owns
intent and invariants; [DESIGN.md](../DESIGN.md) owns interaction rules;
[architecture](architecture.md), [integrations](integrations.md), and
[policies](policies.md) own boundary semantics. The [roadmap](roadmap.md) owns
priority, feature gaps, and completion criteria. This file owns the current code
map and practical continuation notes.

**R1: generated Connect transport is implemented.** The recommended next task is
**R2: implement the durable proposal/evidence contract** before selecting storage
or expanding the synthetic demo shape. The [ingestion contract](evidence-ingestion.md)
defines identities, capture, ordering, normalized values, CODEOWNERS/reviewer
requirements, and storage guarantees;
its ports and persistence are not implemented. Start with the pure reducer and
contract tests. Preserve the current mock composition. User feedback on the
workbench can be addressed independently; do not treat the current composition as a frozen design.

The read-only runtime uses real GitHub source evidence when configured.
Its [runbook](read-only-deployment.md) covers platform-infra resources, IAP, credentials, and staged deployment.
Live availability requires deployed-revision and authenticated-browser verification.

## Current baseline

| Area | What exists | What it does not establish |
| --- | --- | --- |
| Review workspace | Overview, root/resource comparison, inspector, before/after, illustrative source/plan evidence, relationships, policy, execution, history | A scalable graph, real plan parsing, source editing, collaboration, or historical evidence comparison |
| Workflow | Plan, request changes, request/grant acceptance, approve, apply, verify | Authentication, real role separation, persistent jobs, or production execution |
| Policy | `PolicyEvaluator` and `ActionPolicy` ports with deterministic sample rules | OPA/Rego evaluation, trusted policy distribution, reference-data/input digests, revocation, or organization policy administration |
| Store | Isolated in-memory sessions; transactional expected-version mutation; detached reads | Durable storage, restart recovery, retention, tenant isolation, or a production audit store |
| Original integration adapters | GitHub reads/reviews/checks; Atlantis command mapping and notification decoding; local HTTP contract tests | Runtime composition, configured installations, authenticated webhook ingress, reconciliation, or exact-artifact apply |
| Read-only runtime | Fixed-repository GitHub reader, source patches/reviews/checks/statuses, signed IAP assertion validation, separate executable | Plans, complete root scope, policy authorization, decisions, actions, persistence, or verified deployment |
| API | Generated Connect handler/client, separate source-evidence service, pinned local generation | Durable evidence ingestion or production workflow authorization |

GitHub/Atlantis/OPA types do not define the frontend or domain. Initial sample
models deliberately cover less than the target architecture; do not read every
future entity in `architecture.md` as an implemented feature.

## Mocked provider acceptance path

A separate signed GitHub webhook → source loading → Atlantis planning acceptance
harness now exists. See [planning acceptance](planning-acceptance.md) for its command,
scenarios, boundaries, and production gaps. `ProposedPlans` orchestrates the existing
provider ports; the HTTP ingress is intentionally not mounted in the demo runtime.
It retains command evidence and checks source freshness without constructing a
PlanSet or approval. Durable webhook receipts/queued work and evidence ingestion
remain required before deployment.

## Code map

| Change you need to make | Start here |
| --- | --- |
| Runtime composition and listen address | `cmd/statecraft/main.go` |
| Read-only composition and deployment | `cmd/statecraft-readonly/main.go`, [runbook](read-only-deployment.md) |
| Read-only HTTP surface | `internal/adapters/readruntime/handler.go` |
| Read-only GitHub evidence and IAP | `internal/adapters/githubread/`, `internal/adapters/iap/` |
| Read-only transport and UI | `internal/adapters/connectapi/source_evidence.go`, `web/src/source.ts`, `web/src/source.css` |
| Demo HTTP routes, decoding, error mapping | `internal/adapters/connectapi/reviews.go`, `mapping.go`, and their tests |
| Workflow transitions and command-time revalidation | `internal/service/demo_workflow.go` and its test |
| Review, evidence, decision, and acceptance shapes | `internal/domain/review.go`, `internal/domain/workflow.go` |
| Source and execution contracts | `internal/domain/source_control.go`, `internal/domain/execution.go`, `internal/ports/` |
| Seed scenarios and recovery planning | `internal/adapters/mock/scenarios.go` |
| Sample assessment and action gates | `internal/adapters/mock/policies.go` |
| Transactional demo store | `internal/adapters/mock/store.go` |
| Source metadata refresh and invalidation | `internal/service/source_reviews.go`, `internal/domain/source_control.go` |
| Pure proposal summaries and comment publication | `internal/domain/proposal_summary.go`, `internal/adapters/github/proposal_comments.go`, [example](comment-publication.md) |
| Structured RPC outcomes | `internal/adapters/connectapi/logging.go` and runtime logger composition |
| External mappings | `internal/adapters/github/`, `internal/adapters/atlantis/` |
| Public API schema and generation | `proto/statecraft/v1/review.proto`, `buf.yaml`, `buf.gen.yaml` |
| Frontend contract and request/focus/session behavior | `web/src/gen/statecraft/v1/review_pb.ts`, `web/src/api.ts`, `web/src/main.ts` |
| View composition and presentation logic | `web/src/review.ts`, `web/src/style.css` |
| Frontend rendering regressions | `web/test/review.test.mjs` |
| Read-only browser regressions | `web/browser/source.spec.ts`, `web/playwright.config.ts` |
| Actual running-UI examples | `docs/screenshots/`; private source captures in platform-infra's `docs/statecraft/screenshots/` |
| Legacy static composition checks | `.ui-review/config.json`, `.ui-review/rules.json` — target root-level prototype only |

The frontend is plain TypeScript with HTML render functions, not a component
framework. Refactor when a use case warrants it; a framework migration is not a
prerequisite for the next feature. The mock source/plan strings remain illustrative fixtures.
The read-only view instead displays GitHub patches, with omission/truncation warnings.

## Run and exercise

Use the commands in [AGENTS.md](../AGENTS.md) and the HTTP routes/port options in
[steel-thread.md](steel-thread.md) for the mock workbench.
Use [read-only deployment](read-only-deployment.md) for the real-source runtime.
The mock app creates a session through the
backend rather than importing frontend fixture JSON. Session identity lives in
per-tab browser storage; reset/scenario selection creates another session. The
API caps the store at 512 sessions and a restart clears them.

| Selector option | Expected observation / next action |
| --- | --- |
| Policy review | Database replacement blocks approval/apply. Request acceptance with rationale and recovery evidence; simulate the database owner's grant; separately approve, apply, then verify. |
| Ready to plan | No plan evidence is shown. Run a plan to collect it. |
| Routine change | Advisory connectivity concern remains visible. Approval is separate from apply. Requesting changes supersedes that reviewer's approval. |
| Incomplete planning | Network failed and supplies no current resource changes. Replanning restores the root's evidence and complete assessment. |
| Stale approval | New head differs from the retained plan. Replanning creates new evidence; old decisions remain historical. |
| Expired acceptance | Approval remains recorded, the violation remains violated, and expiry blocks apply. Request renewed acceptance. |
| Partial apply failure | API already changed, network failed, observability did not start. Replanning excludes applied changes and requires a decision on remaining work. |

The owner and reviewer are fixed demo personas. Simulating owner acceptance is not
an authorization mechanism. Background refresh updates displayed eligibility;
every command still checks current state and expiry on the server. A stale version
returns a conflict instead of silently replaying a decision.

## Limits that matter for the next agent

- External clients now require official SDK first, API-specification generation
  second, and documented exceptions only after both fail. The existing GitHub
  community SDK and Atlantis handwritten client still need that exception review
  before extending their provider transport. The separate read-only reader uses
  GitHub's pinned official specification; see the [selection rule](integrations.md#client-selection-rule).
- `PlanSnapshot` uses review-local IDs and synthetic digests. History keeps plan
  identities/change IDs, human decisions, and simulated attempts; it does not keep
  full immutable old plans, evaluations, relationships, and evidence for replay.
- `PlanPolicyInput` is currently a review plus an explicit time. `ActionPolicyInput`
  adds the action and violation ID. Trusted actor/role and versioned reference-data
  context from the policy design are not implemented.
- Changes use display strings for properties. Typed values, unknowns, sensitive
  values, stable resource identity across plans, and evidence references need a
  normalized plan-ingestion design before processing real artifacts.
- Demo transitions complete synchronously. There is no durable queue, operation
  receipt, uncertain-result recovery, external lock model, or execution cancel path.
- Generated message types use `bigint` for 64-bit values in TypeScript. Preserve
  that precision; use protobuf equality instead of `JSON.stringify` on reviews.
  Regenerate rather than editing `gen/` or `web/src/gen/`. See [transport](transport.md).
- Current tests exercise domain/service behavior, provider mapping, HTTP validation,
  and rendered HTML. CI configures Playwright 1.62.1 source-view regressions using isolated API fixtures.
  They cover disclosure/focus, failed-refresh evidence retention/recovery, and narrow long-patch layout.
  Full decision-workflow, SSO, and comprehensive accessibility coverage remain open.
- Local real-source checks inspected platform-infra PR #253, including recovery and desktop/mobile layouts.
  Source screenshots remain in private platform-infra's `docs/statecraft/screenshots/`.
  The public bad-credential screenshot contains no private source data.
  These validate the local read-only UI; deployed GCP/IAP access remains unverified.
- The `.ui-review` selectors and comparison counts describe the older nine-resource
  static prototype, not the four-change workbench. They must be migrated before
  claiming that tool validates this UI.
- The Atlantis command API's replan-before-apply behavior is a production blocker,
  not a frontend confirmation problem. [Read the boundary](integrations.md#apply-is-not-approval-enforcement)
  before proposing live execution.

## Keep the handoff useful

When finishing a slice, update current status and the next bounded task, link any
new source-of-truth design or adapter contract, and record unresolved decisions.
Do not preserve chat chronology, machine-specific setup, or passing test counts as
permanent product documentation. Evidence, remaining limitations, and decisions
another agent would otherwise have to rediscover are the useful handoff.

