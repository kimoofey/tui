# Plan 003: capture retained usage before native deletion

Status: IMPLEMENTED locally on 2026-11-08. Planned at `f234344`. Priority P1; effort M; risk medium.
Depends on completed 002. Read `001-ocm-history-contracts.md`, root/OCM AGENTS and
the actual 002 API before editing. No historic process-recovery gate applies.

## Outcome and scope

Enabled deletion commits complete captures of all known affected sessions before
launching the native delete. Any capture/archive failure blocks that action.
Disabled deletion is unchanged. Retained usage survives successful/partial deletion.

In scope: `ocm/session/history.go` optional impact capability; OpenCode history/
actions adapter tests; `ocm/service/operations.go` and history orchestration/tests;
`ocm/ops.go`, `ops_mock.go`, model availability/messages and tests; cmd composition
only as needed for landed interfaces; `ocm/README.md`, plan/index. No source SQL
DELETE implementation, process supervisor, intent journal, registry, PRQ or new deps.

Drift check: `git diff --stat f234344..HEAD -- ocm cmd/ocm`. Plan 002 changes are
expected; reconcile its actual interfaces before implementing.

## Current anchors

`service/operations.go:Delete` validates the entire refs batch before side effects,
then invokes sequential `source.Delete` calls with a 30s timeout, accumulates errors
and performs one authoritative `s.Load`. Keep this shape and partial-failure behavior.
`opencode/actions.go:runDelete` uses CommandContext/CombinedOutput and WaitDelay.
`sessionArgs` adds --standalone for V2 and storageEnvironment routes the source.
These are the existing native action path; capture must not select another executable
or database. Single and bulk UI commands already call Service.Delete.

The research native trace records recursive V2 deletion but does not prove an atomic
tree-wide transaction. Inspect the supported deletion behavior and adapter fixtures
only as needed to identify known descendants. No broad upstream version survey is
required. The service must receive refs/scope through an optional adapter capability,
not import OpenCode SQL or infer scope from the filtered browser table.

## Implementation sequence

1. Add deletion-impact discovery for supported V1/V2 layouts. Include target and all
   known descendants the native command removes, including children hidden from the
   browser. Handle cycles defensively and deduplicate refs. If impact cannot be
   established for a layout, return unsupported and disable guarded deletion for it;
   do not guess. This does not block collection/UI for that layout. Keep foreign-source
   rejection before any capture/action. Use temporary schema fixtures and existing
   `source_test.go`, `actions_test.go`, V1/V2 tests as patterns.
   Verify: `go test ./ocm/sources/opencode/...` → pass.
2. Replace 002's enabled-delete blocker with a service guard. Validate whole batch,
   acquire the same in-process serializer used by collection, derive scope, capture
   and commit every affected session, then invoke the existing native Delete. Hold
   serialization until action returns/times out and the authoritative reload ends.
   A missing target is an error; incomplete/stale/binding-mismatched capture blocks
   its native action. Existing saved history alone is not fresh capture. Release
   serialization on every error/cancellation. Native failure never rolls back capture.
   Preserve disabled path and its per-call timeout. Capture has its own per-session
   timeout, separate from the native action timeout.
   Verify: `go test ./ocm/service/...` → pass.
3. Bulk behavior: process selected targets sequentially, retaining successful captures
   and collecting independent failures. Before actions, use known impacts to collapse
   selected descendants under a selected ancestor, avoiding a second delete for a
   child already covered. If an ancestor action fails, report that group failed;
   do not automatically retry its descendants. Count successful normalized native
   targets in DeleteResult.Count; explain the count in help/error text if needed.
   Non-overlapping groups can continue. Reload exactly once after the batch as today,
   even after failures; never merely remove displayed rows. Queue one coalesced history
   sweep after any successful native action, after releasing the serializer.
   Verify: `go test ./ocm/service/... ./ocm/...` → pass.
4. Keep confirmation/busy behavior, messages and real/mock paths consistent. Enabled
   capture failures show actionable errors, clear busy state and retain authoritative
   browser selection. Unsupported guard means delete unavailable while other valid
   capabilities remain usable. Vacuum/prune stay separate and do not delete archive
   usage. Document capture ordering, blocked cases, partial bulk results and limits.
   Verify: `go test ./ocm/... ./cmd/ocm/...` and mock equivalent → pass.

## Focused tests

Use service spies and fake executables, never real deletion. Assert event order:
all scope captures committed → native invocation → one authoritative reload.
Test single, descendants, overlapping selection, foreign refs, capture failure,
archive commit failure, native failure/timeout, cancellation and reload failure.
Prove failure before commit never calls native action. Prove archived totals remain
after fake source deletion and reopening the archive. A barrier test with background
collection plus deletion proves neither can publish an older snapshot out of order.
Tests execute tea.Cmd and feed results through Update. Disabled mode uses panic-on-use
history dependencies and preserves existing actions. Fake child cleanup is mandatory.

There is intentionally no surviving-child recovery harness: the release does not
promise coordination after parent death. Document that native writers, new children
after discovery, disabled OCM, separate archives and surviving native processes can
race capture/deletion. Already committed observations survive; uncaptured usage
cannot be recovered by rerunning. Never auto-retry destructive actions on startup.

## Exit checks and handoff

