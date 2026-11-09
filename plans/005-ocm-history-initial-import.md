# Plan 005: explicit initial History import

Approved and implemented locally on 2026-11-09 against `5e8bda0`.
Depends on implemented plans 002–004. This amendment supersedes their automatic
first-run collection behavior; the accounting and deletion guarantees in 001 remain.

## Behavior

After a usable source loads, an incomplete archive offers Start scan (Enter) /
Not now (Esc). Dialog actions use Enter and Esc only; Ctrl+C remains the global
quit shortcut.
Starting blocks normal navigation, shows discovery animation followed by a
processed/total/failed session progress bar, and offers Cancel scan (Esc). Counts are
session-based, without a remaining-time estimate. Cancel waits for the import to
return, preserves committed captures, and allows browsing. Failure offers Retry (Enter) or
Continue with partial history (Esc). Incomplete setup remains visible in History and is
offered again next launch; History refresh restarts it in the current run.

Successful full sweeps, including empty sources, persist completion per archive.
Partial failure, cancellation and capture-before-delete never complete setup.
Automatic sweeps are suppressed until completion. Afterward, existing startup,
refresh and post-delete collection remains in the background. Guarded deletion
continues capturing its complete scope even while setup is deferred. Source failure
leaves saved History accessible with discovery retry. No new flags or config fields.

## Implementation evidence

- Archive schema v2 adds a singleton initial-import completion table, independent
  of source binding and usage. Valid v1 archives migrate transactionally and retain
  accounting; their setup starts pending. Newer versions are still rejected.
- `BrowserState.ImportPending` carries setup state through source responses;
  optional `HistoryImportBackend.Import(ctx)` runs the initial sweep. The browser
  tracks import lifetime and serializes it with refresh; shutdown cancels and joins
  before closing source/archive handles. Import cancellation does not kill the worker.
- `HistoryStatus` exposes phase and processed/total/failed counts. The initial
  import reuses sequential per-session capture, timeouts and deletion serialization.
  Completion is saved only after the full sweep succeeds.
- Bubble Tea commands perform imports; ticks update cached progress, and request
  IDs reject stale completion/tick messages. Rendering is side-effect free, adapts
  to terminal size, and bounds long errors so retry/continue actions stay visible.
- The mock preview exercises the same orchestration with an in-memory store and
  simulated captures. It never reads user config/storage or invokes real actions.

## Verification

Tests cover deferral suppressing automatic requests, successful empty import,
completion across reopen, v1 migration preserving accounting, cancellation retaining
commits, idempotent retry, partial failure, unavailable/recovered sources, pending
setup guarded deletion, shutdown cancellation/join, stale UI messages, modal
navigation, progress counts, long errors/resizing, and mock capabilities.

Required local gates use Go 1.25.0 and `GOCACHE=/tmp/tui-go-cache`:
`go vet ./...`, `go test -race ./...`, `make build`, `go test -tags mock ./...`,
and `go build -tags mock ./cmd/prq ./cmd/ocm`: all PASS. A focused mock race
check covering `./cmd/ocm ./ocm/...` also passed. Changed Go files are gofmt-formatted
and `git diff --check` passes.
Builds can emit a nonfatal sandbox-denied module metadata-cache warning; their
exit status determines success. Hosted Linux CI has not been run. Tests use only
temporary fixtures and fake/in-memory sources, without live user databases.

## Maintenance defaults

Retry repeats the idempotent sweep rather than maintaining a persistent resume
queue. Setup belongs to the archive and is unaffected by toggling configuration.
Older OCM versions cannot open schema v2. Coverage remains limited to successfully
observed usage; completion does not promise capture of concurrent external writes.
