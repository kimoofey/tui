# Plan 004: deliver the History view and archive-only startup

Status: IMPLEMENTED locally on 2026-11-08. Planned at `f234344`. Priority P1; effort M; risk medium.
Depends on 002 and 003. Read the accepted contract in 001 and root/OCM AGENTS.
This completes the releasable opt-in feature; no performance research gate remains.

## Outcome and scope

Users can see retained UTC daily costs and provider/model totals, including after
all sessions or the source disappear. Disabled startup and browser behavior remain
unchanged. Use existing semantic Nord styles and Bubble Tea/Bubbles/Lipgloss v2.

In scope: `ocm/history_model.go` plus commands/messages/rendering/tests; host model/
options/help integration; history range queries if needed; `cmd/ocm/` real/mock
composition and lifecycle tests; `ocm/README.md`, plan/index. No chart dependency,
shared UI redesign, PRQ, project/variant filter, timezone settings or source-period
cost replacement. Drift check: `git diff --stat f234344..HEAD -- ocm cmd/ocm`;
expect prerequisite changes and use their actual API.

## Current anchors

`cmd/ocm/main.go:runBrowser` currently exits with “No OpenCode sessions found.”
when opts.Sessions is empty. Preserve that only for disabled history.
`composition.go:loadSource` closes the source on Load error; enabled composition
must represent archive and source availability separately. Never pass nil to
`service.New` and then call methods that assume source.Info exists.

`ocm/model.go` owns keys, table selection, busy/confirmation state and Init's
tea.RequestBackgroundColor. History must have independent query/collection state,
not set browser `reloading` for background work. Existing commands in `ops.go` and
`ops_mock.go` differ; tests must cover both. Examples: `operations_test.go` executes
Cmds and feeds messages to Update; `browser_test.go` tests capabilities;
`cmd/ocm/composition_test.go` injects startup dependencies/program runners.

## Implementation sequence

1. Implement committed archive queries for half-open UTC month ranges and all-time,
   with daily totals, provider/model totals, unknown counts, unbucketed count and
   persisted V1 quality. Read all components of a response in one short archive read
   transaction, so concurrent publication cannot produce contradictory totals/status.
   SQL GROUP BY over usage is sufficient; no aggregate cache.
   Render “Recorded USD (approximate)”, UTC, numeric amounts beside horizontal bars,
   model breakdown, last capture/status. Sum before formatting. Use maximum visible
   cost for bar scale, safe zero widths, viewport/scroll for many days/models, and
   readable amounts on narrow terminals. No-data means “No captured usage”; unknown
   cost is not zero. All-time includes unbucketed spend outside dated bars. State
   current-period partialness and limited observed coverage. Keep previous successful
   results visible with stale/error status on query failure.
   Verify: `go test ./ocm/history/... ./ocm/...` → pass.
2. Compose browser and History in one program. Enabled `h` switches; disable switching
   during confirmation or foreground mutation. In History: [/] previous/next UTC month,
   0 current month, a all-time, r refresh, up/down/j/k scroll, esc/h browser when usable,
   ? help, q/ctrl+c quit. From all-time [ or ] returns to current month. No usable
   source means esc/h stays in History. Preserve browser cursor/marks/scroll on switch
   and resize. Route async responses to their owning state even while inactive.
   Tag range queries with request IDs and ignore old responses; collection errors
   must not overwrite newer success. Resize/background-color messages reach both.
   Refresh uses 002's coalescing collector; startup runs one sweep. Archive-only r
   retries source discovery/load then capture; errors leave saved data visible.
   Verify: normal/mock `go test ./ocm/...` → pass.
3. Refactor enabled composition to open archive and acquire its lock independently
   of source discovery. Do not wait for backfill to show saved data. Source discovery/
   loading may run asynchronously so a slow source does not block saved-history use.
   Keep entry-point ownership with cancel/join/close exactly once on every failure
   and exit. Source retry obeys binding/routing policy; a different source does not
   become silently bound. No fake capabilities or zero source period costs.
   Verify: normal/mock `go test ./cmd/ocm/...` → pass.
