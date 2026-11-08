> **Historical reference — superseded 2026-11-08.** The current release contract
> is [plan 001](../001-ocm-history-contracts.md); execution starts at
> [plan 002](../002-ocm-history-storage.md), as listed in the [index](../README.md).
> Measurements and fixtures below remain evidence for their original experiments.
> Old open gates, approval requirements, proposed precision/coordination/resource
> contracts and “production out of scope” statements do not govern implementation.
> Do not resume this research checklist. The accepted V1 approximation is retained
> in plan 001; other conflicting policies are replaced there.

# Gate C pipeline and responsiveness research — 2026-11-07

Research against `252b283` plus this increment. **Step 2 remains IN PROGRESS.**
This is measurement evidence, not approval of the proposed budgets, workload
support, accounting authority or production archive contract. No production code,
CLI, dependencies, user databases or native OpenCode processes changed.

## Reproduce and interpret

Use Go 1.25.0 from `go.mod`, with a writable build cache if needed:

```sh
GOTOOLCHAIN=go1.25.0 go test ./plans/ocm-history-research
GOTOOLCHAIN=go1.25.0 go test -v ./plans/ocm-history-research -run '^TestHistoryGateC$' -count=1 -timeout=10m -args -history-gate-c -history-samples=5
GOTOOLCHAIN=go1.25.0 go test -v ./plans/ocm-history-research -run '^TestHistoryGateC$' -count=1 -timeout=5m -args -history-gate-c -history-ui-only -history-samples=5
GOTOOLCHAIN=go1.25.0 go test -v ./plans/ocm-history-research -run '^TestHistoryGateCapacity$' -count=5 -timeout=5m -args -history-gate-c
```

Optional `-history-shape=v1-10k`, `v2-10k`, `v1-100k`, `v2-100k`, `v1-huge`
or `v2-huge` restricts the full-pipeline run. Normal tests skip measurements.
Internal worker flags require an explicit generated database and matching marker;
there is no user-source discovery. Parent-generated fixtures, databases, stages
and executables stay temporary. Raw artifacts contain synthetic measurements only.

[Pipeline](pipeline.go), [safety tests](pipeline_test.go),
[runner/resource sampling](gatec_test.go),
[input/renderer and pressure harness](responsiveness_test.go),
[capacity probe](capacity_test.go).

## What is now exercised

- Inventory uses authoritative V1/V2 selection and bounded keyset pages of 100
  IDs with a 250 ms deadline. Each page is discovery, not a source-wide snapshot.
  Captures revalidate required source schema inside separate read-only snapshots;
  hidden children are inventoried. Existing source indexes remain unchanged.
- Ordinary attempts retain the 1,000-row / 250 ms limits. Exceptional capture
  restarts in a new snapshot with the unchanged 2 s deadline, streaming into one
  private stage. Exhaustion defers; it does not count as successful import.
  Completed stages are closed and removed after publication. There is no
  diagnostic deadline retry in the full-pipeline measurements.
- A research archive compares canonical accounting records in a transactional
  scratch table, retaining old generations for ambiguous changes/loss. Staged
  rows stream through bounded inserts; no huge in-memory observation is rebuilt.
  Accepted contributions, per-session daily/token aggregates and session generation
  change in one transaction with an expected-generation check. Unchanged sessions
  retain their generation. Independent fixture oracles check final cost, daily,
  model, token, unknown-field and V1 approximation results.
- The archive uses WAL, synchronous FULL, foreign keys and auto-checkpoint at
  1,000 pages. Scratch is in the archive itself so its pages and WAL are included
  in disk measurements; it is cleared before commit. This disposable schema and
  float64 accounting are **not** production migrations or the decimal contract.
  Source identity/epochs remain prerequisites for production, outside this probe.
- Each worker measures initial import, unchanged sweep, saved aggregate query,
  and import under sustained writer/checkpoint pressure. A separate matrix covers
  a new 100-row session, unchanged observation, append, rejected/authorized changed
  usage, and rejected/authorized correction to an existing native fixture record
  without changing its session timestamp. Huge old corrections use the sealed
  streaming path. Correction authority is explicit synthetic input, not discovered
  from native semantics or stable IDs.
