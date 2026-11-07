# Roadmap step 2: validate usage extraction and settle history contracts

Status: IN PROGRESS (2026-11-05: V1 fork inaccuracy accepted; research can
continue; 2026-11-06 private real-data extraction passes). Full performance
measurement remains an unmet exit gate. See
[evidence pack](ocm-history-research/README.md) and
[verification](ocm-history-research/verification.md). No exit gate is complete.
The accepted V1 exception and next work are recorded in
[decisions](ocm-history-research/decisions.md).
Planned at `0c710a6` on 2026-11-05. Priority P1; effort L; risk high
(accounting mistakes propagate permanently). Depends on completed foundation.

Executor: read root/OCM AGENTS instructions and this entire plan. This milestone
delivers measured contracts and isolated experiments, not a production feature.
Run `git diff --stat 0c710a6..HEAD -- ocm cmd/ocm go.mod` and compare the anchors
below to current code. Expected foundation changes are not automatically blockers;
unexplained contract changes require revising the plan before proceeding.

## Outcome and boundaries

2026-11-05 continuation: the isolated V1/V2 extraction and conservative
reconciliation experiments are implemented; synthetic component measurements are
recorded in [results](ocm-history-research/results.md), with
[native deletion/migration tracing](ocm-history-research/native-trace.md).
The original characterization tests remain unchanged. All A–D exit gates remain
open pending the remaining evidence/contracts; no production wiring is added.

2026-11-06 scale continuation: [V1/V2 pipeline results](ocm-history-research/scale-results.md)
record 30 successful fresh-process samples, independent oracles, memory,
cancellation, controlled writer/checkpoint and headless browser measurements.
The full-source 100k multi-session read exceeds the proposed reader limit;
bounded per-session observations and huge-session staging need an evidenced
contract before scheduling/publication can be frozen. This increment adds no
production collector and does not complete gate C or D.

An executor for step 3 must be able to implement without inventing source
identity, accounting, snapshot completeness, synchronization, or recovery policy.
History remains unavailable in the running application after this step.

In scope: research artifacts and isolated fixture/benchmark helpers under
`plans/ocm-history-research/`; new extraction experiment tests in
`ocm/sources/opencode/` only if they cannot change production behavior; design
records in `.zed/ocm/HISTORY.md`, `ARCHITECTURE.md`, and `ROADMAP.md` when available.
No CLI/composition/browser edits, archive creation on startup, production
collector, deletions, schema migrations, source indexes, new dependencies, or Pi.
Experiments must build without production imports of research tooling.

Daily provider/model totals and OCM-owned SQLite are already agreed. So are
configuration-only opt-in and custom horizontal bars without a chart library.
Synchronization, timezone, durable identity, exact archive layout, metadata, and
navigation remain open. Recommended choices in this plan need an explicit
decision record before downstream implementation; measurements alone do not
approve product policy.

## Current code anchors and conventions

- `ocm/session/source.go`: small optional capability interfaces; required source:

  ```go
  type Source interface {
      Info() Info
      List(context.Context, ListOptions) (Listing, error)
      Close() error
  }
  ```

  Do not add mandatory usage methods to it. The history contract must not import
  Bubble Tea, UI, SQLite, or the concrete adapter.
- `ocm/sources/opencode/sessions.go:openStorage` opens `sqliteURI(path, "ro")`
  with `conn.SetMaxOpenConns(1)`. Never reuse that connection for long extraction.
- `sessionTableContext` prefers V2 whenever present and probes each call. Empty
  V2 is authoritative; frozen V1 is not a fallback.
- `periodCost` V1 sums JSON cost for all `message` rows in `[start,end)` without
  role/session filtering. Do not change that public behavior as part of research.
- `v2PeriodCostContext` includes assistant projections, excludes fork copies
  before fork creation, and adds only compaction `session.usage.recorded.1`
  events using `event.created`. It does not add compaction projections again.
- `source.go:Info` derives runtime identity from the absolute cleaned path URI,
  without symlink resolution. That is not durable history identity.
