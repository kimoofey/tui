# Step 2 implementation contract proposal — 2026-11-07

**For review; not approved and not a step-2 exit declaration.** This is the
single proposed contract for steps 3–5. The accepted V1 approximation in
[decisions](decisions.md) remains binding. This document consolidates remaining
choices, including conservative behavior where native provenance is insufficient.
Measurements are in [bounded collection](bounded-results.md),
[scale results](scale-results.md) and [real data](real-data.md); native evidence
and unresolved questions remain in [native trace](native-trace.md).

## Accounting and retained data

Use UTC calendar days and half-open millisecond timestamp ranges. Label UTC in
the view. Store native timestamps as signed int64 milliseconds; an unavailable
or invalid timestamp stays unbucketed, appears in all-time totals, and is
excluded from dated ranges with an explicit unbucketed count. Do not invent dates.

Retain recorded USD cost as a canonical, nonnegative base-10 decimal string,
parsed from the original JSON number with arbitrary-precision decimal arithmetic.
Retain the original finite value without fixed-scale quantization; aggregate with
exact decimal addition, rounding only the displayed total to six fractional
digits (half-even). Reject resource-exhausting numeric encodings (over 128
significant digits or exponent magnitude over 128) as unknown with diagnostics.
Native recorded cost is not a billing invoice. No price-table reconstruction.
The current float64 prototype does **not** implement this proposed contract;
step 3 needs independent rounding, overflow and mixed-magnitude oracles.

Cost and each of the five int64 token counters carry independent known/unknown
status. Recorded zero remains known zero. Missing/malformed/negative/nonfinite
values are unknown; aggregate known subtotal plus unknown counts. Checked token
addition must report overflow as unavailable rather than wrap. Preserve missing
provider/model/variant independently, using explicit unknown categories, not
synthetic provider identifiers. Unknown variant differs from a recorded empty
variant. Unsupported schema is unavailable, never a complete empty source.

Retain source UUID, source generation, native session/usage identity and kind,
native timestamp plus presence, provider/model/variant plus presence, decimal
cost plus presence, five token counters plus presence, extraction/policy version,
quality and diagnostics, observation generation/time and reconciliation status.
Retain session parent/fork identifiers only where necessary to validate scope and
accounting; retain a digest of the validated fork boundary. Retain no titles,
paths from session content, prompts, messages, tool arguments, parts or raw JSON.
Local source locator/executable information belongs in the private registry,
not accounting rows. All V1 contributions retain their approximation warning.
Completeness, quality, freshness and unknown values remain separate dimensions.

## Source identity and ambiguous copies

Assign a random archive-owned source UUID on explicit first binding. A path,
inode, database size, timestamps or schema hash alone is never durable identity.
Store canonical locator, executable/channel/storage selection, observed file
identity and schema fingerprint as diagnostics. Source generation changes on
verified migration/replacement. A moved or replaced file requires validation
against its binding before capture or deletion; never route actions using only
native session IDs.

Fail closed when a new locator overlaps known native IDs, a bound file is
replaced, or migration/import lineage is uncertain. Keep existing totals and
mark the entire affected source unresolved; do not count a second copy. Require
an explicit registry resolution with one of three meanings: bind this locator to
the existing source; exclude it as a copy; or attest that it is an independent
source and allocate a new UUID. A missing source is not automatically forgotten.
Resolution must include the expected registry generation and observed fingerprint;
stale decisions fail. The implementation must expose this as an intentional
configuration/maintenance operation before shipping, with a reviewable overlap
summary; silent auto-resolution and title/content heuristics are prohibited.
A claimed independent copy may contain the same spend; explain that consequence.

For migration, require a validated native identity mapping for the supported
version pair before replacing the generation. Without it freeze that scope for
explicit resolution. A migration does not clear V1 approximation or reimport
potentially overlapping V2 usage alongside V1 contributions.

## Correction and retention

An initial complete session observation establishes its accepted generation.
An unchanged repeat is idempotent. New IDs can be added only when the accepted
IDs remain present and unchanged under the validated accounting policy. A changed
known ID requires native correction authority validated for that version/layout;
identity and newer timestamps alone do not confer authority. Otherwise retain
the prior entire session and label unresolved-correction. Apply approved changes,
including downward amounts, zero and reattribution, atomically with aggregates.