- The pressure writer updates a non-accounting JSON field every 5 ms, requests a
  PASSIVE checkpoint every ten commits, and records writer distribution, pinned
  checkpoint samples, WAL high-water and final TRUNCATE recovery. Baseline pressure
  runs for 250 ms; concurrent pressure spans a whole import. This is sustained
  **synthetic SQL pressure**, not a native OpenCode workload guarantee.
- The actual OCM browser runs in a Bubble Tea program with input decoder, event
  loop and renderer enabled. Alternating j/k bytes arrive through an input pipe.
  The final harness pins TrueColor and verifies RGB style bytes reached the
  sink. A test wrapper replaces the final visible help line with a changing marker;
  timing ends when renderer output reaches that marker after table updates.
  No direct Update/View timing substitutes for this measurement. Archive-write
  sampling is enabled only immediately before accepted archive rows change,
  after reconciliation/scratch preparation. Start/end phase counts are reported;
  samples are not artificially delayed to lengthen a write. They establish phase
  coverage, not exact attribution of every nanosecond in a sample to that phase.
- The output sink measures renderer bytes, not terminal-emulator painting or
  display refresh. Browser load-to-output starts before source opening/listing;
  it is not process/CLI startup or the future saved-history screen.
- Cancellation cases hold an exclusive source lock (rollback-journal mode), an
  archive writer lock (WAL), the connection pool, or a real advisory flock on a
  private lock file. SQLite busy_timeout is explicitly 10 ms for these cases.
  Pending-at-cancel and returned error are recorded: SQLITE_BUSY is not relabeled
  as a context interruption. Connections recover after lock release. The flock
  probe uses separate descriptors in one process; it is not parent-death or
  descendant-quiescence evidence. Existing row/pre-first-row cancellation tests
  remain in the default suite.

## Safety and resource interpretation

Default tests cover stale publication generations, unsealed/wrong-session stages,
unknown correction authority, authorized zero replacement, lost IDs, cancellation
and injected failure before commit. Deterministic SQLite page quotas force stage
and archive SQLITE_FULL failures without filling the host filesystem. Rejected
attempts preserve prior accepted totals. This tests SQLite capacity failure,
not every OS-level ENOSPC/fsync failure or a production disk-reservation service.

Resources are sampled every millisecond over initial import, unchanged sweep,
writer/browser baselines and concurrent import. HeapAlloc and HeapInuse increases
are relative to a post-GC baseline, include instrumentation/garbage, and can miss
peaks. Whole-worker peak RSS is parent-observed process rusage, including SQLite,
Go runtime, oracles and later correction/cancellation work; it is not additional
browser heap. Neither metric establishes an exact 128 MiB peak-live-heap bound.
Disk maxima include archive, scratch, stage, WAL and journals; combined bytes
are sampled simultaneously. Source WAL is reported separately. Filesystem free
space, allocator overhead and arbitrary future archive sizes are not bounded by
these samples.

The capacity probe creates a separate accounting-only shape near 64 MiB payload,
with 3 KiB provider labels and smaller model/variant labels within the 4 KiB field
limit. It probes both shared labels (one aggregate bucket) and a distinct model
per usage (19,168 aggregate buckets), then repeats against populated accounting.
This exposes the extra heap and archive cost of aggregation cardinality. It uses a **15 s diagnostic timeout** for capacity observation only; this
is not an operational fallback or evidence for the 2 s source deadline on that
shape. One extra record must trip the 64 MiB cap without sealing or changing
accepted totals. Accounting payload alone substantially understates disk demand.

## Decisions still required

The measured results below should inform explicit decisions, not imply approval:

1. Accept or revise supported dimensions, the 4 KiB field / 64 MiB payload caps,
   oversized-session deferral and guarded-deletion restrictions. New deferred
   history is unavailable, never successful import or known zero.