- `service/period.go` uses local calendar boundaries, Monday-start weeks.
- `v2_test.go:TestPeriodCost_V2` is the accounting exemplar: its mixed fixture
  totals **6.5** over `[1000,3000)`, **3** over `[2000,3000)`, and **0** outside.
  `sessions_test.go:createTestStore` and `v2_test.go:createV2Store` show isolated
  SQLite setup; `testWriter` opens a second writable fixture handle.

## Increment A — establish a reproducible evidence pack

1. Record repository SHA, Go/SQLite versions, OS/architecture, storage medium,
   schema detection results, source table/index shapes, and upstream version/commit
   references. Use Go 1.25.0 from go.mod. Read `ocm/README.md` and existing V1/V2,
   custom/channel, migration, and period tests before adding fixtures.
2. Create `plans/ocm-history-research/README.md` with exact experiment commands,
   synthetic fixture generation instructions and expected totals. Raw source
   data/content must not be committed. Real-database runs are optional read-only
   observations on an available database; do not search a user's home for one.
   Record unavailable measurements explicitly.
3. Pin OpenCode stats and deletion implementations by version/commit for the
   supported V1/V2 layouts. The mutable references in `plans/README.md` are leads,
   not enough to promise version coverage. Record minimum required fields and
   behavior for missing tables/columns, migration during reads, and unknown schemas.

Gate A: `go test ./ocm/sources/opencode/...` passes with existing fixtures; the
evidence pack identifies which claims are repository-verified, upstream-verified,
experimentally measured, recommended, or still unresolved. Documentation-only
work needs content checks, not invented runtime results.

## Increment B — prove accounting and extraction completeness

Build an isolated extractor experiment over temporary schemas. Return aggregate
values and completeness diagnostics without keeping conversation text. Select
only usage fields where possible; measure JSON parsing separately.

Create table-driven fixtures covering:

| Scenario | Required result |
| --- | --- |
| One session, two days, multiple models/providers and variants | Attribute recorded usage per timestamp/model; never assign lifetime cost to the last model. |
| Subagent hidden in browser | Include its usage once; listing/root filters never bound collection. |
| Fork copies | V1: include recorded assistant usage without speculative deduplication; fixture reports 7 versus actual-call reference 4, with approximation metadata. V2: preserve validated fork exclusion; no duplicate parent spend. |
| Assistant + compaction projection + usage/ended events | Count each supported usage source once; current V2 6.5 oracle still holds. |
| Mixed V1/V2 after migration | Only authoritative schema, no frozen duplicate rows. |
| Missing/null/zero cost or model, malformed usage, missing timestamp | Explicit unknown/incomplete diagnostics; known zero is distinguishable; unknown-model amounts reconcile. |
| Verified corrections to old usage | Replace affected contributions, including justified decreases, under the approved accounting rule. |
| Usage disappears from a still-existing session through compaction, pruning, migration or unexplained removal | Absence alone does not justify a decrease; preserve captured spend or report unresolved reconciliation under the approved policy. |
| No rows vs session disappears during scan | Complete known empty differs from missing/failed observation. |
| Interrupted/page-limited/date-limited extraction | Never claim a full session replacement. |
| Concurrent fixture writer during extraction | One defined snapshot or detected invalidation; no silently mixed state. |
| DST forward/backward, midnight, month/year boundary | Exactly one persisted calendar bucket per eligible usage item under selected timezone. |

V1 remains supported under the accepted recorded-usage approximation. Extract
assistant usage for existing sessions using validated timestamp/provider/model
fields; do not guess fork provenance from titles, matching amounts, parent IDs,
or pre-session timestamps. Count distinct native message IDs independently,
including copied fork messages. The fixture's history oracle is **7 recorded**,
with **4 actual calls** retained as the known discrepancy. Ordinary non-fork
fixtures must still match their independent reference exactly. Repeated capture
must remain idempotent; the exception does not permit collector duplication.

Persist the accounting policy and approximation reason, and expose the limitation
when affected contributions are queried, including archive-only mode. Apply the
marker to all V1 contributions because reliable fork detection is not established.
Do not change `--cost-period`. Missing fields, imports with unresolved source
lineage, unsupported schemas, and record loss retain their separate diagnostics
and gates. Accurate V1 fork/import provenance is follow-up work, not a stop
condition for this milestone. See [decisions](ocm-history-research/decisions.md)
for the accepted scope, required tests, and future correction policy.

