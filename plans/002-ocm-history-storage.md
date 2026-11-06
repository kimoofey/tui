# Roadmap step 3: add opt-in archive storage and bounded collection

Status: TODO, blocked on step 2's exit gate. Planned at `0c710a6` on 2026-11-05.
Priority P1; effort L; risk high (durable accounting, concurrency, startup).

2026-11-05 prerequisite update: step 2 can continue under the accepted V1 fork
approximation; see [decisions](ocm-history-research/decisions.md) and the
[evidence pack](ocm-history-research/README.md). V1 stays supported. Identity,
reconciliation authority, deletion recovery, archive schema/path and budgets
remain unfinished; production implementation still requires step 2's exit gate.

Executor: read root/OCM AGENTS instructions and this entire plan. Run
`git diff --stat 0c710a6..HEAD -- ocm cmd/ocm go.mod`. Compare the anchors below,
then read the completed step 2 contract. Expected prerequisite changes require
updating this plan, not discarding them. Do not invent unresolved policy.

Step 2 continuation: [experiment results](ocm-history-research/results.md) and
[native tracing](ocm-history-research/native-trace.md) now exist. These do not
approve remaining signatures, schema, budgets, identity or deletion recovery;
refresh this plan only after those contracts pass step 2's gate.

## Outcome and scope

Configuration enables an independent OCM-owned SQLite archive and resumable
collection under the approved step 2 policy. Repeat collection never duplicates
the same native usage record; V1 fork copies with distinct IDs remain included
under the accepted approximation; failed or partial collection never destroys the last complete snapshot;
captured usage survives external source deletion. This step does **not** promise
capture before cleanup, expose History navigation, or change no-session startup.
Document that interim limitation clearly for anyone enabling the feature.

In scope: new `ocm/config.go` and tests (or a small OCM-local config package),
`ocm/session/usage.go`, OpenCode extraction in `ocm/sources/opencode/usage.go`,
new `ocm/history/` domain/collection code and `ocm/history/sqlite/` storage,
`cmd/ocm/{main,composition,data,data_mock}.go` and tests, minimal additive OCM
options/lifecycle plumbing, `ocm/README.md`, and design records/index updates.
These package/file names are proposed; reconcile them with step 2 before edits.
Keep SQL out of the UI and service. Keep the collector free of Bubble Tea.

Out of scope: deletion behavior, existing period-cost SQL, source DB writes,
source migrations/indexes, new navigation/keybindings/charts, archive-only
startup, other agents, shared UI/terminal refactors, PRQ changes, dependencies.

## Current code anchors

`cmd/ocm/main.go:run` parses flags, handles version, validates cost period, then:

```go
opts, err = loadData(*sessionsOnly, costPeriod)
```

`--help` exits during flag parsing. Insert config loading after help/version and
existing argument validation but before source/history composition. No flags or
environment variables for enabling history. Config is read once, not on reload.

`cmd/ocm/composition.go:loadSource` creates the service, calls `Load`, closes the
source on failure, and assigns `opts.Close = source.Close`. `runBrowser` defers
Close even for empty sessions and exits without running the UI when empty.
Preserve those disabled-mode contracts and test expanded enabled ownership.

`prq/config.go` is the local convention exemplar: `GlobalConfigPath` uses empty
XDG fallback and `yaml.NewDecoder(...).KnownFields(true)`. Reuse the pattern, not
the PRQ package. yaml.v3 and modernc SQLite are already direct dependencies.
Do not copy PRQ's unrelated flags, config scaffold, defaults, or CLI precedence.

`ocm/session/source.go` uses optional capability assertions. `session.Source`
must still require only Info/List/Close. `Store` has a single read-only browser
connection. Compose a separate extraction reader from the **same resolved
storage context**; rediscovery can select a different executable/database.

Tests to follow: `cmd/ocm/composition_test.go:TestSourceOwnership`,
`cli_test.go:TestCLIFlagsAndFailures`, `ocm/service/service_test.go` fake sources,
and OpenCode V1/V2 temporary fixtures. Minimal listing schemas must still browse
even if usage extraction is unsupported. Mock startup must never use real
OpenCode or a real archive.

## Prerequisite gate

Before coding, obtain the completed step 2 decisions: exact capability signatures,
accounting/version matrix, timezone, durable identity/ambiguity policy, archive
path/schema, completion/snapshot boundaries, numeric budgets, triggers, cross-
process coordination (including a surviving deletion child), recovery,
reconciliation authority for decreases, freshness/coverage, and archive failure policy.
Refresh this plan with those facts. A recommendation or unresolved TBD does not
satisfy the gate. Read-only source validation and no conversation retention are
mandatory regardless of the chosen policy.

## Increment A — implement configuration and prove the off path

