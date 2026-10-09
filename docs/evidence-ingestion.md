# Durable proposal and evidence contract

This is the R2 design contract, not an implemented ingestion service or storage
adapter. The mock runtime remains unchanged. The next implementation slice is a
storage-independent reducer and contract tests, followed by a durable adapter that
can replay a multi-root proposal after restart. Database and object-store selection
remain open.

## Ownership and boundaries

Statecraft retains exact evidence and immutable history. An execution adapter
captures saved plans, structured plan output, and logs before its workspace goes
away. A normalization adapter translates engine documents into typed Statecraft
changes and relationships. Application services validate provenance and scope,
assemble proposals, and select the current review projection. Storage adapters
persist those records; they do not decide readiness or approval.

The existing `PlanRun` represents command evidence. A successful command and text
output cannot manufacture an exact plan artifact or a complete PlanSet. This
contract adds no apply capability and does not resolve Atlantis's replan-before-apply
limitation. Evidence capture and enforcement of the artifact actually applied are
separate requirements.

## Identities and immutable records

All records and lookups carry a trusted tenant ID. Repository identity includes
the source-host instance and its stable repository ID; owner/name is retained for
display and adapter routing. A repository rename must not change historical identity.

| Record | Required binding and content |
| --- | --- |
| Source revision | Tenant, repository, stable source-change ID, head commit, base commit/ref, source observation evidence, server-assigned review version. A new authoritative observation can invalidate eligibility without deleting a prior proposal. |
| Ownership snapshot | Exact base-branch CODEOWNERS evidence (or verified absence), base commit, changed-path set, resolver version, matched rule locations, resolved principal IDs, membership/access observations, and effective reviewer-requirement configuration digest. |
| Planning generation | Server-issued ID and monotonic sequence within the review; source revision; trusted root-discovery/configuration digest; complete, sorted expected root IDs; creation time and producer configuration. Replanning creates a new generation even at the same commit. |
| Root attempt | Server-issued ID, generation/root binding, ordinal within the root, execution-system operation correlation, producer identity, start/end observations, immutable event sequence and evidence references. A retry creates a new attempt. |
| Artifact | Tenant-scoped opaque evidence ID, kind, SHA-256 of exact decoded bytes, byte length, media type, capture time, verified producer/operation binding, engine/version, access classification and retention class. Storage locations stay adapter-private. |
| Normalization | Exact plan and structured-output references, normalizer name/version/configuration digest, normalized schema version, output digest, typed changes/relationships, explicit coverage and errors. Each normalization is immutable; rerunning creates another record. |
| PlanSet | Canonical manifest of source identity, generation scope/configuration, and one selected exact root plan per expected root. It references the supporting attempts, evidence, and normalizations through immutable assembly records. |

Root IDs are repository-scoped Statecraft configuration IDs, independent of an
Atlantis project name or directory/workspace selector. A resource identity is the
root ID plus the engine's canonical module/resource instance address, including
instance keys. A rename/move is a separately evidenced relationship between old
and new identities; it is not guessed from matching display names. Cross-plan
correlation never transfers approval or acceptance.

### PlanSet identity

Use `planset:v1:sha256:<hex>` over a versioned manifest, not the current demo's
synthetic digest. V1 contains, in this order:

1. Schema tag `statecraft.planset.v1`, tenant ID, source-host ID, repository ID,
   source-change ID, head commit, base commit and base ref.
2. Trusted root-scope/configuration digest.
3. Count of roots, followed by entries sorted by UTF-8 root ID bytes. Each entry
   contains the root ID, exact saved-plan SHA-256, and its byte length.

Encode strings as their UTF-8 byte length (unsigned 64-bit big endian) followed
by bytes, and counts/lengths as unsigned 64-bit big endian. Digests in the manifest
are lowercase 64-character hex strings; identifiers are opaque exact strings,
without case folding or Unicode normalization. Reject empty required identities,
duplicate roots, malformed digests, negative/overflowing lengths, and incomplete
scope before hashing. Define golden vectors when implementing this function.

Attempt IDs, arrival order, timestamps, logs, and normalizer versions are excluded
from this identity. Two generations with identical manifests identify the same
proposal but retain different assembly/provenance records. Any changed exact plan,
base/head revision, or root configuration changes the identity. A normalization
revision changes the assessment input digest, even if the PlanSet remains the same;
it requires eligibility reevaluation under the policy contract.

