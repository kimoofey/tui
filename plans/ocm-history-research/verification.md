# Verification — 2026-11-05

Repository baseline: `0c710a69db06d4007b94d333516c34333024b1d8`.
All commands from the repository root using `GOTOOLCHAIN=go1.25.0`.
Changed Go files formatted with gofmt. Tests use temporary synthetic SQLite
fixtures. No real deletion, maintenance, credentials or graphical launches.

| Command | Result |
| --- | --- |
| `go test ./ocm/sources/opencode/...` before adding tests | PASS (exit 0) |
| `go test -v ./plans/ocm-history-research` | PASS (exit 0), four tests |
| `go test ./ocm/... ./cmd/ocm/...` | PASS (exit 0) |
| `go test -tags mock ./ocm/... ./cmd/ocm/...` | PASS (exit 0) |
| `go vet ./...` | PASS (exit 0) |
| `go test -race ./...` | PASS (exit 0), includes new research package |
| `make build` | PASS (exit 0) |
| `go test -tags mock ./...` | FAIL (exit 1), existing PRQ test compilation |
| `go build -tags mock ./cmd/prq ./cmd/ocm` | PASS (exit 0) |

Mock failure: `prq/filter_test.go` references `ghPRNode` and `ghAuthor`, undefined
under mock tags. `git diff --exit-code HEAD -- prq` exits 0, and targeted
`go test -tags mock ./prq` reproduces the same compilation failure using unchanged
baseline files. No PRQ fixes attempted. This remains a failed exit gate, requiring
resolution or explicit maintainer acceptance under the plan's verification rules.

The initial toolchain/cache writes were sandbox-denied; retries with approved
cache access succeeded. No auto-review rejection or remaining toolchain block.
Generated `bin/` outputs from make are ignored and must not be committed.
No Linux CI run or manual terminal smoke test is claimed. No extraction benchmark,
numeric acceptance budgets, real-data measurement, or final contract is claimed.

Content checks passed: relative Markdown file targets across all 10 plan/design
documents exist; plans contain no trailing whitespace. `git diff --exit-code HEAD
-- ocm cmd/ocm go.mod go.sum internal prq` exits 0: production code and dependencies
are unchanged. Existing `plans/` was untracked on entry and remains unstaged;
local `.zed/` updates remain ignored, so handoffs rely on the tracked-location
evidence pack rather than assuming those design records travel with git.

## Documentation follow-up — accepted V1 approximation

The maintainer accepted V1 fork-accounting inaccuracy while retaining V1 history
support. [Decisions](decisions.md) and steps 2–5 now define the recorded-usage
policy, persisted/displayed approximation, deferred accuracy work, and remaining
gates. This changes acceptance criteria, not the fixture's actual-call reference:
4 remains actual calls; 7 is the intended approximate recorded total.

During the preceding blocker review, `go test -v ./plans/ocm-history-research`
passed with Go 1.25.0 (cached result); `go test -tags mock ./prq` reproduced the
existing compilation failure. No full-suite rerun is claimed for this docs-only
revision. Production code, dependencies and `evidence_test.go` are unchanged.
The real-database, numerical-budget and native-deletion gates remain unmet.
The earlier local HISTORY checklist is superseded for V1 support by the decision
record; it must not reinstate exact V1 fork accounting as a prerequisite.

Documentation checks passed: relative Markdown file links and trailing whitespace
across all eight `plans/` Markdown files; current step-status consistency;
`git diff --exit-code HEAD -- ocm cmd/ocm go.mod go.sum internal prq` (exit 0).
`plans/` remains untracked and unstaged. No runtime code or fixture was edited.

## Extraction/reconciliation and synthetic continuation — 2026-11-05

Applied the accepted V1 decision without changing `evidence_test.go`. Added
isolated extraction, conservative reconciliation, candidate retention,
measurement fixtures and benchmarks; no production package or dependency changes.

All commands below used `GOTOOLCHAIN=go1.25.0`. Changed Go files were formatted.