History is reconciled accounting, not an immutable ledger: verified source
corrections can revise captured totals. A complete read of currently available
rows does not prove that those rows still represent all historical spend.
Investigate supported-version compaction, pruning, projection rebuilding and
usage-removal semantics separately from corrections. Specify the evidence that
authorizes downward replacement, including replacement with zero. If the source
cannot distinguish a correction from loss of historical records, choose an
explicit conservative retention/unresolved-state policy; do not infer correction
from absence or simply take the maximum total. Fixtures must cover corrected
amounts, lost records, replacement projections/events and ambiguous decreases,
including their model/day attribution and double-counting risks. If daily
contributions alone cannot support the chosen reconciliation, revise the retained
metadata/schema contract before step 3, without storing conversation content.

Define a proposed optional capability in the design record, including method
signatures and examples for candidate enumeration, complete session extraction,
source identity evidence, and deletion impact. Avoid exposing raw SQL/path details
to the service. The result must identify source/session, schema/accounting
version, accounting-quality/reason metadata, snapshot scope, completion status,
usage buckets, unknown fields and
missing-session state. A nil slice alone cannot mean complete/empty/not found.
If paging is required, specify staging, stable order/tiebreakers, snapshot
lifetime and publication marker; a cursor alone is not evidence of completeness.

Gate B: `go test ./ocm/sources/opencode/...` passes; an independent oracle checks
daily/provider/model sums and category counters; interrupted reads preserve the
previous complete result. Fixture source schema and logical data are identical
before/after reads. Do not compare changing WAL file bytes as a corruption test.

Resume independent work now: implement the isolated V1/V2 extractor and policy
oracles, trace native deletion/record-loss behavior, and prepare synthetic
benchmarks. Lack of a real-database path does not block these activities. Use the
[decision record](ocm-history-research/decisions.md) for the recommended order;
finish the remaining contracts before starting production step 3.

## Increment C — measure budgets rather than guess them

2026-11-06 maintainer decision: use the available real database plus generated
workloads for scale, replacing the assumed roughly 2 GB real-database prerequisite.
The available private snapshot is 37.24 MiB; repeated extraction and process RSS
are recorded in [real-data evidence](ocm-history-research/real-data.md). No public
database download or arbitrary minimum real-file size is required. Full gate C
remains open. Measure realistic synthetic V1/V2 workloads, including variable-size
JSON and huge sessions, alongside the available real sample. Never call a
synthetic result a real-data measurement. Use a safe SQLite snapshot/backup if copying live data; a plain
database-file copy can omit committed WAL data. Keep real copies private and
outside git. Do not enable immutable mode on a live database.

Include full backfill, warm refresh without changes, new/changed sessions,
corrected old usage with unchanged session timestamp, bounded reads, explicit
full reconciliation, and a single huge session. Capture:

- Dataset shape (sessions, usage rows, JSON bytes, model/day cardinality, indexes)
  and query plans (`EXPLAIN QUERY PLAN`) with no source index modifications.
- Read/parse/group/write wall time, throughput, allocations/peak memory, source
  transaction duration, lock waits, archive file and WAL/journal size.
- Time to usable browser with/without concurrent collection and input response
  latency; separate saved-history query time from full freshness time.
- Cancellation latency, restart work, and source-writer/checkpoint impact.
- Repeated runs and distributions; distinguish warm runs, fresh-process runs,
  and actual cold I/O. Do not purge OS caches or relabel a warm read as cold.

The harness must document exact flags and generated dimensions; the reusable
benchmark invocation is `go test ./ocm/sources/opencode/... -run '^$' -bench
History -benchmem -count=5` if benchmark functions named `BenchmarkHistory...`
are added. Exit 0 with measured results is necessary, not sufficient.

