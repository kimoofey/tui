# Step 2 closure review — 2026-11-07

Reviewed against `50c693e` plus the current uncommitted research increment.
Step 2 remains IN PROGRESS. This is a closure checklist, not a new implementation
milestone or approval of the [contract proposal](proposal.md). No source code was
changed during this review. Production history wiring remains out of scope.

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
  [Raw command output](linux-verification.txt), [verification details](verification.md).
  The earlier Docker-unavailable statements are historical and superseded.

Local Docker execution does not establish hosted Ubuntu/amd64 CI success for the
current uncommitted changes. The roadmap's explicit CI gate still needs a passing
run after these changes are submitted through the normal workflow. No branch was
pushed and no workflow was dispatched during this review. Large opt-in scale
experiments were not rerun inside Linux; their recorded timings remain macOS data.

## Remaining work, in recommended order

| Priority | Required closure | Concrete evidence / decision needed |
| --- | --- | --- |
| 1 | Accept measured huge-session scope and remaining limits | [Optimization](optimized-collection.md) now completes all 10 huge-session attempts under the unchanged 2 s deadline (1.37–1.51 s). Confirm under sustained contention in item 2 and explicitly accept the supported dimensions/resource caps and deferral/deletion policy for larger scopes. The original measured completion failure is addressed. |
| 2 | Complete Gate C measurements | Measure bounded inventory → capture → reconciliation → per-session publication, initial import and unchanged/new/changed/old-correction refresh; browser contention during archive writes; actual input/event-loop/render latency; sustained writer/checkpoint pressure; blocked-query cancellation; peak heap/RSS, stage disk reserve and archive WAL/journal overhead. Record distributions and agree numeric budgets. |
| 3 | Supported native accounting/deletion matrix | Pin supported versions/layouts; validate compaction, revert/pruning/removal, migration identity mapping and skipped rows, correction authority versus lost records, and version-specific recursive deletion/bulk overlap/new-child scope. Use explicit conservative unresolved states where authority cannot be established. |
| 4 | Actionable parent-death recovery contract | Specify what admits another collector/deleter after OCM dies with a mutating native child alive. Define the recovery operation, admissible completion evidence, lock aliases/identity, bounded waits/cancellation, epochs and restart states. Prove the mechanism or justify an explicitly approved narrower contract with targeted macOS/Linux process evidence. |
| 5 | Freeze the handoff | Revise/approve the consolidated proposal, supply exact optional capability/result signatures and archive schema/keys/indexes/durability settings, define the explicit copy/import resolution operation and state transitions, accept budgets and update steps 3–5/design records consistently. |

Item 2 is the next executable performance increment; item 1 still needs scope/limit acceptance. Native tracing can
proceed independently; final contract approval follows the evidence. This review
does not require production collector, archive startup, CLI or browser changes.

## Why the current evidence does not close these items

The [original bounded results](bounded-results.md) established capture-component
timings with failed huge-session completion. The [optimized run](optimized-collection.md)
now passes 11,000 ordinary observations and all 10 huge stages without diagnostic
retries. Neither run measures the full import/refresh pipeline. New sampled heap
and whole-worker RSS are not an exact additional live-heap bound. A permanently
deferred new session has unavailable history and cannot pass guarded deletion; it must
not count as successfully imported toward the five-second target without an
explicitly revised support contract.

[Existing contention tests](contention_test.go) perform one controlled source
update/checkpoint and headless Update/View calls. They do not run sustained native
writing, a real program/input/renderer loop or prove archive-write phase sampling.
The earlier scale experiment's concurrent browser samples all started in the read
phase. Source-lock blocking and a 25 ms scheduler yield are still unmeasured.

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
