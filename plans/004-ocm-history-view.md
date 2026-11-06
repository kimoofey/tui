# Roadmap step 5: deliver History and archive-only startup

Status: TODO, blocked on steps 3 and 4 exit gates. Planned at `0c710a6` on
2026-11-05. Priority P1; effort L; risk medium (startup/state/async behavior).

2026-11-05 prerequisite update: step 2 research can resume; see its
[evidence pack](ocm-history-research/README.md). No timezone, coverage state table,
collection policy or numeric responsiveness budgets have been approved. Keep
production navigation gated on the completed persistence and cleanup milestones.
V1 support with visible fork approximation is decided in
[decisions](ocm-history-research/decisions.md).

Executor: read root/OCM AGENTS instructions and this entire plan. Run
`git diff --stat 0c710a6..HEAD -- ocm cmd/ocm internal/ui`. Review the code anchors
below against current code and completed prerequisite contracts. Update this
plan for expected prerequisite changes; resolve unexplained drift before edits.

Step 2 continuation: [experiment results](ocm-history-research/results.md) and
[native tracing](ocm-history-research/native-trace.md) now exist. These do not
approve remaining signatures, schema, budgets, identity or deletion recovery;
refresh this plan only after those contracts pass step 2's gate.

## Outcome and boundaries

Enabled users can inspect durable daily recorded costs and provider/model totals,
navigate ranges, see freshness/coverage, and reach the archive even after all
sessions are gone or OpenCode is unavailable. Background work remains responsive.
Switching views preserves browser state. Disabled users see today's browser and
startup flow, with no history navigation, archive access, or capture.

In scope: `ocm/history_model.go`, history commands/messages/render helpers and
tests; additive host composition in `ocm/model.go`/`options.go`; range queries in
`ocm/history/`/SQLite implementation if needed; `cmd/ocm` real/mock startup and
ownership tests; `ocm/README.md` and design/index updates. Use existing
`internal/ui` semantic styles; shared changes only if strictly necessary and
verified against PRQ. Proposed filenames must follow the landed step 3 layout.

Out of scope: changing session period-cost totals to archive totals, default
opt-in policy, new source adapters, stacked charts, model repricing, tool telemetry,
subscription billing, arbitrary timezone rebucketing, source/action discovery
refactors, dependencies. Project filtering is included **only** if step 2
approved and step 3 retained sufficient metadata; otherwise explicitly deferred.

## Current code anchors and examples

`cmd/ocm/main.go:runBrowser` currently defers resource closure and then:

```go
if len(opts.Sessions) == 0 {
    fmt.Println("No OpenCode sessions found.")
    return 0
}
```

`composition.go:loadSource` fails if source `Load` fails and closes the source.
These are disabled-mode contracts. Enabled archive-only mode needs an explicit
composition result, not a fabricated successful source snapshot or nil source
passed to `service.New` (its validation/capability methods assume a source).

`ocm/model.go:Model` owns browser table, selection by complete `session.Ref`,
confirmation and busy state. `Init` currently returns
`tea.RequestBackgroundColor`; preserve it when scheduling initial history work.
`Update` locks keys during reload/bulk-delete/vacuum/prune. History collection
must have its own state, not set `m.reloading`. View is side-effect-free.

Current keys: up/k, down/j, pgup/b, pgdn/f, space, enter, r, d, v, p, esc, ?, q,
ctrl+c. Do not steal a browser binding. Choose history navigation only after
documenting its active-view context and confirmation/busy precedence.
`updateKeys` and help must reflect available capabilities per model.

`ocm/browser_test.go` shows fake-source/capability tests;
`ocm/operations_test.go` executes Cmds and returns messages through Update;
`model_test.go` distinguishes explicit empty reload from failure;
`ops_mock.go` uses explicit acknowledgements. Mirror these patterns for new
async flows in both build variants. Reuse Bubble Tea/Bubbles/Lipgloss **v2** APIs
already pinned in go.mod; do not paste v1 examples.

## Prerequisite and UX decision gate

Require step 3's durable archive queries, stored timezone/identity/coverage,
approved synchronization policy and numeric budgets, plus step 4's guarded
cleanup passing all gates. Do not release navigation around unfinished capture.

Before wiring keys, record the following in HISTORY and README: switch key,
History default range, range presets and previous/next bindings, day selection,
back/escape behavior, explicit reconciliation action, current-period partial
label, refresh/progress/error labels, and initial view with no usable source.
If step 2 did not decide these, create a fixture-only presentation prototype
first and settle the decisions here. Recommended first version: total-cost bars
plus a separate provider/model table, no stacked/multi-series chart.