2. Approve numeric performance budgets with named workload, hardware and warm-I/O
   assumptions. Retain explicit unsupported/adversarial cases; five samples per
   shape are not p95 or a cross-machine SLA.
3. Clarify scheduler semantics: the research scheduler checks elapsed 25 ms / ten
   sessions between completed sessions. A source attempt or atomic publication
   can cross 25 ms; huge capture/publication is not split across live snapshots.
   Its measured batch high-water must not be called a strict 25 ms bound.
4. Set a disk-reserve policy covering stage pages/journal, archive scratch,
   accepted rows and WAL together, including an already populated archive and
   low-space failure. A fixed reserve based only on 64 MiB payload is inadequate.
   Any suggested reserve is provisional until its supported dimensions and
   additional archive-growth bound are accepted.
5. Decide whether sampled memory evidence is sufficient for a scoped budget or
   require stronger allocation/resource bounds. Do not rename sampled heap/RSS
   as exact peak-live memory.

Native supported-version accounting/deletion semantics, source identity,
parent-death recovery, exact interfaces/schema and hosted CI remain separate
Step 2 closures. The prototype does not implement those production mechanisms.

## Full-pipeline measurements (initial no-color renderer)

[Raw full output](gatec-results.txt): six existing generated shapes × five fresh
workers, 30 workers, exit 0, 256.264 s including parent fixture generation.
Go 1.25.0 / SQLite 3.53.1 / darwin arm64 / Apple M1 / APFS; filesystem caches
are warm from generation. Initial local and Linux gates finished before this run;
no other build/test/profiling ran concurrently. Exploratory runs are not pooled
with this final distribution. Five workers do not establish p95.

The full-pipeline artifact predates the explicit color profile. Its output sink
was detected as NoTTY, so its real input/program/renderer timings describe a
no-color loop. They are retained as such; use the separate TrueColor validation
below for colored UI claims. The change only pins the renderer profile and adds
an observed-color assertion. Inventory, capture, archive SQL and accounting logic
are unchanged. `-history-ui-only` skips the baseline import, unchanged sweep,
saved query and correction/cancellation matrix, then measures a fresh import
under sustained pressure with the colored browser. Zero values in skipped fields
of that artifact are not measurements. Its resources span baseline/pressure only.

Each cell is min / median / max milliseconds over five workers. Capture includes
source reads/parsing and stage work where needed; publication includes scratch
preparation, grouping, reconciliation and atomic accepted-row commit. Those
subcomponents are included end-to-end, not separately timed causal estimates.

| Shape | Initial inventory → commit | Unchanged sweep | Import under pressure |
| --- | ---: | ---: | ---: |
| v1-10k | 289.694 / 301.012 / 305.738 | 260.290 / 262.906 / 265.950 | 291.150 / 293.144 / 346.544 |
| v2-10k | 295.271 / 298.999 / 305.837 | 266.995 / 267.956 / 268.466 | 295.365 / 300.538 / 302.602 |
| v1-100k | 2736.554 / 2824.770 / 3172.994 | 2665.279 / 2681.202 / 2728.556 | 2837.004 / 2881.117 / 2928.336 |
| v2-100k | 2845.569 / 2902.162 / 3378.798 | 2821.564 / 2829.190 / 2857.593 | 2907.791 / 2914.033 / 2963.107 |
| v1-huge | 2254.986 / 2259.432 / 2323.799 | 2504.162 / 2506.694 / 2933.162 | 2274.663 / 2294.594 / 2334.207 |
| v2-huge | 2384.358 / 2451.829 / 2520.123 | 2616.993 / 2663.175 / 2700.670 | 2361.780 / 2424.014 / 2490.195 |

All 33,000 ordinary session captures and 30 main-sweep huge stages completed;
zero deferred sessions, no diagnostic retry. Huge old-correction scenarios add
20 further bounded staged captures. Every accepted sweep matches the independent
construction oracle. A deferred new session would not be counted as imported.

