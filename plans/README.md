# OCM history implementation handoff

Revised 2026-11-08 against `f234344`, following maintainer acceptance of the
scope review. This package replaces the previous research-gated roadmap.
**Plans 002–004 are implemented and verified locally.** The approved 2026-11-09
initial-import amendment is implemented in [005](005-ocm-history-initial-import.md),
against `5e8bda0`; it replaces automatic first-run collection with a cancellable
setup dialog. Later collection remains in the background. No further research
milestone or contract-approval round is required. See each plan for implementation
evidence and supported-scope limitations.

## Execution order

| Plan | Outcome | Depends on | Status |
| --- | --- | --- | --- |
| [001](001-ocm-history-contracts.md) | Accepted release contract | Existing architecture | DECIDED — research phase closed by scope reduction |
| [002](002-ocm-history-storage.md) | Extraction, archive, config and collection | 001 | DONE locally — fixture oracles, retention/lock/worker/startup tests; normal/mock, vet, race and builds pass (see 002); Linux CI pending |
| [003](003-ocm-history-cleanup.md) | Capture before deletion | 002 | DONE locally — guarded V2 scope, failure/ordering/UI/fake-CLI tests; all local gates pass; V1 scope unsupported; Linux CI pending |
| [004](004-ocm-history-view.md) | History view and archive-only startup | 002, 003 | DONE locally — History navigation, source retry/startup matrix, snapshot and fixture-journey tests; all local gates pass; Linux CI pending |
| [005](005-ocm-history-initial-import.md) | Initial-import consent, progress, cancel and durable completion | 002–004 | DONE locally — migration, cancellation/retry, partial/empty/source-unavailable, guarded deletion, shutdown, UI and mock tests; vet, race, normal/mock builds and mock suite pass; Linux CI pending |

The original roadmap's steps 3–5 correspond to plans 002–004. Implement these
sequentially; the opt-in sequence is now locally complete. History remains disabled
by default. The 003 guard permits supported V2 deletion; unsupported V1 scope remains
blocked when history is enabled. Update this table with implementation and
verification evidence, not just compilation success.

## Paste this to the implementing agent

> Read `AGENTS.md`, `ocm/AGENTS.md`, `plans/README.md` and plans 001–005 completely.
> Plans 002–005 are implemented. Preserve the accepted contract in 001, including
> the initial-import amendment in 005, when making follow-up changes.
> Make routine implementation choices yourself; do not reopen the retired research
> gates. Inspect existing code and prerequisite changes before each increment.
> Preserve normal/mock builds and disabled behavior. Use fixtures and fake
> executables; do not inspect or mutate real user databases. Run each plan's
> checks, update this index with results and report any genuine blockers.
> Do not commit, push, publish, or change dependencies without authorization.

## Authority and historical material

Plan 001, the approved initial-import amendment in 005, and these revised handoffs
are the current product contract. Their
2026-11-08 decisions supersede conflicting instructions in all research Markdown,
previous versions of these handoffs, and ignored `.zed/ocm/HISTORY.md` and
`ROADMAP.md`. `.zed` documents are not prerequisites and need not be supplied to
another checkout. Repository architecture guidance still applies.

`ocm-history-research/` remains a reference library of fixtures, extraction SQL,
and measurements. It is not production code or an active checklist. Its old
proposal, status, native-process requirements, precision rules and performance
gates are superseded. Do not import research packages into production or expand
benchmark work as a prerequisite. The scope review is commentary, not a contract;
in particular its suggestion to capture after deletion is incorrect.

## Deliberately deferred

Exact V1 fork attribution; arbitrary source copies/imports/migration reconciliation;
arbitrary-precision money; persistent deletion intents, epochs and child-process
recovery; hard heap/latency/reader budgets; disk staging; new synthetic scale
campaigns; automatic periodic collection; project/variant filters; other agents.
These are not blockers. Preserve useful existing tests and measurements.

## Verification policy

Use Go 1.25.0 from go.mod (set `GOTOOLCHAIN=go1.25.0` where needed). Each code
milestone runs gofmt on changed Go files, `go vet ./...`, `go test -race ./...`,
`make build`, `go test -tags mock ./...`, and
`go build -tags mock ./cmd/prq ./cmd/ocm`. Focused commands are in each handoff.
Tests use temporary fixtures and injected clocks/launchers. Report unavailable
checks and actual failures; do not claim previous research runs validate new code.
Linux CI checks remain required by repository workflow, but observing a hosted CI
run is not a reason to delay local implementation or require a push. No binaries,
coverage, real databases, profiles or user content in changes.
