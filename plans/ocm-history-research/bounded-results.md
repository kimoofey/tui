# Bounded collection and huge-session staging — 2026-11-07

Implemented the next isolated step-2 experiment. No production collector, source
writes, deletion, or private-data reads. **Gate C remains open.** Per-session
snapshots address the measured multi-session reader-lifetime failure. Huge-session
staging is safe to abandon but does not reliably complete inside the proposed
2 s exceptional budget. See the consolidated [proposal](proposal.md) for review;
measurements do not approve contracts or unlock steps 3–5.

## Reproduction and scope

```sh
GOTOOLCHAIN=go1.25.0 go test ./plans/ocm-history-research
GOTOOLCHAIN=go1.25.0 go test -v ./plans/ocm-history-research -run '^TestHistoryBoundedScale$' -count=1 -timeout=10m -args -history-bounded -history-samples=3
```

Set GOCACHE to a writable directory if needed. Final run used
`/private/tmp/ocm-history-20261106-tj193pol/go-build`. It completed in 58.705 s,
exit 0. [Raw final output](bounded-results.txt) contains 18 fresh-process samples,
three per existing generated shape. Go 1.25.0, darwin/arm64, Apple M1, SQLite
3.53.1/module driver, APFS; warm filesystem, not cold I/O. No other verification
commands ran concurrently with the final run. Earlier exploratory output was
replaced, not combined with this distribution. Three samples are not p95 evidence.
Fixtures and independent oracle are unchanged from [scale results](scale-results.md).

Code: [stage implementation](stage.go), [safety and measurement tests](stage_test.go),
[extractor](extract.go), [fresh worker integration](scale_test.go).
Each ordinary read has its own source transaction, a 250 ms context deadline and
a 1,000-source-row cap. V2 fork-boundary validation is scoped to the selected
session. Each source transaction still detects schema and validates required
columns. A fixture-owned inventory supplies known session IDs; production
inventory enumeration, concurrent inventory changes and source identity are not
measured. The source is always opened mode=ro on a dedicated connection.

The worker combines only small day/model/token oracle buckets across ordinary
sessions, discarding each observation before the next. It never retains a full
100k-usage observation. The independent construction oracle checks the final
buckets. Snapshots are per session, not one coherent source-wide observation.
There is no archive publication on this path and no correction/lineage authority
is inferred. The loop yields every ten sessions; a real scheduler's 25 ms yield,
fairness and event-loop behavior remain unimplemented/unmeasured.

An oversized/timed-out ordinary attempt is discarded. A new source snapshot
streams accounting-only records into one separate SQLite stage transaction;
source pages from different snapshots are never combined. A seal is inserted
only after successful source iteration and transaction completion. Stage rows
and seal commit together. Metadata-only extractor results have Complete=false
and cannot go through the existing candidate writer. A sealed stage certifies
one session's traversal, not publication authority or deletion safety.

The source deadline includes parsing and synchronous stage inserts: it bounds
source-reader occupancy with a cancellable deadline rather than excluding staging
work from the measurement. Deadline return can overshoot; it is not hard real-time.
Stage metadata fields cap at 4 KiB each; total serialized accounting plus keys
caps at 64 MiB. These are defensive prototype limits, not approved capacities;
SQLite pages/journals and temporary engine allocations are additional. No claim
of handling arbitrarily large records with a fixed process-memory ceiling.

## Results

Each cell is min / median / max over three fresh processes, milliseconds.

| Shape | Sweep time | Slowest source attempt in each process |
| --- | --- | --- |
| V1 10k / 100 sessions | 172.117 / 172.614 / 175.049 | 4.952 / 4.999 / 5.453 |
| V2 10k / 100 sessions | 192.611 / 192.937 / 193.074 | 5.722 / 5.860 / 6.011 |
| V1 100k / 1,000 sessions | 2647.967 / 2655.719 / 2989.266 | 7.536 / 7.689 / 17.404 |
| V2 100k / 1,000 sessions | 2796.113 / 2873.519 / 3179.206 | 6.673 / 6.827 / 11.141 |