Missing IDs, missing sessions, compaction replacement and ambiguous imports retain
the prior whole session, marked unresolved-record-loss; never mix replacement
usage with possibly overlapping prior usage. No maximum-total heuristic. No
expiry for accepted accounting or deletion evidence in the initial release.
Discard unsealed staging on startup; sealed but unpublished staging requires
source-generation and coordination checks before reuse. Policy changes require
an explicit migration/reprocessing rule; unavailable source data may remain
approximate indefinitely. Retention does not imply capture of unobserved spend.

## Archive and versioning

Propose `$XDG_DATA_HOME/ocm/history.sqlite3`, falling back to
`~/.local/share/ocm/history.sqlite3` for unset/empty XDG_DATA_HOME. Create the
OCM directory mode 0700 and database/lock/staging files mode 0600. Stage in a
private sibling directory on the same filesystem; never put staging in a source
or infer archive identity from the current working directory. Disabled history
performs zero archive, registry, lock or staging access.

Use SQLite WAL for the archive with one writer, foreign keys enabled, explicit
schema `user_version`, and independent accounting-policy/extractor versions.
Logical tables: source registry and generations; accepted sessions; per-usage
contributions; daily/provider/model/variant aggregates with unknown counters;
collection attempts/diagnostics; deletion intents and explicit resolutions.
Use composite source/generation/session/kind/usage keys. Daily totals are derived
and replaceable from retained contributions. Migrations are transactional; a
newer unsupported version fails clearly without writing or deleting history.
Take a private archive backup before destructive schema migration. Prototype
candidate/stage schemas are not the proposed production migration.

## Collection, completeness and publication

Show saved history first. Start a background inventory/capture on enabled startup,
after authoritative browser refresh, after a successful source action, and on a
five-minute full reconciliation timer. Coalesce triggers; one active collection
worker per source. Cancellation interrupts queries and prevents publication.
No native timestamp-only freshness shortcut is trusted. A sweep inventories all
sessions, including hidden children, with bounded keyset inventory queries;
inventory pages are discovery only and never certify one source-wide snapshot.
Sessions created/deleted during a sweep may wait for the next sweep. An empty
session and an absent session are different states.

Capture each session in one read-only transaction on a dedicated source handle.
Probe authoritative schema and all required fields in that snapshot. A complete
session means every eligible row and compaction event was traversed successfully,
with iteration, cancellation and transaction-completion checks. It does not mean
billing completeness or one coherent source-wide snapshot. Preserve the existing
V2 fork exclusion and event-only compaction accounting; accepted V1 rule unchanged.

Try at most 1,000 source rows within 250 ms, then discard that attempt and retry
in a new snapshot streaming accounting only to disposable disk staging. Never
concatenate live-source pages across snapshots. Exceptional staging has a 2 s
deadline, a 4 KiB per metadata field cap and a 64 MiB accounting-payload cap;
SQLite file/journal overhead is additional and needs an explicit disk-space
reserve. No deadline extension is an operational fallback. On exhaustion mark
session deferred/oversized and preserve prior accounting; for new sessions show
unavailable rather than zero. Automatic deletion is prohibited for that scope.
The current experiment shows whether this policy is feasible, not that every
huge session can complete. Larger sessions need a separately approved design
(e.g. an operator-provided consistent offline snapshot), not silent partial capture.

Seal a stage only after the source read has completed successfully. An unsealed
or interrupted attempt cannot replace accepted data. Before publication validate
source UUID/generation, binding fingerprint, accounting policy, coordinator epoch
and absence of unresolved deletion intent. Apply reconciliation, contribution
replacement and daily changes in one archive transaction with a compare-and-swap
on the prior session generation. Rejected publication keeps the old generation.
No source writes, migrations or source-side revision tables.

## Coordination, deletion and recovery

Use one OCM-owned advisory per-source lock shared by every cooperating OCM
collector and deleter, acquired before source capture and held through publication
or the complete deletion workflow. A registry transaction allocates monotonically
increasing operation epochs; stale collector output cannot publish. UI cancellation
must not imply native action completion. Source-native writers do not honor this
lock: retained data describes a completed snapshot, not a promise to capture
writes made after it. Before-delete capture and action scope must include every
session the supported native version can delete, including descendants when
applicable. Unsupported versions/scopes disable guarded deletion, not browsing.

