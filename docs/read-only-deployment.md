# Read-only platform-infra deployment

Deploy Statecraft behind IAP to inspect `easypost/platform-infra` source evidence.
The deployment path uses reviewed platform-infra changes and Atlantis.

## Runtime boundary

`cmd/statecraft-readonly` composes `internal/adapters/readruntime` to serve `web/` and `SourceEvidenceService`.
It reads open PRs, changed-file patches, submitted reviews, check-runs, and commit statuses.
Captures include the source commit and capture time.
Upstream Atlantis indicators remain GitHub observations when present.

GitHub patches can omit or truncate content; check/status lists can be incomplete.
Each check-run and commit-status request is capped at 100 entries.
External reviews and successful checks establish neither Statecraft approval nor verified infrastructure execution.
There are no plans, complete root scope, policy assessments, decisions, actions, or persistence.
The runtime composes no Atlantis client or mock workflow.

## Infrastructure and access

The platform-infra root is `live/gcp/development/easypost-core/statecraft`.
It provisions `easypost-statecraft-dev` in `us-east1`, including:

- Cloud Run with native IAP, IAM invoker checks, and maximum two instances.
- Artifact Registry, a dedicated token secret, and separate runtime/build service accounts.
- Main-branch GitHub Workload Identity Federation for image publishing.
- A $25 monthly alert budget; alerts do not cap spending.

The runtime identity reads the dedicated secret through its configured grant.
The build identity publishes images without Cloud Run deployment permission.
IAP grants `domain:easypost.com` access to all EasyPost domain accounts.
Changes to `iap_members` require reviewed platform-infra changes.
Every authorized domain account can see repository evidence, regardless of individual GitHub access.

The domain grant requires Security approval under platform-infra's existing broad-principal policy.
Keep that approval gate intact; this deployment adds no policy exception.
See the private [IAM policy](https://github.com/easypost/platform-infra/blob/main/atlantis/runtime/policy/security/src/infrastructure/gcp/privileged_iam.rego)
and [approval configuration](https://github.com/easypost/platform-infra/blob/main/atlantis/runtime/repos.yaml).

The application also validates `X-Goog-Iap-Jwt-Assertion` using Google's official `idtoken` package.
Its audience is `/projects/PROJECT_NUMBER/locations/us-east1/services/statecraft`.
Editable email headers provide no authentication.
See [native Cloud Run IAP](https://docs.cloud.google.com/run/docs/securing/identity-aware-proxy-cloud-run)
and [signed assertions](https://docs.cloud.google.com/iap/docs/signed-headers-howto).

## Staged deployment

1. **Provision the foundation.** Review and apply the platform-infra root through Atlantis.
   Keep `application_image` and `github_token_version` null.
   This creates supporting resources, without starting Cloud Run.
   Retain the `workload_identity_provider` and `github_token_secret` outputs.
2. **Provision the credential.** Create an expiring fine-grained GitHub token for only `easypost/platform-infra`.
   Grant [Pull requests: read](https://docs.github.com/en/rest/pulls/pulls#fine-grained-access-tokens-for-list-pull-requests-files),
   [Checks: read](https://docs.github.com/en/rest/checks/runs#fine-grained-access-tokens-for-list-check-runs-for-a-git-reference),
   [Commit statuses: read](https://docs.github.com/en/rest/commits/statuses#fine-grained-access-tokens-for-get-the-combined-status-for-a-specific-reference),
   and [Metadata: read](https://docs.github.com/en/rest/authentication/permissions-required-for-fine-grained-personal-access-tokens#repository-permissions-for-metadata).
   Complete required organization authorization before testing access.
   Add its value through Secret Manager's console to `statecraft-github-read-token`.
   Keep the value out of shell history, logs, OpenTofu inputs, and state.
   Retain only its numeric version for the deployment change.
3. **Enable image publishing.** Set `lanej/statecraft`'s repository variable `STATECRAFT_WIF_PROVIDER` from the infrastructure output.
   Merge the reviewed application change, then run `build-image` on `main`.
   Retain the resulting immutable `app@sha256:…` image reference.
4. **Deploy the workload.** Open a second reviewed platform-infra change.
   Set `application_image` to that digest and `github_token_version` to the numeric version.
   Inspect the Atlantis plan and obtain Security approval for `domain:easypost.com`.
   Apply through Atlantis with existing policy gates intact.
   Retrieve `web_url` and inspect the deployed revision before reporting availability.

Follow platform-infra's review and merge requirements at each stage.
Never run a deployment-root apply directly from a workstation.
An image build or infrastructure apply alone does not prove authenticated browser access.

## Verify the deployment

- Confirm the serving Cloud Run revision resolves to the reviewed image digest.
- Confirm native IAP remains enabled and invoker IAM checks remain active.
- Inspect IAP membership and the IAP service agent's `roles/run.invoker` grant.
- Confirm unauthenticated requests cannot retrieve source evidence.
- Confirm an authenticated account outside `easypost.com` cannot retrieve source evidence.
- Sign in with an EasyPost domain account; inspect a real platform-infra PR.
- Compare its head, patches, submitted reviews, check-runs, and statuses against GitHub.
- Exercise refresh, unavailable evidence, keyboard focus, and narrow layout.
- Confirm mock workflow actions and Atlantis endpoints are unavailable.

Record deployment and browser evidence separately from local tests.
Live availability remains unverified until these checks succeed.

Local browser checks inspected real platform-infra PR #253 at 1440px and 390px widths.
They exercised filtering/focus, patch disclosure, failed-refresh evidence retention, and recovery.
GitHub check-runs and Atlantis commit statuses appeared as source observations.
A separate bad-token server exposed no PR or file data.
Local Viewrule checks covered accessibility rules, clipping, and controls at both widths.
These checks used the loopback read-only runtime; GCP deployment and IAP remain unverified.

Private [source screenshots](https://github.com/easypost/platform-infra/tree/main/docs/statecraft/screenshots)
record the local result.
The [bad-credential screenshot](screenshots/source-unavailable.png) contains no private repository data.
`web/browser/source.spec.ts` adds isolated fixture-based browser regressions, configured in CI.
Full workflow, SSO, and comprehensive accessibility coverage remain open.

Keep private source screenshots in platform-infra's `docs/statecraft/screenshots/`.
Public Statecraft PRs can reference private, durable GitHub image URLs.
Never publish private source evidence or screenshots in the public Statecraft repository.

## Local development

Build the application and enter a repository-scoped token without terminal echo:

```sh
make build
set +x
read -r -s STATECRAFT_GITHUB_TOKEN
export STATECRAFT_GITHUB_TOKEN
make readonly
```

Use the server's loopback address; unset the token after stopping the process.
`--local` bypasses IAP only on loopback and is forbidden on Cloud Run.
`PORT` selects the listen port when its default is occupied.
The mock runtime remains available separately through `make api`.
Run the source-view browser regressions with `npm --prefix web run test:browser`.

## Credential and image updates

Rotate credentials by creating another secret version, then reviewing its configured version.
Verify the new revision before retiring the old credential.
Promote each image digest through platform-infra review and Atlantis.
GitHub Actions publishes images without deploying them.
The next product task remains durable evidence ingestion; real planning requires that contract.
