# Generated RPC transport

The local mock runtime and browser use the checked-in protobuf/Connect contract.
`internal/adapters/connectapi` maps Statecraft domain models into generated Go
messages. Domain, ports, and services do not import generated API or provider
packages. `web/src/api.ts` uses the generated service descriptor with Connect's
client; rendering imports generated message types. No handwritten wire contract
or JSON route remains.

## RPCs

All methods are POSTs through the Connect protocol. Vite proxies `/statecraft.v1.`
to `STATECRAFT_API_URL` (default `http://127.0.0.1:8081`).

| Service / method | Purpose |
| --- | --- |
| `DemoWorkflowService/CreateDemo` | Isolated session from a named scenario |
| `DemoWorkflowService/GetDemo` | Current review and eligibility |
| `DemoWorkflowService/ActOnDemo` | Action with expected version and rationale/evidence |
| `ReviewService/GetReview` | Read-only review, including original fixture `pr-1842` |

Paths start with `/statecraft.v1.` followed by the service and method above.
The former `/api/demos` and `/api/reviews` paths return 404. The root-level static
prototype still loads its own fixtures and does not depend on those routes.

The unauthenticated demo still binds to loopback, rejects cross-site browser
requests, enables no CORS, and returns `Cache-Control: no-store`. Connect handles
JSON and binary protobuf encoding, method/content validation, and a 16 KiB request
message limit. Unknown protobuf fields are ignored for forward compatibility;
client-supplied actor fields cannot alter the fixed backend demo identities.

## Errors and versioning

| Condition | Connect code |
| --- | --- |
| Unknown scenario, malformed command, missing ID/version, invalid action | `invalid_argument` |
| Missing review | `not_found` |
| Stale expected version | `aborted` |
| Action denied by current workflow/policy state | `failed_precondition` |
| Session capacity exhausted | `unavailable` |
| Request canceled / deadline elapsed | `canceled` / `deadline_exceeded` |
| Request over the message limit | `resource_exhausted` |
| Unexpected internal failure | `internal` with a generic message |

State and policy gates remain in `DemoWorkflow`. A denied or stale mutation cannot
commit a decision. The browser displays the server's reason and refreshes current
eligibility for aborted/failed-precondition responses, retaining the user's draft.
An absent review payload is a failed response, not an invented successful action.

TypeScript uses `bigint` for `uint64 version` and `int64 pull_request`. Keep those
values exact when sending commands and comparing freshness. Protobuf equality is
used for refresh comparisons because `JSON.stringify` cannot serialize `bigint`.

## Reproduce and enforce

After installing Go 1.26+ and Node.js 22.6+:

```sh
npm --prefix web ci
make generate
make proto-lint
make proto-breaking PROTO_BASE=main
make check-generated
make check-format
```

`go.mod` pins the Go message and Connect plugins through Go tool dependencies.
`web/package-lock.json` pins the local Buf CLI and `protoc-gen-es`; `buf.gen.yaml`
uses only those local plugins. Regeneration does not call remote BSR generators.
Commit both `gen/` and `web/src/gen/`. Do not manually edit generated files.

CI rejects gofmt violations, Biome formatting/lint/import findings (including
warnings), incompatible schemas, and generated-code drift. Generated TypeScript is
excluded from Biome and checked by regeneration instead. Generated Go is included
in gofmt checks. The current frontend sources, tests, configuration JSON, and CSS
are included; the legacy static prototype remains outside the frontend check.
`make format` fixes Go/frontend findings; `make format-go` and
`npm --prefix web run format` are available separately.

Use the actual base ref with `PROTO_BASE` for a branch targeting something other
than `main`. CI compares against the PR target, or `origin/main` for a push.
Connect transport does not establish authentication, durable evidence, or
production execution. The next bounded slice is the R2 ingestion/storage contract.

## Workflow logging

The executable composes a JSON `slog` completion interceptor. Successful RPCs log
returned state/version and review/plan IDs; rejected actions log the error code,
action, expected version, and requested review ID. Private rationale/evidence and
raw documents are excluded. These are transport outcomes, not a durable audit log.