An exact saved plan is required for this initial reviewable PlanSet contract. A
producer that can provide only JSON or textual output remains visibly incomplete;
do not substitute the digest of JSON for the digest of an executable saved plan.

## Intake and capture protocol

The shell authenticates the producer and resolves its tenant/repository/root
permissions from trusted configuration. A body claiming another tenant, actor, or
role cannot grant access. A capture is bound to a previously issued generation and
attempt; producer-supplied timestamps and commit strings cannot select current work.

1. Persist the generation and root-attempt intent before dispatching planning.
   Operation correlation survives restart. A timeout remains uncertain and must
   be reconciled before creating a retry; evidence intake does not replay commands.
2. Stream artifacts to restricted staging storage with bounded size and content
   limits. Verify the digest and decoded byte length, then commit an immutable
   evidence receipt only after the backing bytes are durable. Partial uploads
   cannot be referenced by a committed event. Unreferenced staging objects can be
   collected after a configured grace period.
3. Submit an immutable attempt event with ingestion key, canonical envelope
   digest, generation/attempt identity, and producer sequence. Artifact receipts
   and trusted operation context must agree with that binding. Persist receipt,
   event, and resulting review projection atomically against an expected version.
4. Run bounded normalization against the retained exact inputs. Attach an immutable
   normalization record and evaluate coverage. Unsupported formats or missing
   values remain explicit failures/unknowns; no empty-success fallback.
5. Assemble a PlanSet only when the current generation has the required evidence
   for every expected root. Assessment and human approval are later independent
   operations, bound to that PlanSet and its exact assessment input.

The capture integration must attest that structured output was derived from the
referenced saved plan, retaining its command/tool version and operation context.
Matching independently supplied commit labels or digests alone does not prove that
relationship. The concrete workflow transport and authentication mechanism remain
to be selected before connecting a real producer; this design does not claim that
the Atlantis command response supplies these artifacts.

### Ordering, retries, and readiness

Events use producer sequence numbers within a server-issued attempt; wall clocks
are provenance, never ordering authority. Sequence numbers start at one. An exact
duplicate key/envelope returns the original receipt without another write. Reusing
a key or attempt sequence for different content is a conflict. The canonical
envelope digest includes tenant, producer, target identities, event schema/kind,
sequence, timestamps, artifact identities/digests, and payload; its byte encoding
must be pinned with the intake schema before implementation.

Persist out-of-order events, but fold only the contiguous sequence prefix. A gap
cannot establish readiness. Terminal succeeded, failed, cancelled, or uncertain
observations remain immutable; correction requires a separately correlated
reconciliation record, not overwriting a success/failure. A terminal success must
declare the complete required artifact set. A newer attempt supersedes an older
one within its root; late events from the older attempt are history only.

| Observation | Effect on current projection |
| --- | --- |
| One expected root has no attempt or artifact | Missing/incomplete; preserve the expected root in the view. |
| Plan command fails, is discarded, or has an uncertain outcome | Failed/discarded/uncertain evidence remains visible; no ready PlanSet. |
| Root succeeds but normalization is missing, unsupported, or partial | Artifact retained; review evidence incomplete. |
| Duplicate delivery | Return the stored receipt; no repeated transition or dispatch. |
| Result from an old generation arrives | Retain it as history; never promote it to the current generation. |
| Source revision or root configuration changes | Invalidate current eligibility immediately; retain previous evidence/decisions. |
| Complete evidence arrives for all current roots | Assemble the immutable proposal; this establishes evidence completeness, not approval or apply permission. |

A generation is current only through a version-checked application decision. A
late source observation cannot move the current pointer backwards; source-refresh
workers use the review version they started from and re-read after a conflict.
Webhook payload order is not source authority. No historical successful attempt
may fill a missing root in a newer generation automatically. Evidence reuse, if
added later, needs a separate validated binding and explicit provenance.

## Code ownership and reviewer requirements

