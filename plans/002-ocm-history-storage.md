# Plan 002: implement extraction and the opt-in archive

Status: IMPLEMENTED locally on 2026-11-08. Planned at `f234344`. Priority P1; effort M–L; risk medium.
Depends on the DECIDED contract in `001-ocm-history-contracts.md`, not old research
gates. Read that contract and root/OCM AGENTS in full. Implement production code.

## Outcome and boundaries

Durable idempotent collection runs in the background when explicitly enabled.
Disabled startup stays unchanged. Enabled deletion remains blocked until plan 003.
History UI and archive-only startup land in 004; do not release the partial feature.

In scope: new `ocm/history/` store/config files and tests; optional contracts in
`ocm/session/history.go`; `ocm/service/history.go` plus service wiring; OpenCode
`history.go`/tests and read-handle lifecycle; `cmd/ocm/` config/composition/real/mock
wiring; minimal `ocm/` options/command messages for starting collection and reporting
errors; `ocm/README.md`, this plan and index. Exact file splitting is discretionary.
No research edits, PRQ, dependency upgrades, source writes or terminal changes.

Drift check: `git diff --stat f234344..HEAD -- ocm cmd/ocm go.mod`.
Compare changed files with the anchors below. Expected prerequisite work is not
a blocker; investigate incompatible behavior rather than mechanically stopping.

## Current state and boundaries

`ocm/session/source.go` has the required interface:
```go
type Source interface {
    Info() Info
    List(context.Context, ListOptions) (Listing, error)
    Close() error
}
```
Add a small optional history interface, not mandatory methods. Suggested operations
are source descriptor, session inventory and complete session capture with context.
Return accounting-only records, explicit presence/completeness and schema/policy.
Session types cannot import SQLite, Bubble Tea or the concrete adapter. The service
can depend on history store interfaces; it cannot import OpenCode. Avoid a generic
stream/stage/plugin API. Final Go signatures are a routine implementation choice.

`opencode/sessions.go:openStorage` opens mode=ro with one connection. Give collection
its own read-only handle; never hold the browser's connection. `source.go:Info` uses
a path URI as runtime identity; archive binding canonicalization is separate.
`cmd/ocm/data.go:loadData` discovers a source and delegates to `loadSource`.
`composition.go:loadSource` closes on Load failure. `runBrowser` owns opts.Close.
`ocm/model.go:Init` currently returns `tea.RequestBackgroundColor`; preserve it.

Research `extract.go`/`extract_test.go` contain reusable SQL/field-validation ideas.
They are examples, not drop-in production APIs. In particular reject uncertain
eligibility rather than silently accepting a skipped malformed row. Fixture patterns:
`ocm/sources/opencode/sessions_test.go:createTestStore`, `v2_test.go:createV2Store`,
`ocm/service/service_test.go`, `cmd/ocm/composition_test.go`.

## Implementation sequence

1. Add strict config loading, with injectable paths and filesystem dependencies.
   Missing config/field disables history; invalid/unknown fields fail. Help/version
   bypass parsing. Add tests proving disabled code never calls history factories,
   even with corrupt/unwritable preexisting archives. Mock execution must never read
   actual user config/source/archive; expose test injection or deterministic fixtures.
   Verify: `go test ./cmd/ocm/... ./ocm/history/...` → pass.
2. Implement archive initialization, private permissions, lifetime nonblocking lock,
   binding metadata, session/contribution tables, per-session transactions and UTC
   GROUP BY queries. Use float64 and nullable fields; sum unrounded values. Implement
   the exact reconciliation rules in 001: same-key corrections accepted; missing old
   keys or known-to-unknown regressions retain the whole old session as stale.
   Unknown/invalid schema or failed initialization never resets data. Do not bind an
   empty archive until successful source capture and commit. Add a test that a second
   process cannot acquire the lock, then can after the first closes/exits; this is a
   small lock test, not a native child recovery harness.
   Verify: `go test ./ocm/history/...` → pass.
3. Implement optional OpenCode history extraction on a dedicated mode=ro connection.
   Probe schema inside the same read transaction as each complete session. Include
   hidden children; V2 remains authoritative when empty. Persist accounting only.
   Validate V1 7/4 and V2 6.5/3/0 oracles, multi-day/model attribution, unknown vs zero,
   missing timestamps, malformed eligibility, duplicate IDs, absent vs empty session,
   cancellation and source iteration failure. Read transactions must close on errors.
   Verify: `go test ./ocm/sources/opencode/...` → pass.
4. Add service-owned serialization and lifecycle: one sequential collector, at most
   one coalesced pending sweep, cancellation and 30s per-session timeout. Capture and
   publication share the serializer; never let a stale observation publish later.
   A failed/stale session preserves old data and does not abort unrelated sessions.
   Start collection asynchronously on enabled startup and successful explicit browser
   reload. Keep browse/resume usable. Until 003, service Delete must reject enabled
   deletion with a clear “history deletion guard not implemented” error; disabled
   deletion is unchanged. UI availability reflects this but service enforces it.
   Entry points cancel/join before close, including failed initialization paths.
   Verify: `go test ./ocm/... ./cmd/ocm/...` and mock equivalent → pass.
5. Document opt-in config/path, stored accounting, trigger policy, UTC/approximation,
   stale-session behavior and temporary deletion restriction in `ocm/README.md`.
   Do not claim History navigation exists yet. Run the exit checks below.

