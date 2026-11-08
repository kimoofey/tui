# Plan 001: accepted minimal history contract

Status: DECIDED on 2026-11-08 at `f234344`. Research is closed by an explicit
scope reduction, not by claiming every previous gate passed. This is the
implementation contract for plans 002–004. No production history exists yet.
Read root and OCM AGENTS guidance. No external design document is required.

## Product and scope

Preserve observed OpenCode usage after deleting conversations; show UTC daily
recorded USD costs and provider/model totals. Retain accounting metadata only,
never titles, paths from conversations, prompts, responses, tool arguments or raw
JSON. Tokens remain five independent optional int64 counters (input, output,
reasoning, cache read/write); no repricing or invented overall token total.

Configuration-only opt-in: `history.enabled: true` in
`$XDG_CONFIG_HOME/ocm/config.yaml`, fallback `~/.config/ocm/config.yaml` for empty
or unset XDG_CONFIG_HOME. Missing config/field means false. Reject malformed YAML,
unknown fields and extra documents clearly. Read once at startup. Help/version
bypass config and resources. No new flag, toggle or environment enable override.

Disabled mode does zero history extraction, archive access, directory creation,
locking or capture. Existing browsing, source errors/no-session exit, deletion,
resume, root filtering, vacuum/prune and `--cost-period` retain current behavior.
The source remains read-only except existing explicitly requested native actions.
History must not change the current source-only period-cost calculation.

## Accounting and refresh

Use finite nonnegative float64 costs; sum before display formatting. No decimal
library or exact rounding contract. Missing/invalid cost is unknown, not zero;
valid zero is known. Validate counters as nonnegative int64, and detect aggregate
overflow (report unavailable rather than wrap). Missing provider/model is an
unknown group included in totals. Missing timestamps stay unbucketed: include in
all-time totals with an explicit count, exclude from dated ranges. Use UTC days
and half-open millisecond ranges. No timezone selection/rebucketing.

V1: recorded assistant usage joined to existing sessions, including hidden
children; distinct native IDs count independently. The existing fixture reports
7 recorded versus 4 actual-call reference. Persist and show “Approximate: V1
forked sessions may include copied usage.” V2: keep existing fork exclusion and
compaction-event rules; the repository oracle is 6.5 / 3 / 0 for its tested
ranges. Never fall back to frozen V1 tables when V2 exists, even empty.

Read one complete session in one source snapshot on a dedicated read-only handle.
Enumerate all sessions independently of the browser's root filter. Query/iteration/
transaction errors, cancellation, duplicate usage identities or unknown eligibility
(e.g. malformed role/compaction discriminator) fail that session's capture; never
publish a truncated observation. Missing optional accounting fields can remain
unknown. A present empty session differs from an absent session.

Archive contribution key: `(session_id, usage_kind, native_usage_id)` within one
bound source/schema. Per-session reconciliation is deliberately simple:

1. First complete observation is accepted, including known-empty sessions.
2. Repeat with identical keys/values is idempotent.
3. If every old key remains present, accept new keys and updated values atomically.
   Stable IDs within the same supported schema are sufficient authority for
   corrections, including decreases to zero and day/model changes. This explicitly
   replaces the old requirement for an independent correction-authority provider.
4. If any old key disappears, preserve the entire prior session snapshot, mark
   it stale with “source records removed; saved usage retained”, and do not mix in
   new keys from that observation. Also retain the prior snapshot if any previously
   known accounting field becomes unknown. This prevents silent erasure of known
   totals. Other sessions continue collecting. Guarded deletion of that scope fails.
5. Entirely absent sessions retain saved contributions. Successful native deletion
   never deletes archive rows. External deletion before first capture is unrecoverable.

The conservative stale-session rule can stop tracking later usage after pruning
or compaction changes identities. Document that limitation; automatic reconciliation
and a repair UI are deferred. Do not substitute a maximum-total heuristic.

## Archive and identity

Path: `$XDG_DATA_HOME/ocm/history.sqlite3`, fallback
`~/.local/share/ocm/history.sqlite3` for empty/unset XDG_DATA_HOME. Directory mode
0700, newly created database/lock files 0600. Keep WAL sidecars private. Use SQLite
WAL, foreign keys, `synchronous=FULL`, explicit user_version=1 and transactions.
A newer schema, corrupt archive or failed initialization returns an actionable
startup error; never silently reset it or fall through to unguarded enabled deletion.
No archive migration framework beyond transactional v1 initialization is needed.

One archive binds to one canonical absolute source database path and authoritative
schema family (`session` or `session_v2`) at first successful capture. Resolve path
symlinks in the adapter; do not change runtime session.Ref routing. Missing source
still permits archive reads. A different path/schema allows saved history and
browsing but disables collection/guarded deletion with a clear binding mismatch.
Return to the original source, or explicitly disable history accepting capture
gaps; no automatic merging, rebinding, clearing or migration is provided.
Same-path replacement/ID reuse and arbitrary copies are not reliably detected;
this is an explicit limitation, not a durable-lineage guarantee.

