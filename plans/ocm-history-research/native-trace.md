> **Historical reference — superseded 2026-11-08.** The current release contract
> is [plan 001](../001-ocm-history-contracts.md); execution starts at
> [plan 002](../002-ocm-history-storage.md), as listed in the [index](../README.md).
> Measurements and fixtures below remain evidence for their original experiments.
> Old open gates, approval requirements, proposed precision/coordination/resource
> contracts and “production out of scope” statements do not govern implementation.
> Do not resume this research checklist. The accepted V1 approximation is retained
> in plan 001; other conflicting policies are replaced there.

# Pinned native tracing and remaining contracts

Research continuation on 2026-11-05; no native action was executed. This record
complements [the accepted V1 policy](decisions.md) and [experiments](results.md).
Source inspection is not a live-installation test or a supported-version range.

## V2 v2.1.4 — 68bc26b7da1990ce10ffecfbc1e09eb13aa5d824

| Link / code anchor | Verified behavior | Consequence |
| --- | --- | --- |
| [CLI delete handler](https://raw.githubusercontent.com/anomalyco/opencode/68bc26b7da1990ce10ffecfbc1e09eb13aa5d824/packages/cli/src/commands/handlers/session/delete.ts), `cli.session.delete` | Resolves a server using standalone setting, awaits a client session.remove request. | Trace must include private-server lifetime, not only the invoking CLI PID. |
| [Core session](https://raw.githubusercontent.com/anomalyco/opencode/68bc26b7da1990ce10ffecfbc1e09eb13aa5d824/packages/core/src/session.ts), `Session.remove`, lines 374–384 | Interrupts/awaits execution idle, closes transport, enumerates children, recursively removes them sequentially, clears environment, publishes deletion, removes bus aggregate. | A tree-wide atomic delete is not established. Children created after enumeration are a separate scope race. |
| [Projector](https://raw.githubusercontent.com/anomalyco/opencode/68bc26b7da1990ce10ffecfbc1e09eb13aa5d824/packages/core/src/session/projector.ts), Deleted and RevertEvent.Committed | Deleted removes the session projection. Committed revert removes message projections from a sequence boundary and removes affected inbox rows. | Usage can disappear while its session remains. A complete source read does not authorize reducing captured spend. |
| [Projection SQL](https://raw.githubusercontent.com/anomalyco/opencode/68bc26b7da1990ce10ffecfbc1e09eb13aa5d824/packages/core/src/session/sql.ts), SessionMessageTable | Messages reference sessions with cascade deletion; parent_id/fork_session_id are not equivalent message foreign keys. | Do not substitute one SQL foreign-key traversal for native recursive action scope. |
| [Bus](https://raw.githubusercontent.com/anomalyco/opencode/68bc26b7da1990ce10ffecfbc1e09eb13aa5d824/packages/core/src/bus.ts), `remove`, lines 709–720 | Aggregate event sequence and event rows are removed in a transaction. | This transaction is later than deletion publication in the caller; it does not prove a single transaction across the whole native action. |

The [history reader](https://raw.githubusercontent.com/anomalyco/opencode/68bc26b7da1990ce10ffecfbc1e09eb13aa5d824/packages/core/src/session/history.ts)
selects a compaction boundary for transcript replay. That does not establish that
all historical accounting rows survive every compaction, revert, pruning or
projection rebuild. Full compaction/event lifecycle tracing remains incomplete.

## V1→V2 migration

The pinned [Bun migration](https://raw.githubusercontent.com/anomalyco/opencode/68bc26b7da1990ce10ffecfbc1e09eb13aa5d824/packages/core/src/database/v1-migration.bun.ts)
is selected through the v1-migration module alias. Its assistant transformation
preserves source message ID, cost, token fields and creation time, converts model
fields to a nested reference and defaults a missing variant. Migration processes
sessions in transactions with a cursor, replaces each session's V2 messages, and
can skip invalid rows with warnings. This is useful mapping evidence, not proof
that arbitrary imports/copies share lineage or that every row transforms intact.

Proposed reconciliation behavior: schema transition first enters unresolved
migration state; preserve old contributions and their V1 approximation. A later
validated same-lineage mapping must match original usage IDs and accounting
fields, account for skipped/auxiliary records and variant normalization, and
preserve sticky quality. Never import V2 as an unrelated second copy merely
because its policy name differs. The current experiment deliberately stops at
unresolved migration; it does not implement or approve this mapping.

## Candidate usage shape matrix

| Shape | Experiment requires | Validation / remaining limit |
| --- | --- | --- |
| V1 session/message | session.id/time_created; message.id/session_id/time_created/data; assistant role and top-level providerID/modelID; optional variant; cost/tokens presence validated individually | Missing table/column fails even empty. Pinned v1.2.27 field evidence; full native install/migration compatibility remains unverified. V1 fork duplication accepted. |
| V2 projection/event | session_v2.id/time_created/fork_session_id; session_message.id/session_id/type/time_created/data; event.id/aggregate_id/type/created/data; nested model fields | All required tables probed inside one snapshot; no frozen V1 fallback. Candidate v2.1.4 accounting matches repository oracle. Not a blanket promise for every V2 version. |
| Unknown/incompatible V2 | No fallback | Extraction fails with no publishable observation; ordinary browser behavior unchanged. |
| Empty valid authoritative schema | Explicit complete empty observation for selected scope | Not proof of historical zero; reconciliation cannot erase old rows based on absence. |
| Missing session | Absent in candidate membership map | Distinct from a present, known-empty session. Captured usage remains. |

## Contract states to review

State, scan completeness, quality, unknown values, freshness and coverage are
independent. The following is a proposal; it has not been approved as an API.

| Collection state | Retained view / actions |
| --- | --- |
| Uninitialized | No first observation; historical coverage unknown, never zero by assumption. |
| Backfill | Show accepted saved scopes; progress describes observed candidates only. |
| Running | Show previous committed generation until the next scope is accepted. |
| Stale | Retain totals and last successful observation time; schedule under chosen trigger policy. |
| Failed | Preserve prior generation; report extraction/archive diagnostic, retry under policy. |
| Source unavailable | Saved data remains readable; no new capture or protected action. |
| Archive-only | Saved totals/quality survive without source sessions/executable. |
| Unresolved lineage/migration | Suspend collection/publication and enabled deletion; browsing stays unchanged. Resolve explicitly and persist the decision before resuming. |
| Unresolved accounting | Keep last accepted scope, show pending reconciliation, no overlapping partial merge. Enabled deletion cannot bypass required successful capture. |
| Unfinished native operation | Keep committed capture; block competing protected work until the approved recovery contract establishes safe admission. |

A proposed identity resolution path is explicit registration/confirmation with a
persisted opaque lineage ID, and explicit same-lineage mapping or separate-lineage
registration when evidence is ambiguous. Partial overlap must be resolved before
adding totals; a separate ID alone cannot prove disjoint spending. This proposal
needs exact UX/metadata and maintainer acceptance before implementing the service.

## Ordering remains a blocker, not an inferred fix

The required protected interval starts before opening the source snapshot and
ends after archive publication or actual native deletion completion, including
relevant server/descendant mutations. OCM's CommandContext watcher and WaitDelay
live in the parent. No independent surviving-child coordination mechanism has
been implemented or verified here. Native recursive/event cleanup evidence above
does not justify narrowing ordering to parent lifetime.

The mandatory step-4 test must kill a fixture OCM parent while its fake native
child remains blocked and alive, attempt competing collection/deletion from a
second process, then release the child and verify bounded recovery with retained
accounting. Parent death, lock release, PID absence and timeout cannot substitute
for action completion. Choose a mechanism only with macOS/Linux process evidence;
the ordinary race suite is not that evidence.

Remaining before step 3: approved timezone, exact optional capability signatures
and durable lineage protocol, validated correction authority and migration map,
archive schema/path/permissions/versioning, scheduler and publication contract,
process ordering/recovery, measured/accepted budgets, native compaction scope,
full performance gate and resolution or explicit acceptance of the unrelated PRQ mock
verification failure. Experiments reduce uncertainty; they do not approve policy.

2026-11-06: [real-data extraction](real-data.md) now passes on the available private
V2 snapshot. The sample has no fork sessions or events, so the native-behavior
limits above remain. The accepted real-sample/generated-scale route does not
waive compaction, deletion or recovery evidence.