Use Go 1.25.0; gofmt changed Go files. All commands from root must exit 0:
```sh
go test ./ocm/... ./cmd/ocm/...
go test -tags mock ./ocm/... ./cmd/ocm/...
go vet ./...
go test -race ./...
make build
go test -tags mock ./...
go build -tags mock ./cmd/prq ./cmd/ocm
```
Update README/index and record supported deletion scopes, limitations and check
results. Missing CI/tool availability must be reported, not represented as success.

Stop and report if capture failure can fall through to native deletion, source
routing changes, or a supported deletion's known scope is broader than captured.
Unsupported scope is a visible capability restriction, not a demand for a new
research project. Do not expand this plan into process supervision or copy recovery.

## Implementation evidence (2026-11-08)

- Added optional `session.HistoryDeleter` with
  `HistoryDeletionAvailability(ctx)` and `HistoryDeletionScope(ctx, ref)`.
  The OpenCode adapter supports V2 recursive `parent_id` descendants, including
  hidden children, excluding unrelated forks. Scope reads are read-only snapshots;
  duplicate/cyclic traversal is bounded and detected cycles block native recursion.
  Foreign refs and absent targets fail before capture/native action.
- **V1 guarded deletion is explicitly unsupported.** The pinned V1 implementation
  filters recursive children by the CLI's current project, which the source binding
  does not identify. This is the plan's permitted unsupported-scope restriction,
  not a research prerequisite. V1 extraction/browsing/resume remain supported;
  enabled V1 deletion is disabled and a load diagnostic explains why. Disabled
  deletion is unchanged.
- `Service.Delete` validates the full requested batch, joins the history lifecycle,
  acquires a cancellable serializer with priority over background captures between
  sessions, discovers scopes, normalizes selected descendants, freshly captures and
  commits each affected session, then invokes the existing native action. Capture
  timeout and native timeout remain separate. All errors block the affected action;
  independent groups can continue. An ancestor failure never retries descendants.
- The serializer stays held through native completion and one authoritative reload.
  The internal loader can suppress collection while guarded reload is serialized;
  one coalesced sweep is requested after successful native actions and release.
  Close cancels/joins both collection and foreground guarded operations before
  entry-point resources close. Snapshot publication cannot overtake deletion.
- Browser normal and history-enabled mock commands use the same service path.
  Failed commands clear busy state, apply the authoritative snapshot, and restore
  surviving marks/cursor. Successful bulk counts identify native session trees.
  Known binding mismatch disables delete availability; unsupported scope exposes a
  diagnostic without disabling browse/resume. Vacuum/prune remain separate.

Role-error investigation found that the prior V2 classifier rejected valid control
message types. Added explicit handling for agent/model/location switches, synthetic,
skill, shell, compaction and idle projections. Assistant costs still come from
assistant projections and compaction costs from usage events; no double counting.
Unknown/malformed discriminators continue to fail the entire observation. Errors
now include schema and native session/message identities without conversation JSON.
This reproduces/fixes a plausible cause of the reported error; no user's database
was inspected, so the exact triggering row is unconfirmed.

Upstream evidence used for the targeted adapter decisions:

- [V2 SessionMessage.Info union](https://github.com/anomalyco/opencode/blob/68bc26b7da1990ce10ffecfbc1e09eb13aa5d824/packages/schema/src/session-message.ts)
- [V2 recursive native remove](https://github.com/anomalyco/opencode/blob/68bc26b7da1990ce10ffecfbc1e09eb13aa5d824/packages/core/src/session.ts)
- [V1 project-filtered children/remove](https://github.com/anomalyco/opencode/blob/v1.2.27/packages/opencode/src/session/index.ts)

New tests cover scope descendants/forks/cycles/missing targets/foreign refs;
control-message classification and unchanged compaction accounting; scope captures
and commits before native invocation and one reload; overlapping selections;
capture/commit/stale/binding/missing-observation failure; native failure/timeout,
cancellation and reload failure; independent bulk continuation; real archive totals
retained/reopened after both successful and partial-failure fake removal; background
capture/deletion ordering with barriers; shutdown cancellation/join of a blocked
native action; and normal/mock UI command completion, marks and busy state. A fake
executable verifies original source routing, V2 --standalone, and no invocation
after descendant capture failure. No real native delete was executed.

During verification an existing CLI subprocess test inherited enabled user config;
it failed at the sandbox's archive-directory permission boundary. The test now
sets temporary XDG config/data paths. A new concurrent source fixture initially hit
SQLITE_BUSY in rollback-journal mode; its writer now uses WAL and a busy timeout.
Both failures are fixed and final runs pass. An earlier development barrier test
was interrupted after exposing a missing fake inventory; its corrected version
passes, including the race suite.

Final local checks, all exit 0 using `GOTOOLCHAIN=go1.25.0` and
`GOCACHE=/tmp/tui-go-cache`:

- `go test ./ocm/... ./cmd/ocm/...`
- `go test -tags mock ./ocm/... ./cmd/ocm/...`
- `go vet ./...`
- `go test -race ./...`
- `make build`
- `go test -tags mock ./...`
- `go build -tags mock ./cmd/prq ./cmd/ocm`

Build commands emitted nonfatal module metadata-cache permission warnings and
exited 0. Hosted/Linux CI has not been run and remains required by workflow.
No dependencies changed, no commits made, and the preexisting untracked scope
review was left untouched. No real user database inspection or native actions.

Remaining release work: plan 004 supplies History navigation, independent
archive/source composition and archive-only startup. The feature is still disabled
by default and not ready for enabled release. Source/native concurrency, new
children after discovery, disabled instances, separate archives, and surviving
native children remain the explicit limits documented in the README/contract.
