> **Historical reference — superseded 2026-11-08.** The current release contract
> is [plan 001](../001-ocm-history-contracts.md); execution starts at
> [plan 002](../002-ocm-history-storage.md), as listed in the [index](../README.md).
> Measurements and fixtures below remain evidence for their original experiments.
> Old open gates, approval requirements, proposed precision/coordination/resource
> contracts and “production out of scope” statements do not govern implementation.
> Do not resume this research checklist. The accepted V1 approximation is retained
> in plan 001; other conflicting policies are replaced there.

# Generated V1/V2 pipeline measurements — 2026-11-06

The next step-2 research increment is implemented and measured. This is an
isolated extractor/candidate-writer experiment, with no production changes.
**Gate C remains open:** a source-wide read at 100k rows exceeds the proposed
reader limit, and scheduling/staging, some contention cases, final UI latency
and accepted budgets still need work. No storage or cleanup milestone is unlocked.

## Reproduce and fixture contracts

From the repository root, using Go 1.25.0:

```sh
GOTOOLCHAIN=go1.25.0 go test -v ./plans/ocm-history-research -run 'TestHistoryGenerated'
GOTOOLCHAIN=go1.25.0 go test -v ./plans/ocm-history-research -run '^TestHistoryScale$' -count=1 -timeout=10m -args -history-scale -history-samples=5
```

Optional `-history-shape` selects exactly `v1-10k`, `v2-10k`, `v1-100k`,
`v2-100k`, `v1-huge` or `v2-huge`. `-history-samples` accepts 1–20, default 5.
Normal tests skip the scale runner and worker; their small correctness/contention
fixtures have 120 assistant rows and four sessions. Tests never discover, copy or
open user databases. All writes are to generated temporary sources/destinations.

Implementation: [generator and oracle](workload_test.go), [fresh-process runner](scale_test.go),
[WAL and browser experiment](contention_test.go). The parent generates each source
once, then starts five fresh test processes; each has a new candidate archive.
Workers require a generated-fixture marker. The internal `-history-worker-source`
flag is for the runner, not a real-data observation interface. Existing
`-history-db` remains a separate explicit-path, read-only observation.

Environment: **Go 1.25.0 / SQLite 3.53.1 / darwin arm64 / Apple M1**, APFS.
The run used `GOCACHE=/private/tmp/ocm-history-20261106-tj193pol/go-build` because
the default cache is sandbox-restricted. [Raw output](scale-results.txt) contains
all 30 results. Five observations per shape are not a p95 guarantee. Reproduction
may produce different timing and size distributions; no timing thresholds are
asserted as correctness tests.

Fixtures are deterministic, with eight provider/model/variant combinations,
31 January 2026 UTC days and five independent token counters (10 / 5 / 2 / 3 / 4
per eligible item). Ordinary assistant records cost 0.125. Padding cycles through
128, 1,024 and 8,192 bytes; every 8,192nd record, including the first, has 3 MiB
of synthetic padding. No private content is used. V1 has a session index; V2
adds session/sequence, session/time/ID, session/type/sequence, time and event
aggregate/type indexes resembling the inspected layout. These are fixture-owned
minimal schemas, not full native migrations or a supported-version matrix.

Every 100th assistant row has a user row with a sentinel cost that must be
excluded. V2 also has a compaction projection and a matching usage event costing
0.25; only the event contributes. Its legacy tables contain frozen sentinel data
that must be ignored. With multiple sessions, every tenth non-root session is
a hidden child, and a distinct pre-fork copy is attached to session 1. V1 counts
that copy under the accepted approximation; V2 excludes it. The one-session shapes
have no fork/child rows.

The independent construction oracle builds expected day/model/token/quality
buckets without reading source JSON or using extraction/aggregation queries.
All samples check the observation and retained day/model costs, row counts,
tokens and quality. Initial import and unchanged full rewrite must both match.
Small default tests additionally prove repeat writes are idempotent and logical
source data/schema are unchanged by extraction/persistence.

| Shape | Sessions | Eligible items | Source MiB | Recorded synthetic cost |
| --- | ---: | ---: | ---: | ---: |
| V1 10k | 100 | 10,001 | 40.27 | 1,250.125 approximate |
| V2 10k | 100 | 10,100 | 41.18 | 1,275 |
| V1 100k | 1,000 | 100,001 | 381.52 | 12,500.125 approximate |
| V2 100k | 1,000 | 101,000 | 390.75 | 12,750 |
| V1 huge | 1 | 100,000 | 381.51 | 12,500 approximate |
| V2 huge | 1 | 101,000 | 390.91 | 12,750 |