## Required regression cases

Archive tests: repeated capture unchanged; new keys once; same-ID cost increase,
decrease/zero and day/model change; missing old key plus new key freezes entire
session; previously known value becoming unknown freezes; deletion/missing session
retains usage; failed commit/cancellation preserves prior state; reopen preserves
quality; binding/schema mismatch writes nothing; unsupported archive version is
untouched. Known-empty observation still records successful capture. Unknown cost,
model and time remain visibly distinguishable from measured zero in query results.

Service tests use fake sources/clock/barriers: navigation can continue during blocked
capture; repeated refresh requests coalesce; cancellation joins workers; one failing
session does not prevent others; disabled factories panic if accidentally called.
No time-sensitive performance thresholds. Do not recreate the research pipeline.

## Exit checks and handoff

Use Go 1.25.0; gofmt changed Go files. From repository root, all must exit 0:
```sh
go test ./ocm/... ./cmd/ocm/...
go test -tags mock ./ocm/... ./cmd/ocm/...
go vet ./...
go test -race ./...
make build
go test -tags mock ./...
go build -tags mock ./cmd/prq ./cmd/ocm
```
Record results/unavailable platforms and actual exported API/schema in this plan;
update index to DONE only with passing local checks or explicitly recorded blockers.
No claim of hosted CI without a run. No source content/binaries in the diff.

Stop and report if preserving existing source routing requires unrelated changes,
source writes are necessary, or supported fixture accounting cannot be reconciled.
Ordinary naming/DDL/UI status plumbing and correcting failing tests are executor
work, not approval gates. Large-session optimization and arbitrary migration/copy
support are deferred; implement explicit failure/status instead.

## Implementation evidence (2026-11-08)

Production APIs now in place:

- `session.HistorySource`: `HistorySessions(ctx)` and `CaptureHistory(ctx, id)`;
  `Observation` explicitly distinguishes present/absent and contains accounting-only
  `Usage` with nullable fields and a canonical `HistoryBinding`.
- `history.ReadConfig/LoadConfig`, `ConfigPath`, `ArchivePath`: strict YAML and XDG
  fallbacks. Real startup parses once; help/version bypass it; mock startup never
  reads user history configuration or files.
- `history.Open`, `Store.Capture`, `RecordFailure`, `Query`, `Close`: SQLite v1,
  `metadata(policy,path,schema)`, `sessions(id,captured,status,v1)`, and
  `usage(session_id,kind,id,timestamp,cost,provider,model,input,output,reasoning,
  cache_read,cache_write)`. Primary key `(session_id,kind,id)`, foreign key to
  sessions, timestamp index, WAL/FULL, private files and lifetime flock. Connection
  pragmas are also in the DSN so cancellation/reconnection preserves them.
- `history.Range` uses half-open milliseconds; nil selects all time. `Report`
  contains daily/model SQL GROUP BY totals, unknown/unbucketed counts, independent
  overflow-checked token totals, stored session status and range-specific V1 quality,
  all in one read transaction.
- `Service.EnableHistory`, `RequestHistory`, `HistoryStatus`, `WaitHistory`,
  `CloseHistory`: sequential worker, capacity-one pending sweep, configurable test
  timeout (30s default), per-session serializer, cancellation/join. Successful Load
  schedules collection. Composition owns join/source/archive cleanup on all paths.
  UI initialization preserves background-color detection and receives completion
  errors through commands/messages. Enabled delete is unavailable and service-blocked.

New fixture tests cover repeated captures; corrections down to zero and day/model
changes; missing identities/known-field regression freezing the full snapshot;
absent and empty sessions; cancellation and forced SQL write failure; retained V1
quality after reopening; binding rejection; unknown fields/zero/overflow; invalid
archives/versions/policies; private database/WAL/SHM/lock permissions; cross-process
lock contention and reacquisition. Adapter fixtures assert V1 7 recorded versus 4
actual-call reference and V2 6.5/3/0, hidden children, malformed eligibility,
duplicate identities, unknown counters, cancellation and query failure. Worker
barriers prove browser loading continues during capture, refresh coalescing,
independent session failures, timeout continuation and cancellation/join. Composition
covers disabled factory non-use and enabled startup/cleanup failures; subprocess
checks prove help/version bypass malformed config. Normal/mock UI tests execute
history commands and feed completion messages through Update.

Local verification with Go 1.25.0:

- `go test ./ocm/... ./cmd/ocm/...`: PASS.
- `go test -tags mock ./ocm/... ./cmd/ocm/...`: PASS.
- `go vet ./...`: PASS.
- `go test -race ./...`: PASS.
- `make build`: PASS.
- `go test -tags mock ./...`: PASS.
- `go build -tags mock ./cmd/prq ./cmd/ocm`: PASS.

Used `GOTOOLCHAIN=go1.25.0 GOCACHE=/tmp/tui-go-cache` because the sandbox denied
writes to the default Go build cache. Build commands emitted nonfatal module
metadata-cache permission warnings and exited 0. No live user databases, native
actions or credentials used. Hosted/Linux CI has not been run; it remains a
repository workflow requirement. No dependencies changed or commits created.

Next: plan 003 must replace the enabled-delete blocker with scope discovery and
capture-before-action under the existing serializer. Plan 004 must supply History
navigation, independent archive/source composition and archive-only startup. This
intermediate feature remains disabled by default and is not release-ready.