4. Finish README: config/default/restart, archive path/metadata, keys/UTC, quality and
   coverage, corrections and stale retention, binding mismatch, one-instance lock,
   capture-before-delete, unsupported scopes, external/disabled/crash gaps and source
   retry. Remove the intermediate 002 deletion restriction. Preserve --cost-period
   documentation. Run the fixture journey and exit checks.

## Startup matrix (required tests)

| Mode | Source | Archive | Result |
| --- | --- | --- | --- |
| Disabled | Usable/empty/failed | Any | Existing behavior; no history calls or keys |
| Enabled | Sessions | Healthy | Browser plus reachable History; background collection |
| Enabled | Empty | Healthy/new | History with saved data or no-captured-usage state |
| Enabled | Missing executable/DB or load failure | Healthy/new | History; source error visible, source actions unavailable |
| Enabled | Usage unsupported or binding mismatch | Healthy | Browser/resume when valid; saved History; capture/delete unavailable |
| Enabled | Any | Corrupt/newer/unwritable/locked | Clear startup error, resources closed, no unguarded deletion |

Help/version succeed without config/source/archive even with malformed config.
Mock mode uses injected deterministic data; it must not open the user's archive.
Source retry from unavailable → usable must recover supported capabilities without
losing saved state; failed retry preserves the prior archive view.

## Focused tests and fixture journey

Rendering: UTC midnight/month/year boundaries, multiple models/days, known zero,
unknown cost/model/time, all-zero bars, V1 warning persisted after restart and V2-only
range without that warning. Check narrow and normal widths, long Unicode names and
small heights for no panic or unusable negative geometry. A barrier test commits a
capture between component queries and proves the response stays on one archive
snapshot. No exact latency threshold.

Async: execute Cmds then Update; navigate during barrier-blocked collection, rapid
range changes with reversed responses, refresh coalescing, failure retaining old
results, source retry, inactive-view updates, confirmation precedence and cancellation.

End-to-end fixture: enable → collect multi-day/model source → refresh unchanged →
same totals → guarded fake deletion including descendants → source empty → restart
with missing source → same totals/quality in History → disable → existing empty/error
startup behavior → re-enable → same retained totals. Separately exercise deletion
before first capture: no invented history. Use temporary SQLite and fake source/CLI,
no graphical terminal or live credentials. Test normal and mock seams.

## Done criteria

Use Go 1.25.0, gofmt changed Go files; from repository root all exit 0:
```sh
go test ./ocm/... ./cmd/ocm/...
go test -tags mock ./ocm/... ./cmd/ocm/...
go vet ./...
go test -race ./...
make build
go test -tags mock ./...
go build -tags mock ./cmd/prq ./cmd/ocm
```
Record checks/limitations in the handoff report and update index. No generated
binaries/screenshots/raw user data committed. Optional VHS/manual preview is useful
but not required; don't launch real native actions. Report hosted CI separately.

Stop and report if healthy archive access requires a live source, disabled mode
accesses history, UI work blocks in Update/View, or enabled deletion bypasses 003.
Routine layout and API adjustments are implementation work. Do not resurrect retired
resource measurements, native recovery, copy-resolution UX or broad schema research.

## Implementation evidence (2026-11-08)

Implemented the opt-in History view and independent archive/source lifetime:

- `service.HistoryBackend` exposes Query and Refresh; `HistoryBrowser` owns the
  enabled archive, source discovery/retry, service worker, cancellation/join and
  idempotent close. `BrowserState` represents a usable source separately from its
  error. Failed discovery/load leaves Query usable and closes failed source
  resources. No nil source is passed to Service.New. Discovery has a cancellable
  adapter entry point, preserving its five-second safety timeout.
- Enabled composition opens the archive/lock first and returns immediately.
  Model.Init commands query saved accounting and discover/load the source
  independently. A slow/missing source never blocks archive reads or rendering.
  With sessions, successful initial discovery opens the browser unless the user
  has already interacted with History. Empty/unusable sources stay in History.
  Disabled startup retains its existing discovery, error and no-session exit.