| Command | Result |
| --- | --- |
| `go test ./ocm/sources/opencode/... ./plans/ocm-history-research` before edits | PASS, exit 0 |
| `go test -v ./plans/ocm-history-research` after final changes | PASS, exit 0; 16 top-level tests pass; explicit-path test SKIPs without supplied database |
| `go test ./ocm/... ./cmd/ocm/...` | PASS, exit 0 |
| `go test -tags mock ./ocm/... ./cmd/ocm/...` | PASS, exit 0 |
| `go vet ./...` | PASS, exit 0 |
| `go test -race ./...` after final changes | PASS, exit 0 |
| `make build` | PASS, exit 0 |
| `go test -tags mock ./...` after final changes | FAIL, exit 1; same unchanged PRQ `ghPRNode`/`ghAuthor` test compilation errors; research/OCM pass |
| `go build -tags mock ./cmd/prq ./cmd/ocm` | PASS, exit 0 |
| `go test ./plans/ocm-history-research -run '^$' -bench History -benchmem -benchtime=1x -count=5` after final extraction changes | PASS, exit 0; [raw results](synthetic-bench.txt) |
| `go test -v ./plans/ocm-history-research -run 'TestHistory(V2ReplacementLoss\|QueryPlan\|ExplicitPath)$'` | Initial replacement test asserted an overly specific unresolved reason; corrected to check preservation and unresolved state, since field diagnostics also apply. Final suite passes. Query plan passed; real-data test skipped. |

No real database path was supplied or opened, no native deletion/maintenance ran,
and no real source content or profiles were retained. The explicit-path harness
is available but was only exercised in its skipped state. Fixture/benchmark
source creation and candidate destination writes use temporary synthetic files.
No end-to-end UI, peak RSS, cold I/O, source-writer/checkpoint, Linux CI, native
surviving-child, or cross-process coordination result is claimed.

The numeric budgets in results.md are proposals, not maintainer approval. The
injected correction-authority map is a test input, not an implemented native
source authority provider. A–D exit gates remain open; production collection,
cleanup integration and History navigation remain gated.

Final content checks: relative Markdown targets across 13 plan/design documents,
plans whitespace, `gofmt -l` for research Go files, `git diff --check`, and
`git diff --exit-code HEAD -- ocm cmd/ocm go.mod go.sum internal prq` all pass.
`plans/` remains untracked/unstaged as on entry; local design updates remain
ignored. No commit or force-add was performed.

## Private real-data observation — 2026-11-06

The maintainer explicitly requested locating their local database and then
authorized the private-backup/extraction/measurement-plan continuation. Prior
entries above remain historical records. Source availability is now resolved;
the accepted route uses the available real sample plus generated workloads for
scale instead of requiring the previously assumed roughly 2 GB real database.
See [real-data evidence](real-data.md) for backup details, dataset shape, commands,
timing/RSS samples and remaining gate C work.

No Go source, production behavior, dependencies or test assertions changed in
this continuation. Only research/design documentation was updated. The source
was opened read-only for SQLite online backup. All further real-data observations
used the private snapshot; no native OpenCode command was invoked. Snapshot and
test binary remain outside the repository in a mode-0700 temporary directory.

| Check | Result |
| --- | --- |
| SQLite online backup and private-copy `PRAGMA quick_check` | PASS; 39,047,168-byte snapshot, `ok` |
| Go 1.25.0 `go test -v ./plans/ocm-history-research -run '^TestHistoryExplicitPath$' -count=5 -args -history-db SNAPSHOT` | PASS; five runs, 32 sessions / 460 usage items, no field diagnostics |
| Go 1.25.0 `go test -c -o PRIVATE/history-research.test ./plans/ocm-history-research` | PASS |
| Five fresh-process explicit-path test invocations | PASS; extraction 20.096–22.236 ms, process peak RSS 18.83–19.14 MiB via macOS `wait4` |
| Go 1.25.0 `go test -v ./plans/ocm-history-research` | PASS; 16 top-level fixture tests pass; explicit-path observation skips without its flag |
| Go 1.25.0 `go test -tags mock ./plans/ocm-history-research` | PASS |

Every Go invocation above used
`GOCACHE=/private/tmp/ocm-history-20261106-tj193pol/go-build` after the initial
default-cache attempt was denied by the sandbox. `/usr/bin/time -l` failed to
read `sysctl kern.clockrate`; RSS was obtained with `os.wait4` instead. An initial
backup attempt could not reopen the private copy read-only and was removed;
explicitly closing connections and finalizing DELETE journal mode on the
destination produced the validated standalone snapshot. No unresolved tool
block or approval escalation remains for these observations.

Full repository gates were not rerun for these documentation-only changes. The
previously recorded PRQ mock compilation failure remains unresolved; no waiver,
Linux CI, live-writer/WAL, native deletion/recovery, independent real-data monetary
oracle, or end-to-end UI result is claimed. Existing synthetic benchmarks were
not rerun or relabeled real data. Steps 3–5 remain gated.