For the multi-session V1 shapes, actual synthetic calls cost 1,250 or 12,500;
the extra 0.125 is the deliberately counted fork copy. This is a characterization
of the accepted policy, not evidence that duplicated spending is accurate.
Every V1 contribution retains the approximation warning.

## Complete prototype path

Import measures **extract → writeCandidate (including grouping) → commit** as
one interval. Candidate schema creation, fixture generation and oracle assertions
are outside that interval. A 1 ms Go heap sampler runs during the interval; its
overhead is included. Source transactions end before archive writes begin.
All runs use warm filesystem caches: fixture generation and an accounting-JSON
size query precede the measured read. Fresh process does not imply cold I/O.

| Shape | Extraction min / median / max ms | Import min / median / max ms | Unchanged full read/rewrite median ms | One session median ms |
| --- | ---: | ---: | ---: | ---: |
| V1 10k | 157.756 / 159.427 / 161.873 | 198.737 / 199.048 / 204.279 | 196.862 | 4.642 |
| V2 10k | 177.843 / 179.874 / 192.354 | 215.574 / 217.790 / 230.822 | 200.721 | 5.375 |
| V1 100k | 2587.409 / 2712.177 / 2765.435 | 3292.814 / 3472.494 / 3609.726 | 3398.488 | 6.090 |
| V2 100k | 2677.400 / 2691.238 / 2810.676 | 3291.320 / 3330.060 / 3415.601 | 3356.919 | 6.915 |
| V1 huge | 1667.279 / 1699.162 / 1936.078 | 2403.155 / 2498.917 / 2791.813 | 2514.522 | 1559.419 |
| V2 huge | 1805.281 / 1866.306 / 1939.122 | 2408.066 / 2527.361 / 2768.116 | 2359.318 | 1698.624 |

One-session measurements select `s000000`: 100 ordinary rows at multi-session
scale, plus V2 compaction events as applicable. This session also includes a 3 MiB
record (and sometimes more than one). For the huge shapes it is the entire source.
These are extraction timings, not a partial-scope archive publication mechanism.
The candidate writer still refuses session-scoped replacements.

Unchanged refresh currently does a full extraction and archive rewrite; there is
no cheap freshness signal or production incremental collector. New/changed-session
scheduling and justified correction publication are not established by these
timings. Saved `sum(steps)` queries over the daily table took **0.028–0.081 ms**;
this is a small aggregate query, not startup of a production History view.

## Memory and prototype storage

| Shape | Cumulative allocation median MiB | Sampled additional HeapAlloc max MiB | Observation retained after GC median MiB | Whole-worker peak RSS max MiB | Candidate archive range MiB |
| --- | ---: | ---: | ---: | ---: | ---: |
| V1 10k | 50.33 | 8.54 | 3.34 | 53.38 | 1.88–1.90 |
| V2 10k | 48.04 | 8.49 | 3.52 | 53.48 | 1.08–1.09 |
| V1 100k | 498.07 | 65.26 | 32.41 | 114.67 | 18.41–18.53 |
| V2 100k | 475.17 | 69.38 | 34.24 | 119.42 | 10.53–10.59 |
| V1 huge | 497.92 | 62.26 | 32.35 | 111.55 | 18.50–18.56 |
| V2 huge | 475.02 | 67.06 | 34.17 | 116.19 | 10.57–10.64 |

HeapAlloc is sampled allocated-but-not-yet-freed Go memory, including garbage;
1 ms sampling can miss a peak. Retained memory is the post-GC increase while the
observation remains reachable, relative to the pre-import post-GC baseline.
Neither is an exact instantaneous peak-live-heap measurement. Cumulative
allocation is not memory simultaneously resident. The SQLite read handle and
size query exist before the baseline; it is not a no-collector browser baseline.

RSS comes from the fresh worker's process resource usage (`Maxrss`, bytes on
macOS, KiB converted to bytes on Linux). It includes the runtime, SQLite, import,
refresh, cancellation, WAL and headless browser phases; it excludes the parent
generator. Only macOS measurements were run. The candidate uses DELETE journal
mode; archive WAL bytes were zero. Archive sizes exclude transient rollback
journals and are not a final archive schema/capacity promise. Random map insertion
order produces small file-size differences between runs.

## Cancellation and WAL reader/writer experiment

Cancellation after the first row returned in **0.030–2.000 ms**. A second experiment
requests cancellation 1 ms after the extraction snapshot opens: all 30 runs
returned no publishable observation before any item reached Go, with request-to-
return **1.333–3.855 ms**. This is a timed observation, not a deterministic trace
of SQLite's internal instruction at cancellation. It does not test a blocked
file lock, arbitrary native extension or independently surviving deletion process.