| Shape | Slowest source attempt across all three sweeps, ms | Concurrent input→output maximum, ms | Write-phase sample starts / all concurrent samples |
| --- | ---: | ---: | ---: |
| v1-10k | 11.962 | 18.185 | 26 / 98 |
| v2-10k | 6.608 | 17.786 | 20 / 99 |
| v1-100k | 8.183 | 18.259 | 208 / 872 |
| v2-100k | 12.109 | 18.962 | 184 / 885 |
| v1-huge | 1885.899 | 19.033 | 72 / 699 |
| v2-huge | 1624.100 | 18.610 | 72 / 732 |

There are 3,385 concurrent input samples, including
582 starts during accepted archive writes. Navigation maximum is
19.033 ms concurrent and 35.729 ms baseline. Browser
load/list/model/program-to-first-marker maximum is 53.114 ms baseline /
27.479 ms concurrent. The baseline exceeds 50 ms in some samples;
this is source-browser initialization, not saved-history startup. The saved
aggregate-query component maximum is 3.737 ms; future saved-history
startup is still an integration measurement. No terminal-painting claim.

| Refresh scenario | Capture through commit, min / median / max ms |
| --- | ---: |
| new-100-row (30 workers) | 1.902 / 2.210 / 3.869 |
| unchanged-100-row (30 workers) | 1.652 / 1.814 / 3.379 |
| appended-usage (30 workers) | 2.023 / 2.231 / 2.723 |
| changed-without-authority (30 workers) | 1.676 / 1.864 / 3.293 |
| changed-with-authority (30 workers) | 1.909 / 2.338 / 2.831 |
| old-native-correction-without-authority, ordinary | 2.195 / 2.506 / 5.506 |
| old-native-correction-with-authority, ordinary | 2.591 / 2.901 / 4.703 |
| old-native-correction-without-authority, huge | 2295.900 / 2416.112 / 2534.784 |
| old-native-correction-with-authority, huge | 2618.574 / 2738.448 / 2790.462 |

Rejected corrections preserve accepted totals; explicit synthetic authority permits
zero/downward replacement. Neither success nor timing discovers native authority.
The 100-row capture-through-publication cases fit the proposed 25 ms target.

| Blocked case | Cancel request→return, min / median / max ms |
| --- | ---: |
| source-exclusive-lock | 8.807 / 9.744 / 10.357 |
| archive-write-lock | 8.685 / 10.289 / 10.852 |
| connection-pool-wait | 0.002 / 0.002 / 0.006 |
| coordinator-wait | 0.001 / 0.002 / 0.005 |

All waits remained pending at cancellation and all connections recovered. The
source-exclusive case returns SQLITE_BUSY after the explicit 10 ms busy timeout;
archive writer/pool/flock cases return cancellation. These outcomes fit the
proposed 50 ms return target with this timeout; they do not prove arbitrary busy
timeouts can be interrupted promptly.

| Shape | Sampled additional HeapAlloc / HeapInuse max MiB | Whole-worker peak RSS max MiB | Combined archive/stage disk max MiB |
| --- | ---: | ---: | ---: |
| v1-10k | 5.288 / 5.648 | 38.250 | 7.301 |
| v2-10k | 5.584 / 5.922 | 39.375 | 6.930 |
| v1-100k | 6.826 / 7.406 | 41.922 | 31.930 |
| v2-100k | 6.819 / 7.414 | 42.719 | 30.078 |
| v1-huge | 5.502 / 5.930 | 54.609 | 115.494 |
| v2-huge | 5.387 / 5.844 | 55.000 | 116.575 |

Writer baseline/concurrent maxima across workers are
5.091 / 18.828 ms; compare worker medians in the raw artifact
before interpreting tail ratios. Pressure generated 11,299 commits and
1,118 checkpoint requests, with 643 samples showing pinned
checkpoint progress. Source WAL peaked at 1.289 MiB; final TRUNCATE returned
all source WALs to zero. Normal pressure sessions read no more than
11.962 ms; huge pressure stages no more than
1522.309 ms. This confirms measured synthetic pressure for these shapes.