Write and fsync a durable deletion intent (operation UUID, source generation,
executable/version, target/scope, capture generation, state) before launching the
native process. Capture and archive commit must precede launch. Record launch,
observed exit, authoritative reload and resolution separately. Exit failure,
timeout, parent death, lock release and OCM restart do not prove that descendants
stopped mutating source. A fresh OCM process seeing any unresolved intent freezes
publication and further deletion for that source and shows retained history.
It never reruns deletion automatically.

If OCM dies while native deletion continues, keep the pre-delete archive and the
intent unresolved. Require explicit recovery evidence that the native operation
and all mutating descendants are quiescent, followed by schema/identity validation
and a fresh authoritative source reload/capture. PID absence/reuse and a stable
session list alone are insufficient. If quiescence cannot be established, leave
the source quarantined; an operator can deliberately retire the binding while
retaining history. Process supervision can improve liveness but is not sufficient
proof by itself. Step 4 must provide a concrete recovery operation and surviving
child fixture before deletion is enabled. Native scope/compaction/migration
research remains a prerequisite; the real fork check resolves only fork exclusion.

## Interfaces and proposed budgets

Keep history optional in `ocm/session`, separate from session listing. Proposed
roles: `HistorySource.Inspect(ctx)` returns schema/version/capabilities and source
binding evidence; `HistorySource.CaptureSession(ctx, ref, limits, sink)` emits
accounting records and returns scope/completeness metadata; `HistoryStore.Stage`,
`Publish(expectedGeneration, epoch)`, `ReadTotals`, and deletion-intent methods
manage OCM storage. The service owns orchestration, locking, cancellation,
reconciliation and recovery. Adapters know native SQL/routing; UI receives status
messages through tea.Cmd. Entry points own all handles and cleanup. Exact Go
signatures must preserve independently known fields and checked decimal values;
freeze them only after remaining validation, rather than exporting prototype types.

| Target | Proposed contract / current limitation |
| --- | --- |
| Ordinary source transaction | 250 ms context deadline; measure return/rollback overshoot separately |
| Exceptional source transaction | 2 s context deadline, defer on exhaustion; not a hard real-time guarantee |
| Scheduler | Yield after 25 ms or 10 sessions, whichever first; exceptional session cannot be preempted without discard |
| Ordinary changed session | 25 ms capture + publication target for a 100-row shape; publication still unmeasured |
| Initial 100k import / full sweep | 5 s target excluding deferred sessions; failures cannot be hidden from status |
| Saved-history startup/query | 50 ms target; current tiny SQL query is only one component |
| Input-to-rendered-frame | 50 ms target; requires actual program/input/renderer evidence |
| Cancellation | 50 ms return target; blocked-lock case still unmeasured |
| Additional live heap | 128 MiB target; sampled/retained heap and RSS are different measurements |
| Stage resources | One stage per source; 4 KiB field / 64 MiB accounting payload; disk reserve still needs measurement |

## Review and exit conditions

Approve or revise this proposal as a whole after the remaining measurements:
archive-write browser contention, actual input/renderer latency, sustained native
writer/checkpoint impact and blocked-query cancellation. Validate native
compaction, migration, correction authority versus loss, supported-version
scope and surviving deletion children. Add archive publication/session-generation
oracles, coordinator recovery and resource-capacity evidence. Required repository
gates and Linux runtime/CI must pass. Until then step 2 remains in progress and
steps 3–5 remain gated. This proposal does not convert unresolved evidence into
an accepted implementation contract.

2026-11-07 optimization follow-up: the [profiled research path](optimized-collection.md) now
completes all ten generated huge-session stages in 1.37–1.51 s under the unchanged
2 s deadline. Ordinary capture, atomicity and accounting checks pass; macOS/Linux
gates pass. Historical failed timings remain valid for the old implementation.
Sustained contention, actual UI latency, full publication and native/contracts
work remain open. No production collector or policy approval is inferred.


2026-11-07 Gate C continuation: the [full research measurements](gatec-measurements.md)
now include inventory through per-session publication, unchanged/new/changed/old
correction refresh, actual input/event-loop/renderer output with accepted archive
write phase coverage, sustained synthetic writer/checkpoint pressure, blocked
wait cancellation and near-capacity initial/replacement disk overhead. The
scheduler checks 25 ms / ten sessions between completed sessions, not as a hard
batch ceiling. Sampled heap/RSS does not establish exact peak-live heap, and
payload bytes alone do not bound disk demand. Review the report's numeric and
scope decisions before accepting this budget table. These measurements do not
approve this proposal, native semantics or production interfaces/schema.