## Increment A — implement archive query and presentation model

Define a history submodel whose inputs are range query results, status and
messages; it never reads OpenCode directly. Query committed contributions with
half-open calendar ranges in the archive's persisted timezone. Sum before display
rounding. Keep currency/provenance explicit; never add incompatible currencies.
Monthly totals are sums of daily contributions, not a new collection granularity.
Known unknown-model amounts remain part of total cost and model reconciliation.

Render daily costs, range cost/active-day summary and provider/model breakdown;
support range navigation and optional day narrowing under the approved UX.
Show numeric recorded values alongside custom Unicode horizontal bars. Scale zero to the
largest visible value; handle all-zero, unknown and invalid data safely. Use
integer cell widths, Unicode-aware measurements, semantic Nord colors and light/
dark adaptation. On narrow terminals shorten labels or omit bars, preserving
readable amounts/status; use a viewport for long ranges and avoid negative sizes.
No chart dependency and no reliance on color alone for state.

Freshness and coverage are independent: display last successful capture, running/
partial/stale/failed/unavailable status, and unknown historical coverage. An
absent bucket is not automatically a measured zero. Current day/month is partial
even after a successful refresh. Historical gaps remain unknown after external
deletion/disabled periods. Do not call recorded cost an exact invoice.

Accounting quality is separate from freshness and extraction completeness.
Whenever a displayed range includes V1-derived approximate contributions, show
“Approximate: V1 forked sessions may include copied usage.” Use persisted quality
metadata, including mixed-source and archive-only views; do not infer quality
from the currently installed OpenCode version. Mark affected breakdowns too.
The warning needs no additional confirmation and is not a collection error.
If a range has no affected contributions, do not attribute this V1 limitation to
it. Document the limitation in `ocm/README.md` when the feature is implemented.

Gate A: `go test ./ocm/history/... ./ocm/...` passes with fixture aggregates for
multiple days/models/providers, unknown model/cost, known zero, no observations,
partial days, multiple currencies and stored timezone/DST boundaries. UI values
reconcile to independent SQLite fixture queries under identical filters. Test
1-column/tiny-height, 40/80/120-column layouts, long Unicode labels and all-zero
bars; no panic, invalid width or hidden sole value representation. Include V1
fork totals (7 recorded / 4 actual-call reference), quality labels on mixed totals,
retention of the warning after source deletion/restart, and a V2-only range
without the V1 warning. Successful refresh must not clear approximation metadata. Test rounding
at aggregate boundaries, not by summing already-formatted strings.

## Increment B — compose views without disturbing the browser

Keep one host Bubble Tea program. Compose browser and history submodels; route
keys to the active view while delivering async source/history results to their
owners regardless of active view. Window size/background-color messages must
reach both. Preserve table cursor, marks, scroll, root filtering, cost-period
selection and valid browser data when switching away/back or resizing. Source
reloads while History is active still update browser state authoritatively.

History query/collection work runs only through Cmds or a lifecycle-owned worker
with messages; no blocking work in Update/View. Tag range queries and collection
events with request/generation identity. Drop stale query responses after range
changes; ignore late events after cancellation/shutdown. Allow only bounded work
and status queues. Explicit refresh schedules approved reconciliation, not a
second uncoordinated collector. Keep old successful range data visible on query
failure with a clear stale/error marker.

Preserve confirmation/busy behavior: switching views must not silently confirm,
retarget or bypass deletion/capture. Define whether switching is blocked or
cancels a pending confirmation; test the chosen rule. Background history never
uses browser reload's input lock. Range navigation/quit work during collection;
cancel/join workers before closing handles. Keep user key/help definitions in
sync, and omit all History bindings/help when disabled.

Gate B: both normal/mock focused suites pass. Tests execute returned Cmds, apply
results with Update, then inspect state/View. Cover view-switch state retention,
resize, inactive-view updates, rapid range changes with out-of-order responses,
progress followed by newer success then older failure, collection cancellation,
query failure, explicit refresh coalescing, capability restrictions and pending
delete confirmation. Browser navigation remains usable while a fake extraction
is blocked on a barrier.

## Increment C — support archive-only startup and independent failures

Refactor composition only as needed to represent source and archive availability
independently, preserving entry-point resource ownership. Do not require an
OpenCode executable/database merely to open the archive. Resource initialization
must not block saved-history display on full source discovery/backfill. If
discovery itself is slow, move it to cancellable work for enabled startup while
keeping the disabled path intact.

Implement this matrix using step 2's explicit archive-failure policy:

| History | Source | Archive | Expected behavior |
| --- | --- | --- | --- |
| Disabled | Usable sessions | Any old archive | Existing browser, no history calls/navigation. |
| Disabled | Empty | Any | Print existing no-session message, close resources, exit 0. |
| Disabled | Unavailable/load failure | Any | Existing source error/exit behavior. |
| Enabled | Usable sessions | Healthy | Browser plus reachable History; collection independent. |
| Enabled | Empty | Healthy saved history | Enter History; browser empty state truthful. |
| Enabled | Unavailable executable/DB | Healthy saved history | History accessible; source failure distinct; no source actions. |
| Enabled | Browsing supported, usage unsupported | Healthy saved history | Keep browser/resume and other supported actions; History reports unsupported collection; required capture blocks enabled deletion. |
| Enabled | Empty/unavailable | New empty archive | Reachable History empty/unknown-coverage state; no invented zero history. |
| Enabled | Usable sessions | Failed/corrupt/newer schema | Approved explicit error/degraded policy; no unprotected enabled deletion. |
| Enabled | Unavailable | Failed | Show/return both actionable failures; no fabricated usable archive. |

Do not convert an optional requested `--cost-period` source failure into a
successful zero total. If History is available while source startup fails,
retain that requested-stat failure as a source diagnostic. Never display archive
totals under the source-only period label. Distinguish absent source from an
authoritative empty source snapshot. Retry source loading only through the
approved action/policy, without rediscovering a different action target silently.

Every path (including help/version, failed opens, partial initialization, empty
states, program errors and quit) must close each acquired resource exactly once,
cancel/join workers, and release or preserve coordination according to the
approved unfinished-deletion recovery policy. Surface unresolved lineage,
ambiguous accounting reconciliation and unfinished native operations through
their approved diagnostics/recovery paths; do not silently retry protected work.
Mock archive-only states use
fixtures and fake failures; no real config/source data access.

Gate C: expand `cmd/ocm/composition_test.go` for the whole matrix and resource
ownership; process-level CLI tests retain disabled flags/error/exit behavior.
Use the injected program runner for archive-only model assertions without a
real terminal. Source failure never hides a healthy archive; archive failure is
never reported as source failure. `--help`/`--version` pass with malformed config
and missing source/archive. Run normal/mock command tests on Linux CI as well
as command-level macOS checks; no graphical apps are required.

## Increment D — release documentation and end-to-end fixtures

Update `ocm/README.md` with config path/example/default/restart behavior, archive
location and retained metadata, History keys/ranges/timezone, recorded-cost and
coverage meanings, how verified corrections can revise totals and how ambiguous
record loss is reported, collection triggers, archive-only behavior and error/recovery
instructions. Include disabling/re-enabling and unrecoverable gaps. Preserve
existing browser keys and `--cost-period` documentation. Align HISTORY/ROADMAP
with the decisions actually implemented, not obsolete proposal wording.

Create one deterministic fixture journey: enable → backfill multi-day/model data
→ navigate History → capture/delete parent with descendants → restart with no
source → retained totals unchanged → restore an accessible empty source fixture
→ disable with archive present → existing no-session exit → re-enable → same
totals. Separately verify disabled startup with an unavailable source returns
the existing source error. Also simulate an external deletion
before first observation and show unknown coverage rather than fabricated data.
Exercise the journey through services/commands/Update without a real terminal.

Gate D: fixture queries reconcile with all visible total/model/day values; the
journey passes in real and mock variants with no real OpenCode/terminal apps.
Measure time to usable browser/saved History, input latency during collection,
range-query latency and cancellation against step 2's numeric budgets. Report
manual visual checks separately from automated state/render checks. Optional VHS
previews use mock binaries; missing VHS is not a substitute for model tests.

## Exit gate

Use Go 1.25.0, gofmt changed Go files, then run from repository root:

```sh
go test ./ocm/... ./cmd/ocm/...
go test -tags mock ./ocm/... ./cmd/ocm/...
go vet ./...
go test -race ./...
make build
go test -tags mock ./...
go build -tags mock ./cmd/prq ./cmd/ocm
```

All exit 0. Disabled regression matrix passes with panic-on-use history factories
and untouched existing archive; no History help/key appears. Review UI width,
state and async race evidence, query reconciliation and startup ownership tests.
No generated binaries/profiles/raw source data in the diff. Update roadmap and
plan index to DONE only with passed gates and recorded platform limitations.

Stop if archive-only startup requires fabricating source capabilities, range
queries confuse absent observations with zero, current-flow changes leak into
disabled mode, or background work blocks input beyond the approved budgets.
Future filters and timezone changes must respect retained metadata and bucket
granularity; never promise reconstruction the archive cannot support.