- `ocm/history_model.go` supplies current UTC month/all-time ranges, daily horizontal
  bars and numeric amounts, provider/model totals, unknown/unbucketed counts and
  unbucketed spend, persisted last capture/status and V1 quality. Bars scale to the
  visible day region; zero/unknown costs avoid invalid widths. Metadata and long
  Unicode names wrap and scroll; final rendering constrains both terminal axes.
  Coverage/current-period notices and no-captured-usage states avoid claiming zero
  spend from absence. Failed queries retain previous results and their range label.
- `h` switches browser/History when enabled and is ignored during confirmation or
  foreground mutation. History supports [ / ], 0, a, r, arrows/j/k, esc/h, ?, and
  q/ctrl+c. All-time [ / ] returns to the current UTC month. Switching/resizing keeps
  browser cursor/marks; refresh restores surviving refs. Source refresh is separate
  from browser reloading and collection; browser mutations wait for refresh to
  complete. Source recovery restores valid source capabilities; unavailable-source
  views cannot escape into source actions.
- Range queries and source responses carry request IDs; collector messages carry
  source identity and monotonic revision. Old/inactive-view responses cannot replace
  newer results or source state. Refresh coalesces to one pending source request;
  the existing collector still coalesces sweeps. Querying alone never collects.
- Archive reports still use one read transaction and SQL GROUP BY. Added nullable
  UnbucketedCost and a private snapshot-test seam (not a public runtime callback).
  An independent WAL writer commits between component queries in a barrier test;
  totals, breakdown and stored status retain the original reader snapshot.
- Mock builds offer deterministic in-memory accounting and History navigation;
  no user config/archive/source is read and no native executable is used. Injected
  real fixture services exercise normal/mock UI guarded-action seams as before.

New tests cover month/year/UTC boundaries and half-open ranges; missing timestamps;
unknown versus zero; persisted quality rendering; all-zero bars; narrow and tiny
terminal geometry; long Unicode model names; initial command execution and response
routing; reversed range responses; failed queries retaining results; source retry;
refresh coalescing; inactive-view updates; source/revision stale responses;
confirmation precedence and browser selection across switch/resize.

Startup tests cover usable, empty, missing and failed sources independently of
healthy archives; corrupt/newer/locked/unwritable archives fail before discovery;
binding mismatch preserves saved reports and disables guarded deletion; unsupported
collection keeps a usable browser; blocked discovery allows saved queries; shutdown
cancels/joins work; repeated Close closes resources once. Existing help/version and
disabled factory tests remain passing. Mock config is deliberately malformed in a
test to prove the mock preview bypasses user files.

The fixture journey collects multi-day/model usage, refreshes unchanged without
double-counting, captures before fake deletion including descendants, reloads an
empty source, closes/reopens the archive and restarts with a missing source. Totals
and daily/model attribution survive. Separate empty-coverage/retry fixtures prove
that usage removed before first capture is not invented. Earlier plan tests retain
V1 persisted-quality and disabled empty/error regression coverage. No real user
databases or native actions were accessed.

During development, startup rendering tests initially omitted the terminal-size
message and expected a full heading at width zero. The fixture now supplies a
normal resize before assessing rendered content. Final verification passes.

Final local verification, all exit 0 with Go 1.25.0 and
`GOCACHE=/tmp/tui-go-cache`:

- `go test ./ocm/... ./cmd/ocm/...`
- `go test -tags mock ./ocm/... ./cmd/ocm/...`
- `go vet ./...`
- `go test -race ./...`
- `make build`
- `go test -tags mock ./...`
- `go build -tags mock ./cmd/prq ./cmd/ocm`

Build commands emitted nonfatal module metadata-cache permission warnings and
exited 0. Hosted/Linux CI has not been run; repository CI remains required. No
manual VHS preview was needed; rendering/commands were exercised with fixtures.
No dependencies changed or commits created. The preexisting untracked scope review
was left untouched. The opt-in implementation sequence is locally complete;
default remains disabled. V1 guarded deletion remains explicitly unsupported as
documented in 003; the accepted source/native concurrency and crash limits remain.