For each shape, a separate synthetic WAL writer changes the first assistant
record's cost while extraction is paused after opening its snapshot. The writer
commits in **9.255–14.263 ms** (no-reader baseline **9.991–15.199 ms**). These small
samples do not establish that a reader improves writer performance. There is no
simultaneous SQL/CPU read contention during the deliberate pause.

Every sample pins **3,176,552 WAL bytes / 771 frames**, with **0 frames** copied
by the PASSIVE checkpoint. Its busy result is zero, demonstrating why that alone
cannot certify checkpoint completion. The reader still returns the old accounting
oracle; the next snapshot sees the correction although the session timestamp is
unchanged. The writer restores the fixture amount, and TRUNCATE checkpointing
returns the WAL to **zero bytes** after readers release their snapshots. This
tests a single controlled writer and checkpoint recovery, not sustained native
OpenCode writes or authority to accept source corrections into saved history.

## Headless browser under collection

Each worker takes ten baseline and ten concurrent samples. A sample lists sessions
on the browser's separate connection, constructs the actual OCM model at 120×40,
and calls View. It then performs four alternating j/k Update→View operations.
No terminal application or Bubble Tea program/renderer/input loop is launched.

| Shape | List→model→View max ms, baseline / concurrent | Update→View max ms, baseline / concurrent |
| --- | ---: | ---: |
| V1 10k | 3.359 / 4.280 | 1.713 / 2.974 |
| V2 10k | 3.438 / 3.924 | 2.215 / 2.766 |
| V1 100k | 6.917 / 6.228 | 2.307 / 3.651 |
| V2 100k | 5.041 / 10.616 | 1.727 / 2.065 |
| V1 huge | 0.903 / 1.866 | 0.795 / 1.101 |
| V2 huge | 0.779 / 1.214 | 0.416 / 0.780 |

The background goroutine repeatedly extracts and writes its own candidate archive.
Collection phase counters show that all **300 concurrent sample starts** occurred
during extraction; none started during archive writing. Thus this run establishes
read-phase contention only, not browser behavior throughout archive publication.
Actual keyboard-to-terminal-frame latency, CLI startup, saved-history presentation
and write-phase contention remain unmeasured.

## Assessment against the proposed budgets

These are [previous proposals](results.md), not newly accepted contracts.

| Proposed target | Evidence / assessment |
| --- | --- |
| Full 100k import ≤5 s | PASS for measured prototype shapes; maximum 3.610 s. No production coordinator/staging overhead measured. |
| Full 100k sweep ≤5 s | PASS for measured unchanged full read/rewrite; maximum 3.606 s. No native incremental freshness guarantee. |
| Source reader ≤250 ms ordinary / ≤2 s exceptional | FAIL for all-source multi-session 100k observations (up to 2.811 s). 10k reads fit 250 ms; one-session huge samples fit 2 s with little margin (up to 1.939 s). |
| Changed 100-row session ≤25 ms | Extraction component fits (maximum 11.888 ms); publication, scheduling and correction authority remain OPEN. |
| Cancellation ≤50 ms | PASS in measured after-row/pre-first-row cases; blocked-lock/native-operation cases remain OPEN. |
| Additional live heap ≤128 MiB | Retained observation and sampled HeapAlloc fit; maximum sampled increase 69.38 MiB. Production peak-live-heap contract remains OPEN. Whole-worker RSS max was 119.42 MiB. |
| Saved query ≤50 ms | Aggregate-query component fits; actual saved-history startup remains OPEN. |
| Input-to-frame ≤50 ms | Headless Update→View component fits; actual input/event-loop/renderer path remains OPEN. |
| Yield after 25 ms or 10 sessions | OPEN: no bounded scheduler exists, and huge-session reads require an explicit exception/staging strategy. |

The next design experiment should test bounded per-session observations, with a
safe staging/completeness strategy for huge sessions. A source-wide scan cannot
be assumed to meet the proposed reader bound. Do not turn paging cursors into
claims of a complete snapshot, relax the budget implicitly, or wire production
collection before the revised snapshot/publication contract has evidence.

Still open: write-phase UI contention, true input/renderer latency, changed/new
session collection and correction reconciliation through publication, sustained
writer/checkpoint impact and blocked-lock cancellation, final archive/scheduler
contracts, supported native semantics, and accepted numeric budgets. No native
deletion/recovery mechanism or cross-process coordinator was exercised here.

## Bounded collection continuation — 2026-11-07

[Bounded collection evidence](bounded-results.md) now measures separate session
snapshots and atomic disk staging across 18 fresh generated workers. Multi-session
100k reads fit the proposed per-reader bound; huge-session staging exhausts the
2 s deadline and safely defers. Completion/liveness and other gate C evidence
remain open. Review the consolidated [implementation contract proposal](proposal.md)
before freezing interfaces or starting step 3. It is a proposal, not approval.
