> **Historical reference — superseded 2026-11-08.** The current release contract
> is [plan 001](../001-ocm-history-contracts.md); execution starts at
> [plan 002](../002-ocm-history-storage.md), as listed in the [index](../README.md).
> Measurements and fixtures below remain evidence for their original experiments.
> Old open gates, approval requirements, proposed precision/coordination/resource
> contracts and “production out of scope” statements do not govern implementation.
> Do not resume this research checklist. The accepted V1 approximation is retained
> in plan 001; other conflicting policies are replaced there.

# Huge-session profiling and optimization — 2026-11-07

Research continuation from `831e474`. Only the isolated research extractor,
staging and measurement helpers changed. The proposed 250 ms ordinary and 2 s
exceptional deadlines, source read-only mode, accounting policy and atomic seal
rules remain unchanged. No production collector or source schema/index change.

## Profile and changes

[Raw profile observations/query plans](stage-profile-results.txt) record the
baseline and sequential experiments. Profiles use generated 100k-row V1/V2 huge
sessions, including 3 MiB JSON records. Each profile runs in a fresh child process;
source generation, schema creation and the independent oracle are outside CPU
profiling. Stage extraction, serialization, inserts and seal/commit are inside.
The profile experiment uses a 15 s diagnostic deadline so it can observe complete
work. These timings are exploratory, not the bounded acceptance measurements.

| Profile variant | V1 stage ms | V2 stage ms | Interpretation |
| --- | ---: | ---: | --- |
| Baseline | 1996.266 | 2117.875 | First source row reached Go after 953.964 / 980.922 ms |
| Unordered, explicit session predicates | 2118.547 | 2215.033 | First row after 2.643 / 2.709 ms; startup improved but total did not |
| Add bounded insert batches | 1703.593 | 1788.130 | Less per-statement cancellation-watcher/scheduler overhead |
| Add typed raw-JSON envelope | 1658.156 | 1831.530 | Lower envelope allocation; individual timing is noisy |
| Add single multi-field JSON projection | 1368.549 | 1552.567 | Fewer repeated loads of large source JSON |

These are single exploratory CPU-profile observations, not distributions or
isolated causal estimates. A repeat baseline V2 query-plan observation was
2125.117 ms with first row at 979.219 ms. CPU sample time includes multiple
runtime threads and must not be equated with elapsed reader lifetime. Samples
pointed to SQLite reads and goroutine scheduling; the timing experiments and
query plans provide the complementary evidence.

The retained changes are:

- Session predicates go directly into each SQL branch. Usage scans no longer
  sort projected accounting JSON. Baseline V2 EXPLAIN showed temporary B-trees
  for ORDER BY; optimized plans use existing session/type and event indexes
  without a temporary sort. No consumer requires ordered usage: observations
  key by identity and stages enforce uniqueness with a primary key.
- One guarded, multi-path `json_extract` returns an eight-slot JSON envelope.
  This replaces separate field expressions that repeatedly reference large
  source JSON. Go decodes into fixed raw-JSON slots, then validates values as
  before. Raw JSON avoids SQL scalar coercion of cost or token values.
- Stage inserts batch up to 128 rows or 64 KiB serialized payload. A single
  larger record that fits the existing per-field limits is flushed by itself;
  the 64 KiB target is not a new record-rejection rule. Total accounting payload
  still caps at 64 MiB and metadata fields at 4 KiB each. Batches remain inside
  one destination transaction; all rows and the seal still commit together.
- Every batch uses ExecContext. Cancellation checks and query interruption stay
  active. The pinned module's `stmt.go`/`interruptOnDone` implementation starts
  a watcher for cancellable executions; batching reduces execution count without
  substituting an uncancellable write or relaxing source deadlines.

[Extractor](extract.go), [staging](stage.go), [profiling harness](profile_test.go),
[safety/scale harness](stage_test.go).

## Reproduce

From the repository root, Go 1.25.0; set GOCACHE to a writable directory if needed:

```sh
go test ./plans/ocm-history-research
go test -v ./plans/ocm-history-research -run '^TestHistoryBoundedScale$' -count=1 -timeout=10m -args -history-bounded -history-samples=5
```

Profiling is optional and synthetic-only. Use an explicit private output directory
outside git; the parent generates fixtures and only the child profiles staging:

```sh
GOTOOLCHAIN=go1.25.0 go test -v ./plans/ocm-history-research -run '^TestHistoryStageProfile$' -count=1 -timeout=2m -args -history-profile-dir=/private/tmp/ocm-stage-profile
GOTOOLCHAIN=go1.25.0 go tool pprof -top -cum /private/tmp/ocm-stage-profile/v2-huge.cpu
```

Optional `-history-shape=v1-huge` or `v2-huge` restricts profiling. Default tests
skip profile generation and scale measurement. All databases are temporary
synthetic fixtures; neither harness discovers user databases or runs OpenCode.
Binary profiles remain outside the repository. The stored raw artifact contains
only text observations and pprof summaries.

## Correctness and verification