1. Add a small strict YAML configuration with default `history.enabled: false`.
   Resolve `$XDG_CONFIG_HOME/ocm/config.yaml`, falling back to
   `~/.config/ocm/config.yaml` for unset/empty XDG. Missing/empty file and omitted
   setting disable history. Parse known fields strictly, return path/field-aware
   errors, and reject trailing YAML documents or invalid structures rather than
   silently ignoring them. Specify/test null and quoted-boolean handling so
   only the accepted true value enables collection. Do not auto-create config.
2. Load it in CLI composition after help/version/argument processing. Pass a
   resolved config value through both real/mock wiring. The disabled branch
   must not resolve/open/stat/migrate the archive or construct a usage reader,
   coordinator, history lock, or scheduler. Config access itself is allowed.
3. Add injected factories that fail or panic if called while disabled. Tests
   exercise startup, reload, deletion, vacuum, prune, and `--cost-period` through
   ordinary paths. Preserve existing source errors and empty exit.

Gate A: `go test ./cmd/ocm/... ./ocm/...` and
`go test -tags mock ./cmd/ocm/... ./ocm/...` pass. Configuration tables cover
missing/empty/omitted/false/true, unset/empty/custom XDG, malformed/unknown nested
fields, duplicate keys, wrong types, trailing documents and read errors.
Process tests show help/version succeed with malformed config and inaccessible
archive, without discovery. Existing CLI flags/output remain correct. Tests
isolate config/home/data/state paths from the developer machine; update existing
CLI subprocess fixtures that currently inherit `os.Environ()`.

## Increment B — implement archive schema and transactional replacement

Implement the approved schema and migrations only on the OCM archive. Suggested
responsibility split: `ocm/history` types/policies; `ocm/history/sqlite` SQL,
transactions/migrations/querying. Narrow interfaces let collection tests use
fake storage and source tests remain independent of UI.

The persisted model must support:

- Archive schema/accounting versions, persisted timezone and identity lineage.
- Per-contribution accounting quality and reason: all V1-derived usage is marked
  approximate because fork copies may be included. Preserve it across restart,
  deletion, mixed-source queries and migration; successful scanning does not clear
  it. Aggregate queries propagate affected contributions' quality to the result.
  Distinguish this accepted approximation from incomplete extraction, unknown
  cost, unresolved reconciliation and source identity errors.
- Per-source/session/day/provider/model contributions; variant/project dimensions
  only if approved. Currency/provenance and unknown-value flags must survive.
- Capture/completion generations and per-session/sweep progress, timestamps,
  diagnostics and observed coverage. Persist no title, messages, tool output,
  file content, raw JSON, or directory labels unless explicitly approved.
- Efficient range/daily/model queries over committed contributions. Query keys
  must preserve unknown dimensions; SQL NULL uniqueness must not break upserts.

Read a **complete approved replacement scope**, validate the approved authority
to reconcile that scope, then transactionally remove its
old contributions, insert new rows, and update completion/progress. Rollback
every mutation if any part fails. A complete empty read may replace old usage
only when the approved rule establishes an authoritative correction to zero.
Compaction, pruning, projection loss or unexplained disappearance of usage is
not sufficient evidence. Implement step 2's conservative retention/unresolved
policy for ambiguous decreases, and persist its diagnostics/provenance atomically.
An absent session, bounded page, read failure or cancellation may not erase usage.
Missing sessions in discovery retain archived contributions. Do not cascade-delete
history rows because source rows disappeared.

Migration failures leave the previous usable archive intact; refuse unsupported
newer versions, never delete/recreate to recover, and test concurrent openers.
Keep schema migrations and ordinary refresh independent of source availability.
Ensure paths cannot collide with the source database. Use the approved local-file
permissions and journaling/checkpoint policy; include companion files in privacy
and capacity checks. Never migrate or change journal mode on the OpenCode source.

Gate B: `go test ./ocm/history/...` passes for fresh schema, reopen, each upgrade,
failed migration rollback, newer-schema refusal, and atomic replacement. Assert
archive rows/queries exactly for idempotent retry, increases/decreases, known zero,
missing values, unknown models, cross-day/model changes and currency separation.
Add a V1 fork fixture expecting 7 recorded versus 4 actual-call reference;
repeat capture must remain 7. Assert quality/reason persistence on reopen, after
source deletion and in mixed V1/V2 queries. A migration alone must not relabel
retained approximate usage as exact or import it a second time.
Distinguish verified downward/zero corrections from record loss in an existing
session; ambiguous loss must not silently reduce captured spend or mark
reconciliation successful. Test replacement projections without duplicate usage.
Force an insert/progress-update failure after deletion of old rows: all prior
contributions and progress remain unchanged. Inspect schema and stored text to
prove content minimization; `PRAGMA integrity_check` returns `ok` on fixtures.

