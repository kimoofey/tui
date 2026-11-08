> **Historical reference — superseded 2026-11-08.** The current release contract
> is [plan 001](../001-ocm-history-contracts.md); execution starts at
> [plan 002](../002-ocm-history-storage.md), as listed in the [index](../README.md).
> Measurements and fixtures below remain evidence for their original experiments.
> Old open gates, approval requirements, proposed precision/coordination/resource
> contracts and “production out of scope” statements do not govern implementation.
> Do not resume this research checklist. The accepted V1 approximation is retained
> in plan 001; other conflicting policies are replaced there.

# Extraction and synthetic measurements — 2026-11-05

These are isolated step-2 experiments, using Go 1.25.0 / SQLite 3.53.1 on
Apple M1, darwin/arm64, at foundation `0c710a6`. They do not implement history
in OCM or unlock step 3. The [accepted V1 rule](decisions.md) is applied.

2026-11-06 update: [private real-data extraction](real-data.md) now passes on the
available 37.24 MiB snapshot, with repeated timings and process RSS. The approved
route uses this real sample plus generated workloads for scale, replacing the
earlier roughly 2 GB real-database assumption. The results below retain their
original synthetic scope; full gate C remains open.

The later [V1/V2 scale continuation](scale-results.md) measures the complete
prototype path with variable-size records, heap/RSS, cancellation, controlled WAL
and headless UI across 30 fresh processes. Its 100k imports fit the proposed
five-second target; multi-session source-wide reads exceed the proposed reader
limit. The original tables below remain historical component baselines.

## Accounting experiment

`extract.go` validates required columns inside a dedicated read transaction,
selects the authoritative schema, enumerates all sessions independently of root
filtering, and extracts usage in the same snapshot. An optional session scope
supports known-empty versus missing observations; it cannot replace a different
scope. Failed/cancelled/row-limited reads return no publishable result. These
are experiment-local types, not proposed public capability signatures.

Only selected accounting JSON crosses into Go; conversation content stays in the
fixture/source. SQLite still parses the containing JSON. Native session/message
or event IDs, time, provider/model/variant, recorded cost, five independent token
counters, schema/policy version and quality are retained. No prices are inferred.
The float64 cost representation is experimental, not an approved precision rule.
Unknown tokens/cost stay unknown. JSON booleans and numeric strings are not costs;
negative/overflowing cost values are unknown. Known zero remains distinct.
Malformed JSON, unknown roles/event sources, invalid tokens, missing timestamps,
and unknown model/cost/counters have explicit counters. A missing timestamp keeps
known cost in an unbucketed category for all-time totals; date ranges cannot
silently allocate it to a day. Scan completion never means billing completeness.

Independent fixtures prove:

- V1: recorded total **7**, provider/model **p/m=6**, **q/n=1**; actual-call
  reference stays **4**. Every V1 contribution is approximate under policy
  `v1-recorded-assistant-approximate/1`, including non-fork sessions.
- V2: **6.5 / 3 / 0** for the existing half-open period oracle. Assistant usage,
  child sessions and compaction events count; pre-fork copies, non-assistant
  projections, ended/title events, orphan sessions and frozen V1 rows do not.
- Two-day/provider/model/variant totals and all five token counters reconcile.
  Unknown-model compaction amounts remain in totals. Empty V2 stays authoritative.
- UTC and Europe/Berlin fixtures cover DST gaps/repeated hours and month/year
  boundaries. These are explicit experimental parameters, not a timezone decision.
- Logical schema/data hashes are identical before/after extraction on the
  accounting fixtures. WAL bytes are not used as a corruption test.
- A source writer and migration committed after snapshot opening do not mix into
  the observation. The next read sees V2. Publication still needs a coordinator
  and lineage validation; this does not claim those races are solved.

`reconcile.go` proves a conservative publication rule: missing IDs, migration,
scope mismatch, changed fields without explicit correction evidence, and ambiguous
replacement preserve the entire old scope and report unresolved state. It does
not merge overlapping old/new records or take the maximum total. Explicit fixture
correction authority permits a decrease and zero while preserving approximation.
**No production authority provider is established** by that injected boolean map.
Native semantics, correction evidence and lineage remain contract gates.

`candidate.go` is a separate synthetic SQLite capacity/rollback prototype with
per-usage metadata, daily/model aggregates and observation quality/diagnostics.
It only replaces whole-fixture observations; it refuses partial/session scopes.
It is not a migration, supported archive schema, or per-session collector. Tests
prove idempotent writes, quality after source deletion/reopen, content minimization
and transaction rollback on injected failure. Source handles are read-only;
synthetic destination writes use separate databases.

## Reproduce measurements

```sh
GOTOOLCHAIN=go1.25.0 go test ./plans/ocm-history-research -run '^$' -bench History -benchmem -benchtime=1x -count=5
GOTOOLCHAIN=go1.25.0 go test -v ./plans/ocm-history-research -run 'TestHistoryQueryPlan$'
```

Raw results: [synthetic-bench.txt](synthetic-bench.txt). Each sample is one
operation; five observations are a small distribution, not a p95 guarantee.
These are warm, same-process/file-cache runs. Fixture generation and one oracle
extraction precede measured work. No OS cache purge or cold-I/O claim. The default
`-bench History -benchmem -count=5` also works, using adaptive repetition.