Gate C: publish measured tables, commands, limitations, and **numeric** budgets
for batch work, largest-session extraction, cancellation, initial saved-history
load, full import, refresh, memory, and source-reader lifetime. Set acceptance
thresholds from the agreed responsiveness target and baseline, before step 3.
The local real-data observation resolves source availability, not all required
performance measurements. Missing scale, memory, contention or end-to-end results
remain unmet roadmap gates. Do not mark step 2 DONE from a small real sample or
silently waive those measurements.

## Increment D — settle the implementation contract

Record every item below as decided (with rationale/evidence), or explicitly
blocked. Recommended defaults are proposals, not instructions to invent approval.

| Decision | Required specification / recommended starting point |
| --- | --- |
| V1 fork accounting | DECIDED: support recorded assistant usage without fork deduplication; persist and display the approximation. Exact V1 fork provenance is deferred. Validate schemas and fields as usual. |
| Day boundary | Choose fixed UTC or persisted IANA timezone; define initial selection, portability when timezone data is unavailable, and reject silent rebucketing. Arbitrary timezone changes cannot be exact after timestamps are discarded. |
| Cost/tokens | Currency/provenance, precision/rounding, unknown categories, invalid/nonfinite/negative values, overlapping token counters, count semantics. Never reprice historical usage. |
| Reconciliation authority | Distinguish verified corrections from compaction/removal of records representing historical spend. Define evidence for decreases/zero replacement, ambiguous-loss behavior and any required provenance. Snapshot completeness alone is insufficient. |
| Retained metadata | Minimal source/session IDs plus daily usage; decide variants/project identifiers. Recommend deferring directories/titles/project UI to minimize retention. |
| Durable identity | Distinguish runtime ref from archive lineage; specify same-path replacement, symlinks, copies, relocation, imports, reused session IDs and V1→V2 migration. Neither path nor session ID alone proves lineage. Ambiguity must not auto-merge or auto-duplicate totals. |
| Archive | Exact XDG data/state location, schema/key/index design, schema/accounting versioning, file permissions, backup/recovery and newer-schema refusal. No dependency additions. |
| Triggers | Recommended: show saved data immediately; bounded background backfill/changed-first sweep while open; explicit full refresh; fresh capture before delete. Decide reconciliation cadence/budgets and whether browser reload schedules history. No daemon. |
| Completeness | Full-session snapshot replacement initially; stage exceptionally large sessions if measured necessary. Define preservation outside a bounded scope and interrupted work behavior. |
| Ordering | One per-source coordinator spanning source read → archive commit and capture → CLI completion; must work across processes and define lock aliases, wait/cancel/priority and ownership after parent death. Archive write transactions alone are insufficient. |
| Surviving deletion child | Define behavior when OCM dies after capture commits but its native deletion child remains alive. Require coordination through actual action completion, or evidence and explicit approval for a narrower safe recovery contract; parent death is not completion evidence. |
| Recovery | Progress committed with contributions; failed reads/writes/cancellation keep old rows. Resumable sweeps must revisit old sessions; timestamp hints cannot prove completeness. |
| Deletion scope | Verified version-specific descendant/cascade rules, overlap in bulk, missing sessions and new children during capture; unsupported scope blocks enabled deletion. |
| Coverage/errors | Freshness vs historical coverage vs unknown cost; no rows is not proof of zero. Decide archive failure behavior without falling back to unprotected enabled deletion. |

Settle identity ambiguity early: perfect automatic recognition of copies, moves,
replacements and partial imports is not required. Specify what happens when
evidence cannot resolve lineage: for example suspended collection, explicit
lineage confirmation, or explicit separate-lineage registration. Choose the
supported resolution path, diagnostics, persisted decision and resume behavior;
do not silently select one at runtime. Cover partial overlap/double-counting
policy, unchanged browsing, and enabled deletion when lineage remains unresolved.
An explicit unresolved state and recovery path can satisfy this decision; an
unspecified ambiguity cannot.