## Increment C — add optional extraction and ordered collection

1. Implement step 2's optional usage contract and instance validation. No browser
   listing method calls extraction. OpenCode's dedicated reader uses `mode=ro`,
   the same discovered instance, context-aware queries and consistent session
   snapshots. Do not set `immutable=1` for live data. Report unsupported usage
   without breaking supported browsing; never fall back to frozen V1.
2. Implement the approved per-source cross-process coordinator **now**, even
   though deletion integration is next. Acquire it before taking a source
   snapshot; hold it through archive publication. Make the guarded operation
   usable by step 4 through capture and CLI completion, including its approved
   parent-death/live-child policy. Bound waiting and define safe recovery; do not
   make unconditional parent-crash release the deletion recovery contract. If
   step 2 approves narrower ordering, implement its exact restrictions and
   evidence requirements. Never publish pre-lock/stale extracted data.
3. Implement changed-first scheduling plus resumable reconciliation, or the
   policy explicitly selected by step 2. Timestamp hints do not replace full
   reconciliation without proof. Freeze a fair sweep/cursor policy so continuous
   new activity cannot starve older sessions. Persist progress with successful
   replacements; failed work stays retryable. Apply measured session/batch,
   memory, snapshot and cancellation budgets. Staging stays unpublished until
   complete, and abandoned staging is recoverable/reclaimable under policy.
4. Return collection progress/results as data/events through an injected sink
   or queryable status. Generation-tagged results must not regress newer state.
   Coalesce redundant requests and bound queues/progress notifications. Range
   queries see committed generations only, not partial replacements.

Gate C: focused source/history tests pass. Use channels/barriers, not sleeps,
for cancellation during read/write, interrupted huge-session paging, limited-day
reads preserving other days, migration mid-read, and old/new concurrent workers.
Start two helper **processes** against one temporary archive/coordinator, show
that an older extraction cannot publish after a newer generation, then kill the
lock holder and prove recovery. `-race` alone cannot verify process ordering.
This collector-only crash check does not establish deletion-child safety; step 4
must run the live-child scenario. Also test unresolved source identity and its
chosen resolution path: no speculative merge/duplicate import, unchanged browsing,
and consistent persisted lineage on restart.
Also pause extraction and prove browser `Load` completes independently. Source
schema/logical records remain unchanged by collection.

## Increment D — compose lifecycle without changing browser flow

Wire archive/reader/coordinator and the approved trigger policy in `cmd/ocm`.
Keep initial browsing independent of backfill completion; do not put history
extraction in `service.Load` or reuse its input-locking reload path. If approved
policy schedules startup collection, start work only with a bounded lifetime and
cancel/join it before closing resources. No sessions still exits at this stage;
do not leave a collector behind after the exit. Keep mock resources in memory or
temporary fixtures, with the same disabled/ownership semantics.

Resources opened before any failure must be closed exactly once. Test archive
open/migration failure, usage-reader failure, source-load failure, empty startup,
normal quit, program error and cancellation. Apply the approved archive failure
policy with visible diagnostics; never silently disable an enabled history guard.
Document config path/example/default, stored data/location, disabled behavior,
coverage/freshness limits, the V1 fork approximation, and the interim lack of
pre-delete guarantees.

Gate D: both focused normal/mock suites pass and expanded ownership tests prove
no leaked worker/handle on each exit. An existing archive's bytes/logical data
remain untouched through disabled startup and ordinary actions. Re-enabling
reuses it without duplicate contributions. Corrupt/unwritable old archives do not
affect disabled flow. Enabled collection errors do not overwrite displayed
browser snapshots or label unknown coverage as zero.

## Exit gate

Run all commands from root with Go 1.25.0; gofmt changed Go files first:

```sh
go test ./ocm/... ./cmd/ocm/...
go test -tags mock ./ocm/... ./cmd/ocm/...
go vet ./...
go test -race ./...
make build
go test -tags mock ./...
go build -tags mock ./cmd/prq ./cmd/ocm
```

All exit 0. Rerun step 2's reproducible workload with real archive writes and
index/journal overhead; publish file sizes, full-import/refresh/cancel latency,
memory, source contention and time to usable UI against its numeric budgets.
Do not mark DONE if a measured budget fails. No generated binaries/raw records
in the diff. Update README, architecture/design records, roadmap and plan index
with actual commands/results and remind users pre-delete protection lands next.

Stop for unresolved step 2 decisions, incomplete data being used as full
replacement, unavailable process ordering, identity ambiguity being auto-merged,
or source writes required to extract. If source-local usage is unsupported,
report the capability limitation; do not weaken ordinary browsing. Future
accounting changes require explicit archive version/reconciliation strategy so
retained contributions from deleted sessions remain interpretable.
