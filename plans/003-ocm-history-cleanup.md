# Roadmap step 4: capture usage before OCM cleanup

Status: TODO, blocked on step 3's exit gate. Planned at `0c710a6` on 2026-11-05.
Priority P1; effort L; risk high (irreversible source deletion).

2026-11-05 prerequisite update: step 2 research can resume; see its
[evidence pack](ocm-history-research/README.md). Native V2 deletion scope and
surviving-child recovery remain unverified. No cleanup integration is authorized
by the isolated snapshot/fork experiments. V1 recorded-usage approximation is
accepted in [decisions](ocm-history-research/decisions.md); it does not relax
capture completeness, deletion-impact discovery or crash recovery.

Executor: read root/OCM AGENTS instructions and this entire plan. Run
`git diff --stat 0c710a6..HEAD -- ocm cmd/ocm`. Compare current behavior below,
read step 2's approved deletion/ordering contract and step 3's implementation.
Update this plan for expected prerequisite changes before coding.

Step 2 continuation: [experiment results](ocm-history-research/results.md) and
[native tracing](ocm-history-research/native-trace.md) now exist. These do not
approve remaining signatures, schema, budgets, identity or deletion recovery;
refresh this plan only after those contracts pass step 2's gate.

## Outcome and boundaries

When history is enabled, capture the target and all sessions its deletion can
affect, commit their contributions, then call the existing native deletion.
Failed capture prevents that affected deletion. Failed deletion retains captured
usage and can be retried without duplication. Disabled deletion follows the
existing path with no history work, even with an old archive present.

In scope: `ocm/service/operations.go` and narrow injected history contracts,
OpenCode deletion-impact extraction and native action lifecycle behind the
adapter, history coordination,
CLI wiring, existing operation messages if more diagnostics are needed, real/mock
command wrappers/tests, `ocm/README.md`, design records/index. Keep UI changes
limited to capture progress/failure/partial-result messages.

Out of scope: replacing the native deletion with SQL, new confirmation keys,
History navigation, period-cost changes, vacuum/prune synchronization, other
sources, dependency upgrades, terminal launch refactors. Do not make pruning or
vacuum trigger usage collection; they remain explicit independent maintenance.

## Current code anchors

`ocm/service/operations.go:Delete` validates **every ref before side effects**,
checks optional Deleter, executes sequentially, and always performs one final
authoritative `Load` after the valid batch's attempts. Its per-action core is:

```go
callCtx, cancel := context.WithTimeout(ctx, deleteTimeout)
err := callCtx.Err()
if err == nil {
    err = source.Delete(callCtx, ref.ID)
}
cancel()
```

`deleteTimeout` is 30 seconds. `actions.go:runDelete` binds the discovered
executable/storage and has a 250ms WaitDelay. Preserve both; capture has its own
approved budget and must not consume the CLI's 30s budget.
`exec.CommandContext` and WaitDelay do not establish that deletion ends when OCM
dies. Treat the parent, native child and any source-mutating descendants as
distinct lifetimes when implementing the step 2 recovery contract.

`DeleteResult{Count, Snapshot, Err}` currently counts successful requested native
calls, not cascade rows. Joined errors preserve per-target failure plus reload
error. Retain that meaning unless an explicitly approved contract changes it.
Failed reload leaves the browser's prior snapshot intact; successful empty
reload is authoritative. Never substitute manual removal of selected rows.

`ocm/model.go` uses a two-press `d` confirmation and busy locks.
`ocm/ops.go` executes `app.Delete` in a Cmd; messages flow through `Update`.
`ocm/ops_mock.go` explicitly marks acknowledgements; nil data is not an empty
reload. Keep normal/mock semantics compatible.

Test exemplars: `service/service_test.go:TestForeignReferencesNeverAct` and
`TestDeleteRefreshAndPerCallTimeout`, `ocm/operations_test.go` commands fed back
through Update, and `cmd/ocm/operations_test.go` fake CLI subprocesses. That
helper currently deletes only a target and **direct children**; it is not
evidence of recursive upstream cascade behavior.

## Prerequisite gate

Step 3 must have working idempotent transactional collection and cross-process
ordering. Step 2 must specify supported-version cascade semantics, descendant
discovery, overlap/missing-target behavior, capture scope, timeout, archive error
policy, surviving deletion-child ownership/recovery and concurrency limitation.
If those are absent, stop before wiring
deletion. Existing child-refresh tests alone do not satisfy this gate.
Default ordering remains coordination through native action completion. Any
narrower crash-recovery contract must have step 2's explicit approval, pinned
source evidence and defined safe competing-operation outcomes. Update this
plan's affected gates to match before implementation; do not treat a crash as
an implicit exception. Unresolved lineage or reconciliation authority must
follow the approved policy and cannot silently authorize unprotected deletion.

