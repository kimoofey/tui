> **Historical reference — superseded 2026-11-08.** The current release contract
> is [plan 001](../001-ocm-history-contracts.md); execution starts at
> [plan 002](../002-ocm-history-storage.md), as listed in the [index](../README.md).
> Measurements and fixtures below remain evidence for their original experiments.
> Old open gates, approval requirements, proposed precision/coordination/resource
> contracts and “production out of scope” statements do not govern implementation.
> Do not resume this research checklist. The accepted V1 approximation is retained
> in plan 001; other conflicting policies are replaced there.

# Step 2 closure status — 2026-11-07

Updated against `252b283` plus the isolated Gate C research increment.
Step 2 remains IN PROGRESS. This is a closure checklist, not a new implementation
milestone or approval of the [contract proposal](proposal.md). Only research code
under `plans/ocm-history-research/` changed in this increment.
Production history wiring remains out of scope.

## Verified and no longer blockers

- Real data is available; private extraction and a real V2 fork exclusion check
  pass. No larger real-database download is required. [Evidence](real-data.md).
- V1 recorded-usage approximation is accepted. Exact fork deduplication is deferred.
- Ordinary generated per-session reads fit the proposed reader budget. Snapshot,
  partial-stage rejection and collector-crash recovery fixtures pass.
- macOS normal/mock/vet/race/build gates pass. PRQ mock constraints are already
  fixed; no further PRQ maintenance is required.
- Linux **runtime** gates now pass in Docker, Go 1.25.0 linux/arm64: vet,
  repository race tests, make build, repository mock tests and mock builds.
  [Latest raw command output](gatec-linux-verification.txt), [verification details](verification.md).
  The earlier Docker-unavailable statements are historical and superseded.
- The [Gate C continuation](gatec-measurements.md) adds bounded database inventory,
  per-session reconciliation/publication, import/refresh/correction measurements,
  actual Bubble Tea input/renderer timing during archive writes (including a
  separate pinned-TrueColor run with RGB-output assertions), sustained
  synthetic writer/checkpoint pressure, blocked-query/advisory-lock cancellation,
  and a near-capacity initial/replacement disk probe. Numeric targets and support
  policy remain proposals. Latest macOS/Linux gates are recorded with that report.

Local Docker execution does not establish hosted Ubuntu/amd64 CI success for the
current uncommitted changes. The roadmap's explicit CI gate still needs a passing
run after these changes are submitted through the normal workflow. No branch was
pushed and no workflow was dispatched during this increment. Large opt-in scale
experiments were not rerun inside Linux; their recorded timings remain macOS data.

## Remaining work, in recommended order

| Priority | Required closure | Concrete evidence / decision needed |
| --- | --- | --- |
| 1 | Accept measured huge-session scope and remaining limits | [Optimization](optimized-collection.md) now completes all 10 huge-session attempts under the unchanged 2 s deadline (1.37–1.51 s). The Gate C continuation measures these shapes under sustained synthetic contention; explicitly accept the supported dimensions/resource caps and deferral/deletion policy for larger scopes. The original measured completion failure is addressed. |
| 2 | Accept Gate C budgets and resolve their remaining limits | [Measurements and limitations](gatec-measurements.md) now cover the full research pipeline, refresh/correction matrix, write-phase input/renderer latency, sustained synthetic pressure, blocked waits, sampled heap/RSS and near-capacity disk overhead. Decide scheduler semantics, numeric budgets, supported dimensions and disk reserve. Near-capacity distinct-model grouping reaches about 601 MiB disk and 131 MiB sampled additional HeapAlloc; field/payload caps alone do not establish the proposed resource envelope. Exact peak-live heap, native-writer guarantees, terminal painting, cold I/O and cross-machine guarantees are not established; require further evidence where the accepted contract needs those guarantees. |
| 3 | Supported native accounting/deletion matrix | Pin supported versions/layouts; validate compaction, revert/pruning/removal, migration identity mapping and skipped rows, correction authority versus lost records, and version-specific recursive deletion/bulk overlap/new-child scope. Use explicit conservative unresolved states where authority cannot be established. |
| 4 | Actionable parent-death recovery contract | Specify what admits another collector/deleter after OCM dies with a mutating native child alive. Define the recovery operation, admissible completion evidence, lock aliases/identity, bounded waits/cancellation, epochs and restart states. Prove the mechanism or justify an explicitly approved narrower contract with targeted macOS/Linux process evidence. |
| 5 | Freeze the handoff | Revise/approve the consolidated proposal, supply exact optional capability/result signatures and archive schema/keys/indexes/durability settings, define the explicit copy/import resolution operation and state transitions, accept budgets and update steps 3–5/design records consistently. |

The four research tasks for item 2 are implemented and measured; items 1–2 now
need explicit scope/budget decisions and any additional evidence those decisions
require. Item 3 native tracing is the next independent executable increment.
Final contract approval follows the evidence. No production collector, archive
startup, CLI or browser changes are authorized by this status update.

## Why the current evidence does not close these items

The [original bounded results](bounded-results.md) established capture-component
timings with failed huge-session completion. The [optimized run](optimized-collection.md)
now passes 11,000 ordinary observations and all 10 huge stages without diagnostic
retries. Neither capture-only run measures the full import/refresh pipeline; the new
[Gate C run](gatec-measurements.md) does. Its sampled heap and whole-worker RSS
are still not an exact additional live-heap bound. A permanently
deferred new session has unavailable history and cannot pass guarded deletion; it must
not count as successfully imported toward the five-second target without an
explicitly revised support contract.

The [earlier contention tests](contention_test.go) and scale experiment only
proved controlled single-update and headless read-phase behavior. The new
[program/pressure harness](responsiveness_test.go) measures an actual
input/event-loop/renderer path, accepted archive-write phases, sustained synthetic
writes/checkpoints and cancellation of blocked SQLite/pool/advisory-lock waits.
It does not establish native OpenCode workload guarantees or terminal painting.
The scheduler checks 25 ms / ten sessions at session boundaries; that is not a
strict 25 ms batch ceiling. Near-capacity disk demand includes scratch and WAL,
not just accounting payload. Near-capacity high-cardinality publication adds a
measured resource concern:
about 601 MiB combined disk and 131 MiB sampled additional HeapAlloc, and one
replacement stage-plus-publication exceeds five seconds. This does not prove
live heap exceeds 128 MiB, but prevents treating payload caps alone as accepted
support. Numeric acceptance remains open.

[Native trace](native-trace.md) pins candidate implementations but does not finish
compaction lifecycle, migration or a supported-version guarantee. The correction
oracle supplies authority as test input; it does not discover native correction
authority. Complete traversal is not evidence that disappeared usage was invalid.
The real fork observation settles only that particular V2 exclusion behavior.

The [proposal](proposal.md) specifies conservative quarantine but delegates the
actual recovery operation to Step 4. “Prove descendants quiescent” alone leaves
an implementation decision unanswered. Step 2 must specify evidence and expected
parent-death/live-child outcomes. The production coordinator and full Step 4
integration fixture can remain downstream after that contract is settled.
Similarly, interface role names and logical table lists are not the exact
signatures/schema/index contract required by Gate D. Identity resolution still
needs a concrete supported operation, including partial overlap and stale decisions.
Decimal arithmetic implementation belongs to step 3 once its contract is fixed.

## Finish criteria

For each A–D gate, record evidence and approved decisions rather than treating
passing tests as policy approval. Resolve the five items above, retain explicit
unsupported/unresolved cases, synchronize downstream handoffs, and obtain the
required hosted CI result. Then mark Step 2 done. Until then Step 3 remains gated.