Capture code ownership alongside source evidence. The GitHub adapter reads the
selected CODEOWNERS file at an exact base-branch commit, using the documented
location precedence (`.github/`, repository root, then `docs/`). Head-branch changes
to CODEOWNERS are proposed changes, not authority to relax their own review.
Retain exact bytes, file path, commit and digest, or an explicit verified absence.
Use GitHub-compatible, case-sensitive matching with the last matching rule taking
precedence; retain parser diagnostics and matched line locations. Do not substitute
an ordinary gitignore matcher. See [GitHub's CODEOWNERS contract](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/about-code-owners).

Resolve ownership for the complete changed-path set and persist the path-to-owner
mapping. Renames/deletions retain both old and new path evidence. Statecraft's
initial policy conservatively resolves both paths for renames, and the old path
for deletions; record that policy separately from GitHub's observed merge gate.
For root-level reviewers, include the source files/modules known to affect each
root, with explicit dependency-mapping coverage. Directory ownership alone cannot
prove coverage when a root depends on a shared module elsewhere. Missing provenance
cannot silently become an unowned resource.

Normalize users/teams to stable source-host principal IDs, retaining display names.
Membership and repository access are time-varying trusted observations with their
own versions/timestamps; refresh them before recording a qualifying decision.
An unresolved team, insufficient access or an unreadable/invalid ownership file
is a diagnostic, never evidence that owner review is unnecessary. Verified absence,
an intentional ownerless rule and an unmatched path are distinct known outcomes;
trusted workflow configuration specifies their fallback reviewer requirement.

Ownership identifies reviewers; effective branch protection/rulesets and
Statecraft policy determine whether their approval is required. Store each
requirement's scope, source and satisfaction rule explicitly. GitHub permits any
listed qualifying owner to satisfy an owner-review requirement; additional
independent role approvals require an explicit Statecraft rule, not an invented
AND across every name. GitHub merge requirements and Statecraft PlanSet decisions
remain separate, with separate evidence of satisfaction.

Bind the ownership snapshot, path/root mapping, membership/access observations and
requirement configuration to the action-policy input digest, rather than adding
mutable reviewer requirements to PlanSet identity. Append snapshots through a
version-checked `RecordReviewRequirements` ledger operation. A change reevaluates
eligibility, preserves earlier decisions, and requires renewed review where the
current rules demand it. CODEOWNERS never grants violation-acceptance authority,
production access or permission to apply by itself.

## Normalized value and relationship contract

Provider JSON remains restricted evidence. The domain projection records a typed
value tree with a separate state: `known`, `unknown`, `redacted`, or `absent`.
Known values include null, boolean, arbitrary-precision decimal number, string,
ordered list, map keyed by strings, and explicitly typed set. Preserve numeric
lexemes without routing them through floating point. Unsupported types produce a
normalization error. Unknown/redacted subtrees may coexist with known siblings;
redacted values contain no payload or reusable secret fingerprint.

Absent differs from known null. Unchanged is a property-change result established
by comparison, not a value state and never inferred from a missing field. Paths
are typed key/index segments, not ambiguous dotted strings. Changes retain root,
resource identity, action sequence (including replacement ordering), before/after,
evidence reference and locator, and coverage. Sensitive markers take precedence
over known or unknown payloads in anything visible to reviewers.

Relationships record source/target resource identity, kind, evidence locator,
origin (`declared`, `derived`, or `inferred`) and derivation version. Coverage must
distinguish a proven empty relationship set from an unavailable or partial graph.
Unresolved targets remain explicit and do not invent resources or cross-root edges.

## Storage ports and transactional guarantees

These are proposed use-case ports, not new methods on the demo `WorkflowStore`.
Types below denote the records above; no database driver, provider DTO, bucket URL,
or arbitrary engine JSON participates in the service contract.

```text
EvidenceObjects
  Put(scope, metadata, byteStream) -> VerifiedArtifactReceipt
  Open(scope, evidenceID) -> metadata, byteStream

ProposalLedger
  RecordSourceRevision(scope, expectedReviewVersion, sourceObservation)
    -> revision, reviewVersion
  RecordReviewRequirements(scope, expectedReviewVersion, ownershipSnapshot, requirements)
    -> requirementsReceipt, reviewVersion
  BeginGeneration(scope, expectedReviewVersion, sourceRevision, rootScope)
    -> generation, reviewVersion
  BeginAttempt(scope, expectedReviewVersion, generationID, rootID, operationIntent)
    -> attempt, reviewVersion
  CommitIngestion(scope, expectedReviewVersion, envelope, verifiedReceipts)
    -> ingestionReceipt, reviewVersion
  CommitNormalization(scope, expectedReviewVersion, attemptID, normalization)
    -> normalizationReceipt, reviewVersion
  CommitAssembly(scope, expectedReviewVersion, generationID, assembly)
    -> assemblyReceipt, reviewVersion
  GetReview(scope, reviewID) -> currentProjection, reviewVersion
  GetGeneration(scope, generationID) -> immutableRecords, derivedProjection
  ListHistory(scope, reviewID, cursor, limit) -> immutableRecords, nextCursor
  GetIngestionReceipt(scope, producerID, ingestionKey) -> receipt
```

The ledger atomically commits each mutation's receipt, immutable records, current
pointer/projection, and version. Application code supplies validated transitions;
the adapter enforces uniqueness, references, and compare-and-swap. Never expose
mutable domain references or a callback that can perform external I/O under a
storage transaction. Reads return detached snapshots. History pagination uses a
stable ledger position, not arrival timestamps, and is bounded.

Every immutable object ID is put-once: identical content is idempotent, changed
content is a conflict. Exact ingestion retries return their original receipt even
when the supplied expected version is now old; receipt lookup happens before the
version check. A new mutation at an old version conflicts without partial writes.
An ambiguous commit is resolved by receipt lookup before retry; a storage error
never becomes synthetic success. Errors distinguish not found, version/identity
conflict, invalid binding, unavailable evidence and unavailable storage. Unauthorized
lookups do not reveal another tenant's existence.

Blob storage and ledger storage need not share a transaction: durable verified
objects precede ledger references. The ledger rejects missing, cross-tenant,
incorrectly bound, or unavailable receipts. A crash may leave an orphan object,
but must not produce ready metadata referring to an uncommitted upload. Rechecking
availability at read/action time prevents a historical ready projection from
authorizing an action when evidence is no longer accessible.

## Access, retention, and replay

Raw saved plans, structured output, and logs may contain credentials or sensitive
state. Encrypt them in transit/at rest; restrict raw access separately from access
to redacted review projections. Never return raw artifacts through ordinary review
RPCs or publish them to GitHub. Audit raw reads, avoid secret-bearing logs/errors,
and keep normalizer scratch data bounded and disposable. Tenant-scoped content
addressing must not expose cross-tenant digest or existence queries.

Retention is trusted service configuration, recorded by class on intake. Referenced
evidence must remain available for the configured review/decision audit period;
holds prevent ordinary expiry. Expiry/deletion retains a tombstone, digest and
reason without pretending bytes can still be replayed. Affected evidence becomes
unavailable and cannot establish current readiness. Hashes alone do not preserve
replay. Raw evidence and redacted projections have separate access/retention rules.
Policy choice and physical deletion jobs remain deployment decisions.

Replay reads immutable source/scope records, attempt events, verified artifacts and
versioned normalization/assembly records. It reproduces historical coverage and
proposal identity at a selected ledger position, using recorded rules/versions.
New normalization results append another interpretation; they never rewrite the
evidence or historical assessment. Current policy/time/authorization are evaluated
again for new actions; historical replay is not permission to execute.

## Next implementation and acceptance cases

First implement the pure identity/ordering reducer, typed value model, and these
ports with a contract-test adapter. Then choose persistence using the same tests;
R2 remains incomplete until a durable adapter passes restart/replay checks.

- Golden identity vectors: root ordering is invariant; a changed plan, tenant,
  base/head or root scope changes identity; log/attempt/normalizer changes do not.
- Multi-root ingestion: one success and one failed/missing root remain incomplete;
  a new generation succeeds without losing the earlier evidence.
- Duplicate/collision/concurrency: identical receipts are stable, changed payloads
  conflict, competing versions cannot both commit, and an uncertain commit is
  reconciled without repeating a mutation.
- Out-of-order and stale results: gaps cannot finish an attempt; old attempts,
  generations and source-refresh workers cannot resurrect readiness.
- Crash boundaries: interrupt artifact upload, after object durability, and during
  ledger commit; restart yields either an orphan or a consistent committed record.
- Normalization: unknown, redacted, absent, null, large numbers, replacement order,
  unresolved edges and unsupported formats remain distinguishable.
- Ownership: base/head divergence, precedence and last-match rules, ownerless/unmatched
  paths, renames/deletions, shared modules, unavailable ownership evidence, team/access
  changes and revised requirements preserve coverage and invalidate stale eligibility.
- Isolation/retention: cross-tenant references fail, raw evidence stays restricted,
  and expired/missing bytes produce honest unavailable replay.
- Restart a durable adapter, load two complete historical proposals and their
  recorded normalizations, reproduce their identities and compare typed changes.

Open decisions: concrete workflow capture/authentication, normalizer-supported
engine/schema versions, database/object store, size/retention defaults, and the
initial tenant configuration. Resolve each before connecting a production producer;
none is implied by this document.