The allowed guarantee: while enabled, OCM requires a fresh committed capture
before its own deletion; captured usage survives deletion. OpenCode can write
during capture/delete, and new descendants can appear; no atomic cross-system
transaction exists. Follow the approved policy for detecting/handling scope
changes. Do not claim protection for activity in flight or previously unseen
usage deleted outside OCM/while disabled.

Accepted V1 approximation alone is not a capture failure and must not block
otherwise supported enabled deletion. Capture its recorded usage and approximation
metadata before invoking the CLI. Incomplete reads, unknown deletion scope,
archive failures and unresolved lineage/reconciliation still follow the guarded
failure policy. Add a V1 fork test showing successful capture retains the 7
recorded total and its quality marker after deletion, without claiming the 4
actual-call reference was recovered.

## Increment A — establish impact scope and service boundary

1. Implement the approved optional deletion-impact capability behind the
   adapter. Enumerate descendants from authoritative source storage, including
   hidden subagents and arbitrary depth, not the displayed browser list. Define
   fork relationships separately from parent/subagent relationships. Detect
   cycles, unsupported layouts, incomplete discovery and missing targets.
2. Add a narrow injected history operation that keeps coordination held over
   impact discovery, complete capture/commit and the native action callback.
   Preserve layering: service imports no concrete adapter/SQL, and history
   imports no Bubble Tea. Reject foreign refs and unsupported deletion for the
   entire request before capture or archive side effects.
3. If history is enabled and capture or impact capability is unavailable, return
   an explicit preservation failure. There is no silent unprotected fallback.
   Disabled mode need not discover impact at all.

Gate A: `go test ./ocm/service/... ./ocm/sources/opencode/...` passes; tables
cover root→child→grandchild, siblings, independent forks, hidden subagents,
cycles, missing targets and unsupported scope. A foreign ref anywhere in the
batch yields zero capture, lock, archive and native calls. A fake source test
proves all orchestration works without SQLite or OpenCode.

## Increment B — order capture, commit and native deletion

For each requested target, preserving sequential batch semantics:

1. Honor caller cancellation, acquire the per-source coordinator before the
   snapshot, and discover the affected set under the approved consistency rule.
2. Capture **all** affected sessions, publishing complete contributions only.
   All required captures must commit before invoking that target's native delete.
   Earlier successful captures may remain if a later descendant fails; that is
   harmless retained observation, but does not authorize deletion. If the policy
   requires a group transaction, implement/test that instead.
3. Keep coordination across native CLI completion, preventing a collector from
   publishing an older observation after this capture. Apply the approved
   ownership/recovery policy if OCM dies while the child is still running;
   release only when that policy permits competing operations. An error/timeout
   return alone is not proof that every source-mutating subprocess has finished.
   Give foreground capture priority between bounded background units; never
   interrupt a publisher into committing partial state.
4. Invoke existing same-instance deletion with its own 30s context. Record
   capture and delete errors separately per requested target, continue independent
   valid targets under the approved partial-bulk policy, and retry idempotently.
5. Perform the existing single authoritative reload after attempts, including
   capture failures/partial success; preserve failed-reload presentation and
   caller-cancellation semantics. Do not invent an empty snapshot on failure.

Overlapping selected ancestor/descendant targets require the exact approved
policy: deduplicate capture scopes, and specify how an already-cascaded requested
ID affects native calls, Count and error reporting. Preserve selected-request
count semantics or document/test an explicitly resolved change. Do not count a
grandchild as a separately successful request merely because it disappeared.

Gate B: focused service/history tests pass with call-order traces proving
`scope → complete read → committed archive → native delete`. Inject failures at
impact discovery, descendant read, archive insert/commit, lock acquisition,
native timeout/failure and final reload. For any required capture failure the
affected native call count is zero. Failed deletion leaves committed history;
retry leaves exact totals unchanged. Partial results still reload exactly once.

## Increment C — exercise process races, crash recovery and UI messages

Use helper processes on disposable source/archive fixtures and barriers (no
timing sleeps as synchronization). Test these schedules explicitly:

| Schedule | Required result |
| --- | --- |
| Older collector is in a source read when capture requested | It finishes/releases or cancels safely; fresh capture follows before deletion; no late old overwrite. |
| Second process starts collection after capture commit, before native completion | It waits until delete ordering releases; absence afterward retains captured rows. |
| Process dies before archive commit | Last committed contribution intact; no deletion invoked. |
| Process dies after commit, before CLI | Archive retained; lock recovers; retry is idempotent. |
| Capture committed, deletion child alive on a barrier, OCM parent killed | A second collector/deleter obeys the approved surviving-child policy; parent lock release alone does not authorize unsafe work. |
| CLI may have deleted, process dies before reporting/reload | Next load is authoritative; archive remains; missing-source retry follows policy, never erases history. |
| New usage/descendant appears during capture/delete | Approved detection/limitation behavior, no claim of atomic protection. |

The live-child test is mandatory and must exercise real process lifetimes:

1. Commit capture in a temporary archive, then start a fake native deletion
   child that announces readiness and blocks on a test-controlled barrier.
2. Kill the OCM helper parent without graceful context cancellation; confirm
   parent exit and that the original child remains alive. Use an independent
   control channel so closing the parent's output pipes does not accidentally
   terminate the fake child or release its barrier.
3. While the child remains blocked, start a second OCM helper against the same
   source/archive. Run separate collector and deleter cases; record attempted
   snapshot/publication/native-action events.
4. Under full ordering, assert conflicting work cannot enter the protected
   interval until the original action completes. Under an explicitly approved
   narrower contract, assert exactly its permitted operations, source states,
   diagnostics and archive outcomes, backed by the step 2 source evidence.
5. Release the child, observe completion, and verify bounded recovery, retry and
   retained totals. Include success/failure and any source-mutating descendants
   required by the supported native lifecycle. The harness must clean up all
   helpers even if an assertion fails; no real OpenCode deletion is used.

If the chosen policy instead guarantees termination on parent death, prove that
termination precedes admission of competing work, and test the recovery path
with a surviving helper wherever survival remains possible. Do not count a test
that silently loses its child as evidence for a surviving-child contract.

Use fake CLI actions to mutate temporary source fixtures as the supported cascade
model dictates. Keep upstream version evidence distinct from the fake's behavior.
UI tests execute the returned Cmd and feed messages through Update. Display
capture failure, native failure and reload failure truthfully, clear busy state,
and preserve selection/rows according to the authoritative snapshot. No new
global input lock for background collection. Preserve existing confirmation
behavior and mock acknowledgement rules.

Gate C: `go test ./ocm/... ./cmd/ocm/...` and the mock equivalent pass. Two-process
tests fail when coordination is intentionally bypassed in the test harness,
demonstrating they exercise ordering rather than mere scheduling. Normal/mock
operation tests cover failure recovery, selected-overlap results and empty reload.
Report the live-child schedule and assertions on macOS and Linux; a goroutine
test or a crash after child exit cannot substitute for it. Do not claim retained
accounting corruption without separate evidence of deletion/intermediate-state
semantics.

## Increment D — regression proof and user documentation

Disabled tests inject panic-on-use impact/capture/coordinator/archive dependencies
and run single/bulk deletion, browsing/reload/resume, `--cost-period`, vacuum and
prune. Test no archive, existing archive, corrupt archive and unwritable archive;
no disabled history call or archive change is allowed. Keep `--sessions` filtering
and root/cost metrics behavior as before.

Document in `ocm/README.md`: opt-in pre-delete capture, failures block affected
deletion, independent bulk results/retry, archive preservation, external deletion
limits, activity-in-flight limits, parent-crash/unfinished-deletion recovery,
disabling gaps, and vacuum/prune separation.
Do not imply history is already navigable if step 5 has not landed.

Gate D: expanded command/source regression suites pass. Source identity/action
tests still prove V2 `--standalone`, custom/channel storage and same executable/
environment targeting. No real native actions or terminal apps used.

## Exit gate

Use Go 1.25.0, gofmt changed Go files, then run all from repository root:

```sh
go test ./ocm/... ./cmd/ocm/...
go test -tags mock ./ocm/... ./cmd/ocm/...
go vet ./...
go test -race ./...
make build
go test -tags mock ./...
go build -tags mock ./cmd/prq ./cmd/ocm
```

All exit 0. Measure capture latency for large targets and descendant trees against
step 2's budgets; record lock waiting separately from extraction and native
action. Review diff for unchanged disabled semantics and no unrelated maintenance
changes. Update roadmap, HISTORY, architecture and plan index with evidence.

Stop if deletion impact cannot be established, background data can publish out
of order, enabled failure falls through to deletion, or correctness requires
blocking OpenCode's writers. Resolve the contract rather than weakening capture.
Future OpenCode deletion changes require a cascade-matrix review; new deletion
entry points must use the same guarded service operation.