Choose coordination only after defining its ownership and recovery contract on
macOS/Linux. Acquire coordination **before opening the extraction snapshot**;
cover publication and native deletion completion. A parent-owned advisory lock
that releases on OCM death is insufficient if the deletion child continues.
`ocm/sources/opencode/actions.go:runDelete` uses `exec.CommandContext` and a 250ms
WaitDelay; it establishes no parent-death termination or surviving-child ownership
policy. The [Go 1.25 exec implementation](https://raw.githubusercontent.com/golang/go/go1.25.0/src/os/exec/exec.go)
implements context watching/cancellation in the calling process. Neither that
watcher nor WaitDelay is an independent supervisor after the caller dies.

Do not prescribe a recovery mechanism here. Require either coordination that
remains effective until the deletion operation can no longer mutate the source,
including any relevant descendants, or evidence supporting a narrower safe
recovery contract. For the latter, pin supported OpenCode deletion atomicity and
intermediate-state semantics, specify permitted competing reads/publications/
deletes and retained-accounting outcomes, and revise the ordering guarantee
explicitly across these plans and design records. Possible accounting damage is
not established by the process-lifetime gap alone. Parent death, lock release,
PID absence or timeout alone must not be assumed to prove action completion.
Define restart diagnostics, unresolved-operation behavior, bounded waits and
background yielding. If leases are considered, expiry without fencing is
insufficient. Do not claim control over unrelated OpenCode writes.

Specify a state table for uninitialized/backfill/running/stale/failed/unavailable
and archive-only states. Include first observation time and limitations; never
label a successfully scanned archive as complete historical billing coverage.

Gate D: add an explicit decision checklist to HISTORY and refresh downstream
plans with exact chosen signatures/schema/path/budgets. Resolve the roadmap's
background-collection wording consistently with the selected policy. A contract
with identity resolution, surviving-child recovery, reconciliation authority or
completeness still TBD does not unlock step 3. Record the expected outcomes of
step 4's parent-death/live-child subprocess test before choosing coordination.

## Exit gate and handoff

- All A–D gates pass with evidence; no outstanding blocking decisions.
- Reproducible fixture commands and version matrix exist; numeric benchmarks and
  limitations are recorded without raw source content.
- Source/browser/CLI production behavior is unchanged; inspect `git diff`.
- If Go test helpers were added: gofmt changed files, run the full code gates in
  `plans/README.md` (normal/mock focused tests, vet, repository race, make build,
  repository mock tests and mock builds), all exit 0. Use Go 1.25.0.
- If only docs changed: validate Markdown links and decision consistency; no
  runtime checks required. Update plan index and roadmap only with actual status.

Stop if supported schemas cannot provide timestamped accounting under the
selected policy, except for the explicitly accepted V1 fork approximation,
durable identity remains ambiguous without a safe policy, cascade evidence is
missing, or measured session snapshots cannot meet budgets. Report the evidence
and revise the design; do not replace the requirement with lifetime estimates.
Future schema/accounting changes must rerun this evidence pack before changing
archive meaning; keep its fixtures as long-term compatibility oracles.

2026-11-07 closure review: Docker Linux/arm64 runtime vet, race, normal builds,
repository mock tests and mock builds now pass. Hosted CI for current changes
remains unverified. The [current closure checklist](ocm-history-research/step2-status.md) separates resolved
items from huge-session completion, remaining performance/native evidence,
actionable recovery and final contract decisions. Step 2 remains IN PROGRESS.

2026-11-07 optimization follow-up: the [profiled research path](ocm-history-research/optimized-collection.md) now
completes all ten generated huge-session stages in 1.37–1.51 s under the unchanged
2 s deadline. Ordinary capture, atomicity and accounting checks pass; macOS/Linux
gates pass. Historical failed timings remain valid for the old implementation.
Sustained contention, actual UI latency, full publication and native/contracts
work remain open. No production collector or policy approval is inferred.

2026-11-07 Gate C continuation: [full pipeline and responsiveness research](ocm-history-research/gatec-measurements.md)
implements the next four measurement tasks without production wiring. Review
measured import/refresh/correction distributions, write-phase input/renderer and
sustained synthetic pressure, blocked waits and near-capacity disk overhead.
Explicit decisions are still needed for supported dimensions, numeric budgets,
scheduler boundary semantics, memory evidence and disk reserve. This is evidence
for Gate C, not accepted policy or a Step 2 exit declaration. Native accounting,
deletion/recovery, identity and exact interface/schema work remain gated as in
[the closure status](ocm-history-research/step2-status.md).
