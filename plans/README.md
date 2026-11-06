# OCM history implementation handoffs

Prepared on 2026-11-05 against `0c710a6`, using the improve skill. These are
implementation plans, not evidence that history has been implemented or that
all outstanding product decisions have been approved. The accepted V1 fork
approximation is recorded in [history decisions](ocm-history-research/decisions.md).

Start another agent with: **Read `AGENTS.md`, `ocm/AGENTS.md`, and the selected
plan completely. Execute only that plan, honor its prerequisites and stop
conditions, and report evidence for every exit gate.**

| Roadmap step | Handoff | Depends on | Status | Effort / risk |
| --- | --- | --- | --- | --- |
| 1 | Architecture foundation | — | DONE in existing roadmap | Delivered |
| 2 | [Validate extraction and settle contracts](001-ocm-history-contracts.md) | Foundation | IN PROGRESS (fixtures, real extraction and V1/V2 scale pipeline measurements delivered; reader budget/contracts/full validation open; [evidence](ocm-history-research/README.md)) | L / high accounting risk |
| 3 | [Opt-in archive and collection](002-ocm-history-storage.md) | Step 2 exit gate | TODO | L / high persistence risk |
| 4 | [Capture before cleanup](003-ocm-history-cleanup.md) | Step 3 exit gate | TODO | L / high deletion risk |
| 5 | [History view and archive-only startup](004-ocm-history-view.md) | Steps 3 and 4 exit gates | TODO | L / medium UI/startup risk |

Execute sequentially. Each row is one roadmap milestone; its plan divides the
work into reviewable increments. Do not ship navigation before persistence and
cleanup pass their gates. An isolated fixture UI experiment in step 2 does not
authorize production wiring. Status values: TODO, IN PROGRESS, BLOCKED (reason),
DONE (verification reference). Never mark a step done because its code compiles.

Step 2 continuation evidence: [extraction and synthetic results](ocm-history-research/results.md),
[native tracing and remaining contracts](ocm-history-research/native-trace.md).
V1 approximation is accepted; measured component timings do not approve the
remaining policies or waive real-data, process-recovery or verification gates.

2026-11-06 continuation: the maintainer authorized local database discovery and
a private SQLite backup. Repeated V2 extraction passes on the actual 37.24 MiB
snapshot; see [real-data evidence](ocm-history-research/real-data.md). The approved
measurement route uses this real sample plus generated workloads for scale,
replacing the assumed roughly 2 GB real-data prerequisite. Full performance,
native behavior, contracts and repository verification gates remain open.

The next increment adds [generated V1/V2 scale evidence](ocm-history-research/scale-results.md):
30 fresh-process runs with variable-size/3 MiB records, complete prototype
extraction/group/write, memory, cancellation, controlled WAL and headless browser
measurements. Import fits the proposed five-second target at 100k rows, but
source-wide multi-session reads exceed the proposed two-second reader limit.
Bounded collection/staging and remaining validation/contracts must be resolved
before step 3; measured component results do not approve those policies.

## Relationship to existing design

- [ROADMAP](../.zed/ocm/ROADMAP.md) owns milestone ordering and release policy.
- [HISTORY](../.zed/ocm/HISTORY.md) contains the original accounting proposals.
- [History decisions](ocm-history-research/decisions.md) records the current V1
  exception and follow-up recommendations; it supersedes older conflicting V1
  stop conditions in local design records. Keep this file with executor handoffs.
- [ARCHITECTURE](../.zed/ocm/ARCHITECTURE.md) owns foundation boundaries.

Those files exist locally but `.gitignore` ignores `.zed/`. These handoffs live
outside it and inline the essential requirements; `plans/` is currently untracked. Before handing work to
an agent in another checkout, provide those three documents or tracked copies;
do not assume editor-local documents travel with git. Their earlier V1-blocked
status is superseded by the decision record linked above. Do not force-add all of
`.zed/`. Each plan specifies which design records its executor must update.

## Guarantees throughout implementation

History is disabled by default. Only `history.enabled: true` in
`$XDG_CONFIG_HOME/ocm/config.yaml` (fallback `~/.config/ocm/config.yaml` when unset
or empty) enables it, read once at startup. No flag, environment override, or
in-app enable toggle. Malformed config and unknown fields fail clearly;
`--help` and `--version` remain config-independent.

Disabled mode performs **zero archive access, usage extraction, migration,
history locking, or pre-delete capture**, even with an existing corrupt or
unwritable archive. It keeps current browsing, selection, confirmations, reload,
deletion, `--cost-period`, source-error behavior, and no-session exit. Disabling
does not erase history. Enabled mode intentionally adds capture before deletion
in step 4 and archive-only access in step 5; those are explicit behavior changes.

Keep the source read-only; use OCM-owned SQLite for retained usage, with no
conversation content. UI/service cannot import the OpenCode adapter. Keep usage
extraction optional and separate from listing. Entry points own resources.
No Pi, other agents, daemon, dynamic plugins, billing-price reconstruction,
source schema migration, dependency upgrades, or unrelated PRQ changes.

## Research evidence and limits

Repository inspection covered CLI composition and ownership, real/mock command
paths, browser state/key handling, service loading/actions/capabilities,
OpenCode schema selection/period SQL/actions/fixtures, PRQ YAML conventions,
the three design documents, go.mod, Makefile, and Linux CI. This is a focused
history feasibility and integration study, not a repository-wide security audit.

Confirmed integration hazards:

| Evidence | Implication for the plans |
| --- | --- |
| `ocm/sources/opencode/sessions.go:openStorage` uses `mode=ro` and `SetMaxOpenConns(1)` | Collection needs its own handle; do not occupy the browser connection. |
| `sessionTableContext` re-probes and prefers `session_v2`, even empty | Preserve V2 authority and detect migration within a collection run. |
| `periodCost` V1 vs `v2PeriodCostContext` | V1 currently sums message costs without a role/session join; V2 has explicit assistant/fork/compaction rules. Do not silently normalize the legacy behavior. |
| `ocm/service/operations.go:Delete` | Whole-batch validation, sequential actions, per-call 30s timeout, one authoritative reload, partial failures must survive capture integration. |
| `ocm/sources/opencode/actions.go:runDelete` | CommandContext/WaitDelay do not establish child completion after parent death; step 2 must define ownership/recovery and step 4 must test a surviving child. Accounting loss from this gap is not established. |
| `ocm/model.go:Update` | Existing reload locks input; background history needs independent state/messages. |
| `cmd/ocm/main.go:runBrowser`, `composition.go:loadSource` | Empty startup currently exits and source-load failure closes the source; archive-only mode needs deliberate ownership changes. |
| `ocm/ops_mock.go`, `cmd/ocm/data_mock.go` | Real and mock paths differ; new async behavior needs tests in both. |

Primary references checked on 2026-11-05:

- [SQLite isolation](https://sqlite.org/isolation.html): source snapshot isolation
  does not coordinate an independent archive write or external CLI action.
- [SQLite WAL](https://sqlite.org/wal.html): long readers can delay checkpoint
  progress; benchmark snapshot duration and WAL growth as well as query time.
- [SQLite URI parameters](https://sqlite.org/uri.html): `mode=ro` is appropriate
  for a live source; `immutable=1` skips locking/change detection and is unsafe
  for a database another process can change.
- [OpenCode stats implementation](https://raw.githubusercontent.com/anomalyco/opencode/dev/packages/core/src/session/stats.ts)
  and [response schema](https://raw.githubusercontent.com/anomalyco/opencode/dev/packages/schema/src/session-stats.ts):
  current reference for message/event accounting and available summaries.
  These are mutable `dev` links, not a supported-version contract. Step 2 must
  pin actual upstream revisions and validate each supported local schema.

At initial preparation no real OpenCode database was inspected or benchmarked.
The 2026-11-06 private snapshot observation is recorded separately above; the
live source was opened read-only. Earlier HISTORY sample timings are feasibility
data only. Scale performance, actual cascade semantics by version, dependable change signals, durable identity,
timezone, final storage schema, and synchronization policy remain step 2 gates.
Plan review added explicit gates for native deletion surviving OCM death,
resolution of ambiguous lineage without requiring perfect automatic recognition,
and authority to revise accounting downward after corrections versus record loss.
The plans prescribe no subprocess recovery mechanism; a narrower safe ordering
contract requires evidence and an explicit decision before implementation.
No runtime tests were run for this documentation-only change. The local default
toolchain reports Go 1.26.4; executors must use go.mod's Go 1.25.0 rather than
assuming the host default satisfies the requested verification baseline.

## Resume step 2

Keep V1 and V2 history support. V1 fork copies may overstate recorded usage;
this is an accepted, visible limitation, not a reason to stop extraction research.
Preserve exact aggregation/idempotency under that policy and all other validation.
Implement the extraction oracle, conservative reconciliation experiment, native
deletion trace and synthetic benchmarks. The real-database path is now resolved;
remaining measurements and contracts still gate step 2 completion. Steps 3–5
remain gated.

PRQ mock build-constraint mismatch has been resolved (fcf4525). The
`//go:build !mock` constraint was added to `prq/filter_test.go` and
`prq/wait_time_test.go`, and mock build infrastructure was created in
`cmd/prq/deps*.go` with corresponding tests. Both normal and mock builds
pass (`go test -tags mock ./prq/... ./cmd/prq/...`).

## Verification common to every code milestone

Use Go 1.25.0 (`GOTOOLCHAIN=go1.25.0` with that toolchain available), temporary
fixtures, fake executables/sources/launchers, and injected clocks. No live
credentials, real deletion/maintenance, or graphical terminal launches.

Run each command from the repository root; success means exit 0 and all tests
pass. Format only changed Go files with gofmt. Capture command/toolchain/results
in the implementation report; a blocked command is a failed gate until resolved
or explicitly accepted by the maintainer.

```sh
go test ./ocm/... ./cmd/ocm/...
go test -tags mock ./ocm/... ./cmd/ocm/...
go vet ./...
go test -race ./...
make build
go test -tags mock ./...
go build -tags mock ./cmd/prq ./cmd/ocm
```

All history milestones affect wiring or shared contracts, so include mock gates.
Linux CI must pass; terminal launch behavior is command-tested only unless a
manual macOS check is explicitly recorded. No binaries, profiles, raw user
records, or real database snapshots in commits. Compare baseline failures before
changing code; do not repair unrelated failures in a history milestone.

## Deliberately rejected expansions

- Implement everything in one PR: obscures the accounting and deletion gates.
- Use session lifetime cost/latest model for daily history: loses days/models.
- Make the existing source period total use the archive: changes established flow.
- Use a SQLite archive transaction as the only collection/delete lock: cannot
  order source reads made before that transaction.
- Treat timestamp-only incremental scans as complete: no validated signal yet.
- Guarantee recovery of usage never observed, or writes in flight at deletion:
  unavailable evidence and no cross-system atomic transaction.

After each milestone, update its row and the roadmap/design records with actual
decisions and evidence. Refresh downstream plans against those decisions before
dispatching their executors. Recommendations below never override the approved
step 2 contract.
