# A pragmatic core-to-GitHub example

A proposal update is useful in a GitHub discussion, but its meaning does not
require a GitHub destination. This example separates those concerns without
adding another general-purpose publication interface.

| Part | Responsibility | Knows GitHub or a PR number? |
| --- | --- | --- |
| `domain.SummarizeProposal` | Pure summary of plan revision, root completeness, policy coverage, accepted risks, approval, and verification | No: input has only Statecraft evidence and an explicit time |
| Application composition | Projects evidence into the summary input and chooses the destination from trusted context | Yes; this is orchestration |
| `githubadapter.RenderProposalComment` | Formats structured facts as GitHub Markdown | Yes |
| `SourceControl.PublishProposalComment` | Calls the pinned community SDK and records the publication receipt | Yes; SDK types remain in the integration |

The new capability is a concrete GitHub integration method, separate from the
existing `SourceControl` port. A second provider or genuine substitution need can
justify another boundary later. Neither the summary nor publication records a
Statecraft approval, grants acceptance, or authorizes apply.

The core's summary input deliberately excludes repository, change number, URL,
comment ID, and destination. This is a bounded example, not a claim that every
older model has already been reduced to the same thin waist.

## Run the workflow and inspect the logs

```sh
go test ./tests/acceptance -run TestReviewWorkflowPublishesProviderIndependentSummary -count=1 -v
```

The acceptance composition uses generated Connect requests and the current mock
store/policies to:

1. Create a review with a database-replacement violation.
2. Attempt approval and observe `failed_precondition`.
3. Request acceptance with rationale/recovery evidence and simulate the owner's grant.
4. Separately approve the exact plan and assessment; the violation remains violated.
5. Repeat an older expected version and observe `aborted` without another decision.
6. Simulate apply, then separately verify resulting-state agreement.
7. Pass the resulting evidence through the pure summary function.
8. Supply an explicit GitHub destination in the shell and publish the summary.

The last step uses the real `go-github/v92` SDK against a local HTTP mock, which
receives `POST /repos/acme/infra/issues/7/comments`. GitHub conversation comments
use the issue-comment API; they are different from review decisions and inline
code-review comments. The returned comment ID/URL belongs to the integration.
All infrastructure evidence and execution in this walkthrough are synthetic.
No live GitHub or Atlantis traffic is made by the test.

The test prints actual structured completion logs for each RPC and the SDK
publication. Running `cmd/statecraft` enables the same RPC logger with JSON output.
Logs include procedure, action, expected/current version, review/plan identity,
state, outcome, and duration. Rationale, recovery evidence, raw plans, comment
bodies, and credentials are not logged. Domain analysis has no logger dependency.

A rejected GitHub request is reported as rejected. Server/transport errors and
missing publication receipts are uncertain: a comment may already exist. There is
no automatic retry in this publication method; reconcile before attempting
another write. Durable receipts, authenticated composition, and synchronization
remain production work. The publisher is not mounted in the mock browser runtime.

## Sources

- [GitHub issue-comment API](https://docs.github.com/en/rest/issues/comments?apiVersion=2022-11-28)
- [Pinned Go SDK](https://pkg.go.dev/github.com/google/go-github/v92/github#IssuesService.CreateComment)