All 6,600 ordinary session snapshots completed, and combined day/model/cost/token/
quality oracles pass. These sweeps fit the proposed five-second target for the
**capture component only**. Registry, inventory, archive publication, reconciliation
and coordination costs are not included. Small-session capture fits 25 ms in these
samples; total capture + publication still has no evidence.

| Huge shape | Bounded stage return | Completed within 2 s | Diagnostic full stage, new snapshot | Completed stage size |
| --- | --- | --- | --- | --- |
| V1 100k / one session | 2002.006 / 2002.477 / 2002.755 | 0 / 3 | 2011.701 / 2039.126 / 2168.850 | 22.988 MiB |
| V2 100k / one session | 2001.669 / 2001.683 / 2001.895 | 0 / 3 | 2303.997 / 2323.136 / 2340.350 | 23.203 MiB |

Every bounded huge attempt deferred with no staged rows or seal remaining. The
worker then retries with a **15 s diagnostic deadline**, solely to measure complete
staging cost and verify the independent full oracle. This is not an operational
fallback or an implicitly relaxed reader contract. Stage size and huge-shape total
runtime in raw output include that diagnostic pass; they are not sizes of failed
stages. Timed-out reader returns overshot by at most 2.755 ms in this run. That is
not a blocked-lock cancellation measurement.

The combined ordinary attempt + bounded stage + diagnostic retry took 4.268–4.425 s
V1 and 4.562–4.598 s V2. This includes discarded work, so it cannot be reported as
a successful two-second capture or an allowed five-second import path.

End-of-interval additional HeapAlloc was 0.432–1.785 MiB for multi-session 100k
and 0.234–1.872 MiB for huge shapes; cumulative allocation was 431.007–431.021 MiB
V1 100k, 413.996–414.006 MiB V2 100k, 920.773–933.766 MiB V1 huge and
787.070–800.971 MiB V2 huge. Huge allocation includes the failed stage and diagnostic
retry. End-of-interval heap includes garbage and excludes the subsequent stage
oracle load; **it is neither sampled peak, peak live heap, RSS nor post-GC retained
memory**. Do not use it to declare the 128 MiB live-heap budget passed. The new
worker does not collect RSS. This evidence demonstrates reduced retention design,
not a measured peak-memory bound for arbitrary huge metadata/source JSON.

## Safety fixtures

Default normal/mock/race tests validate V1 and V2:

- Row-bound interruption, context cancellation after emitted rows, injected stage
  insert failure and seal failure roll back rows and seal. Prior accepted
  accounting remains unchanged through conservative reconciliation.
- Missing session is rejected as loss rather than sealed as a known empty scope.
- A WAL writer deletes a usage and inserts another after the reader snapshot
  opens; staging retains exactly the prior three usages, including the deleted ID.
- Reopening the stage retains the committed seal; metadata-only results cannot
  publish through the whole-fixture candidate writer.
- Ordinary collection, row-cap-triggered full-snapshot retry and pre-cancelled
  operation select the correct route. Oversized accounting labels fail closed.
- A separate test process exits inside stage insertion without running defers.
  SQLite journal recovery exposes no rows/seal; a full retry seals exactly once.
  This tests collector staging death, **not a surviving native deletion child**.

## Design consequence and remaining work

Keep separate per-session snapshots and atomic per-session publication in the
proposal. Reject live keyset usage paging as evidence of snapshot completeness.
Under the current proposed exceptional deadline, huge sessions can be deferred
indefinitely: preserve prior accounting, show unavailable/deferred for new scopes,
and block guarded deletion. Do not claim successful huge-session support.
A different bounded extraction/staging strategy or an explicitly approved deadline
change with writer/WAL/UI evidence is still required for eventual huge-session
capture. An offline consistent snapshot could be a separate explicit route; a
live database backup cannot be assumed to have bounded source-reader occupancy.

Still open: browser contention during archive writes, actual input/render latency,
sustained writer/checkpoint activity, blocked-query cancellation, source inventory,
publication, correction authority, supported native semantics, durable identity,
coordination and deletion recovery. Repository gates pass locally; Linux runtime
is unresolved. See [verification](verification.md) and the [proposal](proposal.md).
