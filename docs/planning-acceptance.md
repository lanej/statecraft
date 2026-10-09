# Proposed-change planning acceptance

Run `go test ./tests/acceptance -run TestProposedChangePlanningAcceptance -v`.
The ordinary `go test ./...` CI command includes this suite. All HTTP traffic goes
through local test servers; no Atlantis instance, GitHub account, cloud credentials,
Terraform installation, or infrastructure mutation is needed.

The test sends a signed GitHub pull-request webhook through `GitHubWebhook`, which
calls `ProposedPlans` with roots from an explicit repository configuration. The
service uses the real GitHub SDK adapter to load the PR, changed files, and external
review history. It passes the resolved head SHA, base branch, PR number, and explicit
root selection through the real Atlantis adapter to a mocked `/api/plan` endpoint.
The test asserts the complete outbound request and returns representative Atlantis
command responses. After planning, the service rechecks source identity so a changed
head or base, closed PR, or unavailable source cannot establish complete planning.

Covered scenarios: opened and synchronize events, successful planning, failed-root
evidence (including Atlantis HTTP 500), missing root results, discarded plans, and
source changes during planning. Invalid signatures are rejected before provider
access; unrelated events are ignored. Repeated delivery IDs do not reissue plans,
and a delivery ID reused with different content is rejected.

`ProposedPlan` retains source identity, the planned revision, root attempts/output,
and separate completeness/staleness flags. Completeness means successful command
results for all configured roots at an unchanged source revision. It does not
establish exact artifact identity, normalized resource changes, assessment, approval,
or permission to apply. Neither service nor ingress uses the Executor port.

## Scope and continuation

This is an acceptance-harness composition, separate from `DemoWorkflow`. The new
handler is not mounted in `cmd/statecraft`, and its result is not connected to the
browser. Configured roots require distinct stable Statecraft IDs; root discovery
is not inferred from the changed-file list.

The ingress verifies HMAC-SHA256 against the raw body, limits body size, allows only
configured repositories, and handles `opened`, `synchronize`, `reopened`, and
`ready_for_review`. Planning is synchronous. Delivery receipts are process-local,
limited to 4096 entries, and include failed/uncertain dispatches to prevent silent
replay. A duplicate response acknowledges receipt, not successful planning. Restart
loses these receipts; this is not a production webhook intake guarantee.

Before exposing a deployed receiver, add durable authenticated intake and operation
receipts, queued planning and reconciliation, durable plan/evidence storage, and
trusted installation/repository configuration. GitHub webhook retries must never
silently repeat an uncertain operation. Pin the Atlantis version and validate these
mock response fixtures against that version before claiming live compatibility.
Structured plan JSON/artifact ingestion remains a separate delivery step.

Generated Connect transport is implemented in the mock runtime. This harness
remains independently composed; the next task is the durable proposal/evidence
ingestion contract (R2), including receipt retention and replay-safe publication.