The largest ordinary scheduler batch was 34.324 ms; huge batch duration
includes the entire atomic capture/publication and reaches
2933.162 ms. The scheduler checks at session boundaries and a strict
25 ms batch ceiling is **not met**. UI response fitting 50 ms does not silently
waive that separate target.

## TrueColor input/renderer validation

[Raw colored-UI output](gatec-colored-ui-results.txt): six shapes × five fresh
workers, exit 0, 86.607 s. This separate run follows the final macOS/Linux gates,
with no concurrent build/test/profiling. `-history-ui-only` runs the same bounded
full import into a fresh archive under sustained pressure, plus baseline and
concurrent input loops. It omits the earlier baseline import/unchanged/correction
work; skipped fields are zero/null, never claimed as completed measurements.
No no-color samples are pooled into these distributions.

TrueColor is explicitly pinned and RGB style output was observed in **all 60**
baseline/concurrent loops. The renderer processes actual colored table updates
as well as the final-line timing marker. Each import matches the independent
oracle; all source WALs recover to zero.

| Shape | Colored pressure import, min / median / max ms | Baseline / concurrent navigation maximum, ms | Archive-write starts / concurrent samples |
| --- | ---: | ---: | ---: |
| v1-10k | 299.716 / 300.582 / 308.614 | 36.657 / 19.457 | 28 / 100 |
| v2-10k | 312.967 / 313.822 / 318.227 | 36.144 / 19.463 | 30 / 101 |
| v1-100k | 2630.939 / 2636.385 / 2970.334 | 35.095 / 19.528 | 199 / 821 |
| v2-100k | 2699.079 / 2731.500 / 3025.097 | 34.959 / 18.698 | 169 / 841 |
| v1-huge | 2335.922 / 2348.135 / 2419.956 | 20.581 / 20.853 | 69 / 719 |
| v2-huge | 2421.053 / 2462.191 / 2466.571 | 21.041 / 20.813 | 72 / 744 |

Total concurrent input samples: 3,326; 567 begin during accepted archive
writes. Maximum input→renderer-marker output is **20.853 ms** concurrent
and 36.657 ms baseline, below the proposed 50 ms output target. This is
not terminal painting. Source-browser load→first marker maximum is
53.024 ms baseline / 29.964 ms concurrent, distinct from saved-history
query/startup. Colored pressure imports all complete without deferral; slowest
ordinary source attempt is 11.430 ms and slowest huge attempt
is 1572.571 ms. Those are the actual colored-UI pressure observations, not
substituted component timings from the no-color run.

## Final near-capacity resource measurements

[Raw capacity output](gatec-capacity-results.txt): shared labels and distinct
models × five trials each, exit 0, 91.596 s. These trials share one test process;
fixtures are regenerated and a post-GC sampling baseline is taken for each trial.
They are not ten fresh-process RSS distributions. Both shapes contain 19,168
accounting rows, **67,107,168 payload bytes** (1,696 bytes below 64 MiB), with
3 KiB provider and 128-byte model/variant values. Shared labels produce one
aggregate bucket; distinct models produce 19,168.

Both initial publication and replacement against populated accounting are
measured. An extra row is rejected by the payload cap after restoring its original
cost encoding; rejected staging has no seal and accepted totals stay unchanged.

| Capacity shape | Stage, min / median / max ms | Initial publication | Replacement stage | Replacement publication |
| --- | ---: | ---: | ---: | ---: |
| shared-labels | 1155.061 / 1183.901 / 1231.252 | 1292.125 / 1332.193 / 1365.144 | 1175.358 / 1184.068 / 1224.132 | 1929.468 / 1945.712 / 1972.870 |
| distinct-models | 1175.212 / 1180.632 / 1185.576 | 2459.965 / 2466.223 / 2564.168 | 1200.944 / 1216.062 / 1291.016 | 3326.488 / 3342.635 / 4039.657 |

These are diagnostic capacity timings with a 15 s deadline per source stage,
not tests of the 2 s source budget under pressure for these different shapes.
The distinct-model replacement can exceed the five-second import/refresh target
when stage plus publication are combined; it is outside the demonstrated
low-cardinality five-second envelope, despite fitting field/payload caps.

