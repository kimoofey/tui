# History decisions and next implementation work

Updated 2026-11-06 against `0c710a6`. No production changes.
This record accompanies steps 2–5 and supersedes their earlier requirement to
solve V1 fork accounting before continuing. It also supersedes conflicting V1
stop conditions in the ignored, local `.zed/ocm/HISTORY.md` and ROADMAP records.
Other existing safety and exit gates remain in force.

## Accepted: keep V1 history with a visible accounting limitation

The maintainer explicitly chose to retain V1 history support and accept that
forked-conversation calculations may be inaccurate, with investigation deferred.
This removes the V1 fork-accounting blocker; it does not declare step 2 complete.

The initial implementation rule is recorded assistant usage, without speculative
fork deduplication:

- On validated V1 layouts, extract assistant usage associated with existing
  sessions, including hidden subagents. Attribute recorded costs/tokens to each
  usage record's validated timestamp and provider/model fields.
- Count distinct native message IDs independently. Copied fork messages may
  therefore count historical usage again. Do not use titles, equal cost/token
  values, parent/subagent relationships or pre-session timestamps as heuristics
  for dropping records. Repeated collection of the same record is idempotent.
- The existing fixture represents actual calls of **4**, recorded usage of **7**.
  The future V1 history extractor must report **7**, marked approximate: **6**
  for provider/model `p/m` and **1** for `q/n`, bucketed by recorded usage time. Retain
  **4** as the independent actual-call reference demonstrating the limitation.
  The existing evidence test only exercises the legacy period API, not history.
- Record accounting-policy version and approximation reason with retained
  contributions. Mark all V1-derived contributions because reliable identification
  of just the forked subset is not established. Affected totals may overstate cost
  and token usage; no numeric error bound or correction factor is known.
- Propagate quality metadata through day/model/range totals, restart, source
  deletion and archive-only operation. Show: “Approximate: V1 forked sessions may
  include copied usage.” A successfully completed scan does not make these totals
  exact. Quality, scan completeness, freshness and unknown cost are separate axes.
- Existing V1 browsing, actions while history is disabled, and `--cost-period`
  remain unchanged. History is still configuration-only opt-in. Approximation
  alone is neither an extraction failure nor a reason to block guarded deletion
  once its other prerequisites pass.

The exception is specific to V1 fork duplication. It does not permit invented
costs/timestamps/models, silently missing usage tables, arbitrary unsupported
schemas, lost records being interpreted as zero, or duplicate collector imports.
Imports with uncertain source lineage still need the identity-resolution policy.
V2 retains its validated fork/compaction rules. A V1-to-V2 migration cannot by
itself clear approximation metadata or justify importing the same usage twice;
step 2 must define lineage mapping and reconciliation for migrated sessions.

## Recommended continuation (not completed implementation contracts)

The original continuation recommendations below predate the 2026-11-06 local
snapshot observation. Source availability is now resolved; the remaining
measurement and contract requirements still apply.

Execute step 2's isolated research now, without waiting for exact V1 fork
provenance or a real-database path:

1. **Finish the V1/V2 extraction experiment.** Validate required tables/columns
   inside the read snapshot. Build independent day/provider/model oracles,
   missing/malformed-field counters and quality propagation checks. Preserve the
   V2 6.5 / 3 / 0 oracle. Add non-fork, fork, compaction, hidden-subagent, migration,
   correction, interruption and calendar-boundary fixtures. Keep the existing
   evidence tests unchanged as characterization tests.
2. **Prototype conservative reconciliation.** Prefer stable per-usage identity
   and minimal accounting metadata alongside daily aggregates, without content.
   Permit correction of the same usage item only under validated source semantics;
   stable identity alone is not proof of correction authority. Absence alone
   cannot erase captured usage. For ambiguous replacement/loss, retain the last
   accepted scope and mark reconciliation unresolved instead of mixing potentially
   overlapping old/new contributions or taking the maximum total. Prove the
   publication rule with fixtures before freezing schema/signatures.
3. **Finish native deletion and migration tracing.** Pin supported layouts and
   versions; establish deletion scope, compaction/removal semantics and surviving
   child behavior. Define explicit unresolved states for ambiguous lineage and
   unfinished native operations. Do not infer native completion from parent death,
   lock release or timeout. These are separate from the accepted V1 approximation.
4. **Build and run synthetic benchmarks.** Measure extraction, candidate retained
   schema/query costs, cancellation, reader lifetime and browser contention.
   Record dataset shape and repeated distributions. Prepare an explicit-path,
   read-only harness for the larger database. No home-directory discovery or raw
   content in artifacts. The absent database only blocks the real-data portion;
   it does not block the fixture work or synthetic measurements.
5. **Settle remaining contracts from that evidence.** Recommended starting points
   remain saved-history-first display, bounded background collection, periodic
   full reconciliation, fresh capture before delete, and explicit resolution of
   source-identity ambiguity. UTC is a simple day-boundary candidate, not a newly
   approved timezone decision. Freeze exact signatures, schema/path, policy states
   and measured numeric budgets before step 3.

The conservative reconciliation approach is a recommendation to validate, not a
claim that existing sources already provide sufficient provenance. Defer accurate
V1 fork handling; do not defer essential source-field validation or safe retention.

## Remaining exit gates and separate maintenance