Dataset dimensions are fixed in `benchmark_test.go`: 10,000 rows/100 sessions,
100,000 rows/1,000 sessions, and 100,000 rows/one huge session; 31 UTC days,
eight provider/model pairs, five token counters, cost 0.125 per row, 1,024 bytes
of synthetic content padding per message plus accounting JSON. All rows are V1
assistant messages; benchmark results do not establish V2 performance. Only
fixture-owned indexes are created. No real source index is added or modified.

| Work | Shape | Min / median / max wall ms |
| --- | --- | ---: |
| Extract (includes SQL read + JSON projection + Go decode) | 10k / 100 sessions | 149.113 / 150.037 / 151.041 |
| Extract | 100k / 1,000 sessions | 1502.006 / 1508.501 / 1533.605 |
| Extract huge session | 100k / one session | 1590.881 / 1602.665 / 1612.804 |
| Group | 10k | 1.581 / 1.616 / 2.237 |
| Group | 100k / 1,000 sessions | 21.469 / 22.225 / 23.001 |
| Candidate write (includes grouping) | 10k | 38.773 / 41.376 / 42.070 |
| Candidate write (includes grouping) | 100k / 1,000 sessions | 574.542 / 660.207 / 663.216 |
| Candidate write (includes grouping) | 100k / one session | 597.049 / 679.906 / 690.671 |
| Saved daily range query | 100k / 1,000 sessions | 0.104 / 0.124 / 0.198 |
| Extract one 100-row session from 100k | Session scope | 1.525 / 1.566 / 2.103 |
| Browser List, 1,000 sessions | No collection | 1.470 / 1.545 / 1.575 |
| Browser List, 1,000 sessions | Dedicated concurrent extraction connection | 1.497 / 1.558 / 1.691 |
| Cancel request → return after first row | 100k dataset | 0.0152 / 0.0159 / 0.0180 |

The cancellation benchmark's standard ns/op also includes work before requesting
cancellation (~1.2 ms); `cancel-ns/op` measures the tail separately. It does not
measure cancellation during a blocked writer/checkpoint or long SQLite function.
Browser List measures loading only, not rendering, startup or keyboard-to-frame
latency. Concurrent B/op includes background allocations and must not be read as
browser-only memory. Full source-reader lifetime is approximately extraction wall
time (the transaction brackets detection/read/iteration); no archive transaction
is held at the same time. No cross-process coordination or lock waits measured.

Allocation volume per extraction: ~41.6 MB/850k allocations at 10k rows;
~414 MB/8.5 million allocations at 100k. This is **cumulative allocated bytes**,
not live heap or peak RSS. At median wall time, throughput is about 66k rows/sec.
The separate accounting-JSON decode microbenchmark is 4–10 microseconds for one
small record; it excludes SQLite JSON parsing/projection and content traversal.

A supplementary size run:

```sh
GOTOOLCHAIN=go1.25.0 go test -v ./plans/ocm-history-research -run '^$' -bench '^BenchmarkHistory$/10k-100sessions/WriteCandidate$' -benchmem -benchtime=1x -count=1
```

reported **1,904,640 bytes** for the 10k candidate SQLite database (page count ×
page size). This is a single size observation, excluding transient journal/WAL,
backups and future indexes. It is not the final daily archive capacity promise.
The query-plan fixture reports an index scan on message_session_id_idx, a
covering session primary-key lookup, and a temporary B-tree for the last ORDER BY
term. This fixture plan is not a plan measured on a real supported installation.

## Proposed budgets to validate, not accepted contracts

| Area | Proposed acceptance target | Current evidence / gap |
| --- | --- | --- |
| Ordinary background batch | Yield after 25 ms or 10 sessions | 100-row session ~1.5–2.1 ms; scheduler not implemented. |
| Source-reader lifetime / huge session | 250 ms ordinary; 2 s exceptional | Full 100k scan ~1.6 s; huge snapshots need memory/staging investigation and real-data validation. |
| Cancellation | Request-to-return ≤50 ms | ~0.016 ms after first row only; blocked-query cases unmeasured. |
| Initial saved-history query | ≤50 ms | Tiny 31-day query ≤0.2 ms; startup/rendering/large archive unmeasured. |
| Full 100k synthetic import | ≤5 s extraction + write | Sum of measured medians ~2.2–2.3 s; not an end-to-end timed import. |
| Refresh | Changed 100-row session ≤25 ms; full sweep ≤5 s/100k | Session extraction passes; no reliable cheap unchanged-source signal. Full refresh still scans old rows. |
| Memory | ≤128 MiB additional live heap | Unmeasured; cumulative allocation numbers cannot establish this. |
| Input response | ≤50 ms keyboard-to-frame | Not measured; List contention is only an early component baseline. |

These are review proposals informed by component baselines, not an agreed
responsiveness target or completed gate C. They must be accepted and validated
before freezing scheduling/staging, memory and reader-lifetime contracts.

## Explicit real-database observation

The test harness only reads a path explicitly supplied by its operator:

```sh
GOTOOLCHAIN=go1.25.0 go test -v ./plans/ocm-history-research -run '^TestHistoryExplicitPath$' -args -history-db /absolute/path/opencode.db
```

No default path or home search. It opens mode=ro without immutable mode, runs a
consistent extraction, logs counts/diagnostics/duration only, and writes no archive
or raw records. It does not copy live database files. Without the flag the test
SKIPs. The first real-data run occurred on 2026-11-06 against a private online
backup; see [real-data evidence](real-data.md). Repeated extraction/process RSS on
that sample does not complete performance validation. Scale live memory,
WAL/checkpoint/writer impact, changed/unchanged refresh, full import timing and
end-to-end input/startup latency remain exit gates.