Final content checks pass: relative Markdown file links and trailing whitespace
across 14 plan/design documents, `git diff --check`, research Go formatting, and
`git diff --exit-code HEAD -- ocm cmd/ocm go.mod go.sum internal prq`. The snapshot's
SHA-256 matches its pre-observation value; directory/file permissions remain
0700/0600. `plans/` remains untracked and unstaged, and `.zed/` remains ignored.
No commit was made and no private data or binary was added to the repository.

## Generated V1/V2 scale continuation — 2026-11-06

Implemented the authorized next research increment in `workload_test.go`,
`scale_test.go` and `contention_test.go`. The explicit real-data test's skip text
now describes its actual opt-in behavior. All changed Go files were formatted.
Production packages, dependencies and `evidence_test.go` remain unchanged.
All fixtures and writes in this increment are synthetic; no private snapshot or
live database was opened. See [scale results](scale-results.md) and
[raw output](scale-results.txt) for measurement scope, oracles and distributions.

Baseline, before adding the new helpers: Go 1.25.0
`go test ./ocm/sources/opencode/... ./plans/ocm-history-research` passed;
`go test -tags mock ./prq` reproduced the known `ghPRNode`/`ghAuthor` compilation
failure. The foundation diff against `0c710a6` for OCM/CLI/go.mod was empty.
No unrelated PRQ repair was included in the history increment.

All final commands ran from the repository root with `GOTOOLCHAIN=go1.25.0` and
`GOCACHE=/private/tmp/ocm-history-20261106-tj193pol/go-build`:

| Command | Result |
| --- | --- |
| `go test -v ./plans/ocm-history-research -run 'TestHistoryGenerated\|TestHistoryScale'` | PASS; small accounting/contention cases pass, large runner/worker skip by default |
| `go test -v ./plans/ocm-history-research -run '^TestHistoryScale$' -count=1 -timeout=10m -args -history-scale -history-samples=5` | PASS, exit 0, 248.160 s total; six shapes × five fresh workers; all extraction/persistence oracles pass |
| `go test ./ocm/... ./cmd/ocm/... ./plans/ocm-history-research` | PASS, exit 0 |
| `go test -tags mock ./ocm/... ./cmd/ocm/... ./plans/ocm-history-research` | PASS, exit 0 |
| `go vet ./...` | PASS, exit 0 |
| `go test -race ./...` | PASS, exit 0; includes new default fixture/contention tests |
| `make build` | PASS, exit 0 |
| `go test -tags mock ./...` | FAIL, exit 1; same baseline PRQ compilation errors, research/OCM packages pass |
| `go build -tags mock ./cmd/prq ./cmd/ocm` | PASS, exit 0 |
| `GOOS=linux GOARCH=amd64 go test -c -o PRIVATE/history-research-linux.test ./plans/ocm-history-research` | PASS, exit 0; compile only, not Linux runtime/CI evidence |

The first small headless browser check incorrectly required every navigation
frame's string to change; without a terminal color profile, cursor highlighting
can leave identical strings. The corrected check renders each updated model and
requires nonempty output, without claiming a physical terminal frame or input
latency measurement. The final small/scale/normal/mock/race runs above all use
that correction. The one-sample V2 smoke measurement was not used in the final
five-sample distributions.

Builds printed sandbox-denied module stat-cache write warnings but completed
with exit 0. No approval escalation or unresolved command block remains apart
from the independently failing PRQ mock compilation gate. Large scale measurements
ran separately before the parallel correctness/build checks, avoiding measurement
contention from those checks. Only synthetic metrics/test logs were retained;
temporary databases and test binaries were not added to the repository.

Correctness PASS is distinct from performance acceptance: complete 100k prototype
imports fit the proposed five-second target, but multi-session source-wide reads
exceed the proposed reader limit. Proposed targets are not maintainer-approved
contracts. Bounded scheduling/staging, write-phase UI and actual input/renderer
latency, sustained writer impact, blocked-lock cancellation, native semantics and
other gate C/D contracts remain open. Linux runtime CI, native deletion/recovery,
real terminal launches and a production collector were not exercised. No step
was marked DONE and steps 3–5 remain gated.

Final checks pass: relative Markdown file links/whitespace across 15 plan/design
documents, all 30 raw result records and phase/membership invariants, changed-Go
formatting, `git diff --check`, and unchanged tracked production/dependency/PRQ
files. `plans/` remains untracked/unstaged as on entry; local `.zed/` design
records remain ignored. No commit, binary, profile, private data or generated
database was added to git.