Normal/mock/race tests retain the independent accounting, V2 fork/compaction,
unknown-field, snapshot consistency and source-unchanged oracles. New cases cover
session/all-source membership with reversed unordered scans; exact int64 maximum
versus overflowing/fractional/exponent/string token counters; known-zero,
fractional and invalid cost types; duplicate native identities; cancellation and
injected failure after an earlier batch reached SQLite; and a large escaped
metadata singleton above the normal batch-byte target. Collector-crash recovery
now exits after a full batch has been inserted, not before the buffer flushes.
Partial stages and failures still retain prior accepted accounting.

Final macOS gates and Docker Linux/arm64 Go 1.25.0 gates pass: vet, repository
race tests, normal builds, repository mock tests and mock builds. Local focused
OCM tests pass. [Linux output](optimized-linux-verification.txt),
[verification record](verification.md). No hosted CI run is claimed.

## Final bounded measurements

[Raw final output](optimized-bounded-results.txt): six unchanged fixture shapes ×
five fresh processes, 30 samples, all pass; total run 46.030 s. Environment:
Go 1.25.0 / SQLite 3.53.1 module driver / darwin arm64 / Apple M1 / APFS.
Generation warms filesystem caches; this is not cold-I/O evidence. No profiling,
compilation, Docker work or other verification ran concurrently with the final
measurement. Five samples per shape do not establish p95 or a cross-machine SLA.

Each cell is min / median / max milliseconds over five workers:

| Shape | Capture sweep including staging where required | Longest source attempt per worker |
| --- | --- | --- |
| V1 10k / 100 sessions | 144.026 / 144.431 / 145.213 | 4.629 / 4.648 / 5.049 |
| V2 10k / 100 sessions | 155.838 / 156.883 / 157.977 | 5.221 / 5.376 / 5.939 |
| V1 100k / 1,000 sessions | 1333.442 / 1344.157 / 1539.146 | 4.836 / 4.857 / 8.551 |
| V2 100k / 1,000 sessions | 1416.254 / 1435.971 / 1700.498 | 5.549 / 5.577 / 10.045 |
| V1 100k / huge session | 1383.800 / 1407.065 / 1432.157 | 1367.782 / 1390.874 / 1409.598 |
| V2 100k / huge session | 1476.196 / 1518.072 / 1532.884 | 1458.786 / 1500.769 / 1509.219 |

For huge shapes, the longest source attempt is the full stage path, including
seal/commit and the final buffered insert after source release; it is a
conservative interval for reader occupancy. Each is a separate new snapshot
following the capped ordinary attempt. **All ten huge stages complete under the
unchanged 2 s deadline**, with the full accounting oracle passing. No timed-out
stage or diagnostic retry occurred. Previously, all six bounded huge attempts
failed; diagnostic completion was 2.01–2.34 s. Historical measurements are retained
in [bounded results](bounded-results.md), not overwritten or relabeled as paired
controlled comparisons. First-row latency is from the separate profile runs.

All **11,000** ordinary per-session observations completed; largest ordinary
attempt was 10.045 ms. The loop still uses fixture-owned inventory and only yields
every ten sessions. This is not a measurement of real inventory enumeration,
25 ms scheduler fairness, source identity, reconciliation or archive publication.

| Shape | Sampled additional HeapAlloc max MiB | Whole-worker peak RSS max MiB | Cumulative allocation median MiB | Completed stage MiB |
| --- | ---: | ---: | ---: | ---: |
| V1 10k | 2.490 | 28.922 | 39.220 | — |
| V2 10k | 2.452 | 28.875 | 37.825 | — |
| V1 100k | 2.584 | 30.891 | 363.360 | — |
| V2 100k | 3.053 | 31.078 | 349.346 | — |
| V1 huge | 2.798 | 75.641 | 380.133 | 22.988 |
| V2 huge | 2.303 | 76.438 | 362.685 | 23.215 |

HeapAlloc is sampled every millisecond over capture/staging, relative to the
post-GC pre-capture baseline. Sampling overhead is included in elapsed times;
it can miss peaks and includes allocated garbage. It is **not an exact peak-live
heap bound**. Worker RSS comes from process rusage and includes runtime/SQLite
and subsequent full stage-oracle loading, but not fixture generation in the
parent. RSS is not an additional-browser-memory measurement. The old bounded
runner only measured end-of-interval heap; do not compare it to these sampled
peaks. Stage sizes exclude transient rollback journals and SQLite temp files.
V2 page occupancy differs slightly from the old sorted insertion order.

## Step 2 consequence

The original measured huge-session completion failure is addressed for these
100k/381–391 MiB generated shapes, with meaningful margin below 2 s on this
machine. Larger/adversarial sessions remain bounded and may defer; the scope,
resource limits and visible deferral/deletion policy still need explicit contract
acceptance. No guarantee of eventual completion for arbitrary session sizes.

Next measure sustained writer/checkpoint pressure and real UI responsiveness
through archive writes, plus the complete inventory/capture/reconciliation/
publication pipeline. Blocked-query cancellation, disk-space reserve/journal
capacity and accepted numeric budgets remain open. Native accounting/deletion,
identity and recovery contracts are unchanged. Step 2 remains IN PROGRESS and
steps 3–5 remain gated. No source write, native command, private snapshot read,
new dependency or production behavior change was introduced.