| Item | Status / action |
| --- | --- |
| Exact V1 fork accounting | Accepted limitation; no longer a step 2 blocker. |
| Supported schema/version matrix | Still required for both V1 and V2; browser compatibility is insufficient. |
| Full extraction, reconciliation and quality oracles | Implement and pass in step 2. |
| Identity, ordering, deletion recovery, schema and policy contracts | Still required before production collection. |
| Real-data and scale measurement | Real snapshot and expanded V1/V2 pipeline/heap/RSS/controlled-contention measurements delivered. Source-wide reader target fails; bounded collection and remaining performance gates stay open. |
| Numeric budgets | Measure and record; no approval inferred from tiny fixture tests. |
| PRQ mock compilation | Fixed in fcf4525; normal and mock builds both pass. |
| Steps 3–5 | Remain gated on their documented prerequisites. |

PRQ mock maintenance was completed in fcf4525: `//go:build !mock` added to
`prq/filter_test.go` and `prq/wait_time_test.go`, mock build infrastructure
created in `cmd/prq/deps*.go`, and mock tests added. Both normal and mock builds
pass (`go test -tags mock ./prq/... ./cmd/prq/...`, `go vet ./...`,
`go test -race ./...`, `make build`, `go build -tags mock ./cmd/prq ./cmd/ocm`).

## Accepted: available real sample plus generated scale — 2026-11-06

The maintainer authorized locating their database, creating a private SQLite
backup, observing extraction, and revising the old roughly 2 GB real-database
assumption. Use the available real sample to validate its actual layout/data and
generated fixtures for controlled accounting oracles and larger workloads.
No downloaded user database is required. The actual snapshot is **37.24 MiB**;
[real-data evidence](real-data.md) records successful repeated V2 extraction and
whole-process RSS. This resolves the missing-path obstacle, not all of gate C.

Generated workloads must include supported V1/V2 accounting shapes, variable JSON
sizes (the real sample has multi-megabyte records), many sessions and a huge
session. Existing 100k-row V1 measurements remain valid within their documented
limits. Scale/live-memory, source-writer/WAL impact, full import/refresh and
end-to-end UI measurements still need evidence and accepted budgets. Real-data
observations cannot establish unrepresented fork/compaction/deletion semantics.
Steps 3–5 and all other contract/verification gates remain gated.

The subsequent [scale experiment](scale-results.md) implements those variable-size
V1/V2 workloads and records 30 fresh-process results. Whole-fixture import fits
the proposed five-second target, but source-wide multi-session extraction misses
the proposed reader limit. Next validate bounded per-session collection and a
huge-session staging/completeness strategy; do not silently relax the reader
budget or infer approval of a scheduler/archive contract from these measurements.

## Deferred V1 accuracy investigation

Investigate pinned native fork/import behavior, preservation of original usage
IDs, timestamp semantics and migration lineage. Add a deduplication rule only
when independent fixtures demonstrate it does not discard genuine calls.

Any later accounting change needs a new accounting-policy version and an explicit
reprocessing strategy. Correct still-verifiable contributions using available
source evidence; do not silently reinterpret old aggregates. Records whose source
was deleted may remain approximate indefinitely. Preserve their policy/quality
metadata and explain mixed-policy totals. Do not promise that a future fix can
retroactively recover exact spend for already deleted sessions.

## Verification for this documentation revision

Implementation follow-up: isolated extraction/reconciliation/candidate-retention
fixtures and synthetic benchmarks now exist; see [results](results.md) and
[native trace](native-trace.md). The wording below records the earlier
documentation-only revision; this follow-up's runtime verification is recorded
separately in [verification](verification.md). Recommendations above remain
unapproved wherever not covered by the explicit V1 acceptance.

Validate relative Markdown file links, trailing whitespace and decision consistency
across `plans/README.md`, steps 2–5, this record and the evidence README. Preserve
the earlier test/benchmark evidence as historical results. No Go files changed;
runtime tests are not required for this revision. On the next code change, run
all verification gates in `plans/README.md` using Go 1.25.0.

## Bounded collection continuation — 2026-11-07

[Bounded collection evidence](bounded-results.md) now measures separate session
snapshots and atomic disk staging across 18 fresh generated workers. Multi-session
100k reads fit the proposed per-reader bound; huge-session staging exhausts the
2 s deadline and safely defers. Completion/liveness and other gate C evidence
remain open. Review the consolidated [implementation contract proposal](proposal.md)
before freezing interfaces or starting step 3. It is a proposal, not approval.

2026-11-07 optimization follow-up: the [profiled research path](optimized-collection.md) now
completes all ten generated huge-session stages in 1.37–1.51 s under the unchanged
2 s deadline. Ordinary capture, atomicity and accounting checks pass; macOS/Linux
gates pass. Historical failed timings remain valid for the old implementation.
Sustained contention, actual UI latency, full publication and native/contracts
work remain open. No production collector or policy approval is inferred.

2026-11-07 Gate C continuation: [the full research pipeline report](gatec-measurements.md)
records bounded inventory through atomic per-session publication, refresh and
correction scenarios, actual input/renderer output through archive writes,
sustained synthetic writing/checkpoint pressure, blocked waits and near-capacity
initial/replacement resource observations. These are measurements, not new
accepted decisions. Supported dimensions, numeric budgets, cooperative scheduler
semantics, disk reserve and the interpretation of sampled heap/RSS still need
explicit acceptance. Native semantic/identity/recovery contracts remain open.