Each value below is the maximum across five trials, MiB. Disk combines archive,
transactional scratch, WAL and the single current stage/journal.

| Shape | Sampled additional HeapAlloc / HeapInuse | Stage file | Archive file | Archive WAL | Combined disk |
| --- | ---: | ---: | ---: | ---: | ---: |
| shared-labels | 6.030 / 6.172 | 85.586 | 171.176 | 172.214 | 428.989 |
| distinct-models | 131.423 / 131.766 | 85.586 | 256.934 | 258.467 | 600.992 |

Stage rollback journal peaked at 13,832 bytes; archive uses WAL and stage WAL
remained zero. WAL overhead is material: it is comparable to the populated
archive, and archive scratch pages are additional to accepted contributions.
The 64 MiB accounting cap does not imply a 64 MiB stage or disk reserve.
A 512 MiB **total footprint** allowance is insufficient for the distinct-model
probe. Free-space reservation must be defined relative to existing archive
allocation and additional growth, not compared blindly with total footprint.

The distinct-model probe reaches **131.423 MiB sampled additional HeapAlloc**.
That includes allocated garbage and is not proof that live heap exceeds 128 MiB;
it also cannot be used to certify a 128 MiB peak-live bound. The grouping map
retains one bucket per model/day/variant, so staging alone does not bound all
publication memory. Either accept explicit cardinality/resource limits or
investigate bounded/disk-backed grouping and stronger memory evidence before
claiming the broader envelope. Large aggregate labels/cardinality also amplify
archive rows and WAL. One shared-label cap probe would have missed this.

## Budget assessment and next decisions

| Proposed target | Evidence / remaining decision |
| --- | --- |
| Full 100k-shape import/sweep ≤5 s | Demonstrated for all existing V1/V2 shapes, including sustained synthetic pressure, no deferrals. Not a universal guarantee for every payload/cardinality combination. |
| Ordinary source ≤250 ms; exceptional ≤2 s | Demonstrated on those shapes, including pressure. Capacity diagnostics are a separate shape/deadline and do not expand this support claim. |
| 100-row changed session ≤25 ms | Capture through publication demonstrated; native correction authority remains a separate gate. |
| Input→renderer output ≤50 ms | Demonstrated with actual decoded bytes and program/renderer loop, including archive-write samples. Terminal painting is outside the endpoint. |
| Saved-history startup/query ≤50 ms | Saved aggregate query component fits; future saved-history startup remains an integration check. Source-browser initialization has baseline samples over 50 ms. |
| Cancellation ≤50 ms | Demonstrated for the measured pending query/pool/advisory waits with explicit 10 ms SQLite busy timeout. Not proof for arbitrary busy handlers/native children. |
| Yield after 25 ms / ten sessions | Session-boundary checks implemented and measured; strict 25 ms ceiling is not met. Accept those semantics or require another scheduler increment. |
| Additional live heap ≤128 MiB | Exact live bound remains unestablished; high-cardinality sampled HeapAlloc exceeds 128 MiB. Do not approve broad support from the small ordinary samples alone. |
| Stage/archive reserve | Approximately 429 / 601 MiB combined footprint at near-64 MiB payload for shared/distinct models. Specify reserve and archive-growth bounds or narrow supported dimensions; no numeric reserve is approved. |

Recommended next decision: retain the current reader/import/input/cancellation
targets for the demonstrated shapes, but do not accept field/payload caps alone
as the complete support envelope. Decide aggregate cardinality and resource
bounds, and whether the scheduler target is a boundary check or a hard ceiling.
Native version/accounting/deletion tracing can proceed independently.

MacOS and Linux normal/mock/vet/race/build gates pass;
[verification details](verification.md), [macOS output](gatec-macos-verification.txt),
[Linux output](gatec-linux-verification.txt). Hosted CI remains required after
normal submission. These research results do not approve the proposal or unlock
production steps 3–5.
