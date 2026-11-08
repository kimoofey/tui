# Plan 003: capture retained usage before native deletion

Status: TODO. Planned 2026-11-08 at `f234344`. Priority P1; effort M; risk medium.
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