Use a lifetime exclusive nonblocking OS advisory lock on a sibling lock file,
keyed by canonical archive location; acquire it before opening/initializing the
archive. A second history-enabled instance fails clearly. On macOS/Linux use existing x/sys/unix support; lock release on process
exit, no PID-file stale-lock algorithm. Do not unlink a shared lock file. Close
on every startup failure/exit. This coordinates only instances using the same
archive. Separate archives, disabled OCM and native writers do not participate.

Minimal logical schema (executor chooses Go names and exact DDL):
- singleton metadata: schema/policy version, bound canonical path and source schema;
- sessions: native ID, last successful capture time, stale/error status, V1 quality;
- usage: composite key above, optional timestamp/cost/provider/model, five nullable
  counters and V1 quality. Foreign key to sessions. Index timestamp and session ID.

Daily/provider/model totals are SQL GROUP BY queries over retained usage, not a
second stored aggregate cache. This avoids double-counting refreshes and redundant
state. Persist no conversation payload. Variant filtering/storage is deferred.
All accepted rows plus successful observation metadata commit atomically per session;
failed-attempt status may be written separately without replacing accepted rows.

## Collection and process lifetime

One service-owned worker serializes capture/publication and deletion. Collect on
enabled startup, explicit browser/history refresh and after successful deletion.
Coalesce requests to at most one pending sweep. No periodic timer in this release.
Process sessions sequentially; check cancellation between rows/sessions, yield to
queued deletion between sessions. Use cancellable contexts with a configurable-in-
tests 30-second per-session safety timeout; it is not a performance promise.
Use in-memory accounting records for one session; no staging pipeline. This release
has no hard heap/latency guarantee and oversized sessions may fail capture.

All blocking work runs through tea.Cmd or the lifecycle-owned worker; Update and
View stay nonblocking. Each History response uses one short archive read transaction for totals, breakdowns,
unknown counts and stored status so every component reflects the same snapshot. On shutdown
cancel and join work before closing handles and releasing the lock. Entry points
own cleanup. Keep snapshot reads and publication serialized so older work cannot
overwrite a newer capture; UI query request IDs handle stale display responses.

## Deletion guarantee and explicit limits

While history is enabled, capture the target and its known native recursive deletion
scope, commit every required session, then invoke the existing native action.
Capture/binding/archive failure prevents that action. Preserve already committed
captures even if later capture or deletion fails. Hold the in-process serializer
through capture, action completion/timeout and authoritative reload.

Keep existing executable/database routing, V2 --standalone and bulk error/reload
behavior. Unsupported deletion scope disables guarded deletion; browsing/resume
remain available. Validate all requested refs before any side effects.

Guarantee: observations successfully committed before an OCM-requested deletion
survive it. No guarantee of capturing concurrent native writes or descendants
created after scope discovery. No atomic transaction across source/archive/native
process. A killed parent may leave a native child alive; restarting OCM does not
prove that child stopped. No intent journal, epochs, quarantine/recovery protocol,
automatic retry of deletion or surviving-child crash-test gate. Document these
limits; retain archive rows during partial source removal. Users should let any
still-running native deletion finish before retrying. Rerunning cannot recover
usage deleted before capture.

## UX and completion

`h` switches browser/history when enabled; ignored during confirmation or foreground
mutations. History starts at current UTC month; `[`/`]` previous/next month, `0`
current month, `a` all-time, `r` refresh, up/down or j/k scroll, esc/h back to browser
when available, ? help, q/ctrl+c quit. From all-time, [ or ] first returns to current
month. No day drilldown/preset menu/project/variant filter. Use horizontal daily bars
with amounts plus provider/model totals; all-time may paginate/scroll daily rows.

Show “Recorded USD (approximate)”, UTC, last capture, running/stale/failed/unsupported
status, unknown counts and retained V1 warning. Show “No captured usage” for no
observations; absence of rows is not proof of zero spend. Current month/day is
partial. Retention coverage is always limited to observed usage.

With no usable source or no sessions, enabled startup opens History, including an
empty archive. Source errors stay visible; no fabricated source capabilities or
zero period costs. Healthy archive access does not require OpenCode. Archive open
failure fails enabled startup. Disabled startup is unchanged.

## Deferred work and verification

No further performance/native lifecycle research is an implementation prerequisite.
Reuse existing extraction SQL and fixture oracles, but not research package imports
or its coordinator/staging architecture. A targeted inspection of current deletion
scope is part of 003, not a new supported-version matrix research project.

Plans 002–004 define focused tests plus repository normal/mock/vet/race/build gates.
Do not access live data or run real deletes. No dependency upgrades, PRQ changes,
new terminal integrations, other-agent adapters, daemon or generalized plugin API.
