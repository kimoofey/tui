> **Historical reference — superseded 2026-11-08.** The current release contract
> is [plan 001](../001-ocm-history-contracts.md); execution starts at
> [plan 002](../002-ocm-history-storage.md), as listed in the [index](../README.md).
> Measurements and fixtures below remain evidence for their original experiments.
> Old open gates, approval requirements, proposed precision/coordination/resource
> contracts and “production out of scope” statements do not govern implementation.
> Do not resume this research checklist. The accepted V1 approximation is retained
> in plan 001; other conflicting policies are replaced there.

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

## Bounded collection continuation — 2026-11-07

Added isolated `stage.go`/`stage_test.go`, a streaming sink in the research
extractor, session-scoped V2 fork-boundary validation, and a fresh-worker bounded
measurement route. All changed Go files are gofmt-clean. No production, PRQ,
dependency or native-action implementation changed. The incoming edits in
`real-data.md` were preserved. Results and remaining limits are recorded in
[bounded evidence](bounded-results.md); the [proposal](proposal.md) consolidates
contracts for review without asserting acceptance or native validation.

Using Go 1.25.0, darwin/arm64, from the repository root:

| Command | Result |
| --- | --- |
| `go test ./plans/ocm-history-research` | PASS |
| `go test ./plans/ocm-history-research -run 'TestHistoryStage\|TestHistoryBoundedRouting'` | PASS; includes process-exit journal recovery |
| `go test ./ocm/... ./cmd/ocm/...` | PASS |
| `go vet ./...` | PASS |
| `go test -race ./...` | PASS; all packages, new safety/crash tests included |
| `make build` | PASS |
| `go test -tags mock ./...` | PASS; includes OCM, research, PRQ and CLI packages |
| `go build -tags mock ./cmd/prq ./cmd/ocm` | PASS |
| Opt-in bounded scale, six shapes × three fresh processes | PASS, 58.705 s; correctness passes, huge-session performance target fails safely |

The first repository mock run using the default Go cache failed setup because
sandbox access to a cache entry was denied. Repeating with
`GOCACHE=/private/tmp/ocm-history-20261106-tj193pol/go-build` passed. All final
repository gates and final scale used that writable cache and
`GOTOOLCHAIN=go1.25.0`. Builds printed nonfatal module stat-cache write denials;
exit status was zero. No approval escalation was requested.

The PRQ build constraints were already fixed on entry (`filter_test.go` and
`wait_time_test.go` carry `!mock`); no further PRQ change was necessary. This
supersedes the earlier repository-wide mock failure, not the historical record
of that failure. Required mock OCM checks are covered by the passing full mock
suite. No test needs live credentials or an installed native terminal application.

Linux runtime/CI remains OPEN: local `docker version` cannot connect to
`/Users/kimoofey/.docker/run/docker.sock`; no Docker daemon is running. No Linux
runtime or new remote CI run was obtained, and no branch was pushed. Existing
Ubuntu workflow configuration and Linux cross-compilation are not runtime proof.
Native correction/migration/compaction/deletion evidence, archive-write contention,
actual input/renderer latency, sustained writes, blocked-query cancellation and
production publication/coordination remain open. Step 2 is still IN PROGRESS.

Final Linux compile check passed:
`GOOS=linux GOARCH=amd64 go test -c -o /private/tmp/ocm-history-bounded-linux.test ./plans/ocm-history-research`
with the same Go toolchain/cache. This is compile-only evidence; the binary was
not executed. Final `git diff --check`, changed-Go formatting, local links in
updated research documents and all 18 raw measurement records pass validation.
No source snapshot, generated database, binary or profile was added to git.

## Docker Linux runtime verification — 2026-11-07

Docker is running. A sandboxed socket probe returned permission denied; an
approved Docker socket access succeeded (server 29.9.2). This supersedes the
earlier Docker-unavailable observation. No automatic approval rejection occurred.

Executed the current workspace with the repository mounted read-only at
`/workspace` in `golang:1.25.0` (image manifest digest
`sha256:5502b0e56fca23feba76dbc5387ba59c593c02ccc2f0f7355871ea9a0852cebe`).
Environment: `GOTOOLCHAIN=local`, Go 1.25.0 **linux/arm64**, Linux aarch64.
The disposable container downloaded dependencies; source and dependencies in the
host workspace were not modified. Build outputs were kept in container `/tmp`.
The complete container invocation exited **0**. [Raw output](linux-verification.txt).

| Linux command | Result |
| --- | --- |
| `go vet ./...` | PASS |
| `go test -race ./...` | PASS, all packages including research subprocess recovery tests |
| `make build BIN=/tmp/ocm-step2-bin` | PASS; same Makefile targets with temporary output location |
| `go test -tags mock ./...` | PASS, all packages including PRQ and research |
| `go build -tags mock ./cmd/prq ./cmd/ocm` | PASS |

Reproduce from the repository root (requires Docker daemon access):

```sh
docker run --rm \
  --mount "type=bind,source=$PWD,target=/workspace,readonly" \
  --workdir /workspace --env GOTOOLCHAIN=local golang:1.25.0 sh -euxc '
  go version
  uname -sm
  go vet ./...
  go test -race ./...
  make build BIN=/tmp/ocm-step2-bin
  go test -tags mock ./...
  go build -tags mock ./cmd/prq ./cmd/ocm
  '
```

This closes the missing **local Linux runtime** evidence. It does not claim
hosted Ubuntu/amd64 CI passed for the current uncommitted changes, or that the
opt-in scale measurements were repeated on Linux. The Docker image supplies
neither live credentials nor native macOS applications. No native source was
opened, discovered or deleted. No production code was changed. See the current
[Step 2 closure review](step2-status.md) for remaining work.

## Huge-session profiling/optimization — 2026-11-07

Baseline `831e474`, clean workspace before this increment. Research-only changes:
explicit unordered session queries, one multi-field raw-JSON projection, typed
accounting envelope, bounded cancellable stage insert batches, opt-in profiling,
and sampled heap/RSS in the bounded runner. Added numeric-type/membership and
later-batch failure/duplicate/cancellation checks. The process-exit test now
interrupts after a batch reaches SQLite. No production/dependency/PRQ changes.

Final commands use Go 1.25.0. Local macOS commands set
`GOCACHE=/private/tmp/ocm-history-20261106-tj193pol/go-build` and
`GOTOOLCHAIN=go1.25.0`:

| Command | Result |
| --- | --- |
| `go test ./plans/ocm-history-research` | PASS |
| `go test ./ocm/... ./cmd/ocm/...` | PASS |
| `go vet ./...` | PASS |
| `go test -race ./...` | PASS; all packages |
| `make build` | PASS; nonfatal sandbox module stat-cache write warnings |
| `go test -tags mock ./...` | PASS; all packages |
| `go build -tags mock ./cmd/prq ./cmd/ocm` | PASS; nonfatal stat-cache warning |
| Opt-in profile runner, V1/V2 huge | PASS; diagnostic CPU runs, not deadline acceptance |
| Final opt-in bounded scale, six shapes × five fresh processes | PASS, 46.030 s; 10/10 huge stages complete below 2 s, zero diagnostic retries |

Linux final source verification also passes all five gates (vet, repository race,
make build with container `/tmp` output, repository mock tests and mock builds),
exit 0. Go 1.25.0 linux/arm64 in the same pinned `golang:1.25.0` image as the prior
run. Workspace and host module ZIP seed are mounted read-only; Linux build/module
caches are writable temporary directories under `/private/tmp/ocm-stage-linux-*`.
[Raw Linux output](optimized-linux-verification.txt). No hosted Ubuntu/amd64 CI
run is claimed. No live credentials, native OpenCode calls or terminal launches.
Final scale ran only after local and Linux checks completed, avoiding concurrent
build/test contention. Exploratory intermediate scale output was replaced by the
final 30 samples; the historical pre-optimization results remain intact.

See [optimization report](optimized-collection.md), [raw profiles/query plans](stage-profile-results.txt)
and [raw bounded results](optimized-bounded-results.txt). All changed Go is
gofmt-clean. Binary profiles and generated databases stay outside git. Native,
contention/publication and approved-contract gates remain open; improved fixture
performance alone does not complete Step 2.

## Gate C pipeline / UI / resource continuation — 2026-11-07

Research against `252b283` plus this increment; all Go changes are confined to
`plans/ocm-history-research/`. The [report](gatec-measurements.md) documents
scope, reproduction, final measurement tables and limitations. No production,
dependency, native executable or private-source changes.

Final local gates use `GOTOOLCHAIN=go1.25.0` and
`GOCACHE=/private/tmp/ocm-gatec-cache`. The ordinary shared build cache was
sandbox-restricted, so the writable temporary cache was used instead. All pass:
`go vet ./...`, `go test -race ./...`, `make build`,
`go test -tags mock ./...`, `go build -tags mock ./cmd/prq ./cmd/ocm`.
[Raw macOS output](gatec-macos-verification.txt). Builds emitted nonfatal module
stat-cache permission warnings; both build invocations exited 0. Default focused
research tests also pass. Changed Go files are gofmt-formatted.

The same five repository gates pass in a disposable Docker `golang:1.25.0`
container, Go 1.25.0 linux/arm64, with source mounted read-only and build outputs
in container `/tmp`. The Docker socket required an approved sandbox escalation;
no automatic approval rejection occurred. The container used `--network=none`,
`GOTOOLCHAIN=local` and `GOPROXY=file:///seed`, with the host module-download cache
mounted read-only as `/seed`. No network dependency fetch or host source mutation
was required. [Raw Linux output](gatec-linux-verification.txt). Both complete
verification invocations exited 0. Linux runtime checks include the actual
program/input/renderer default test and SQLite quota/cancellation tests.

The full-pipeline run occurs after an initial macOS/Linux verification, with no
other build/test/profiling command running concurrently. Final gates were repeated
after the capacity-cardinality extension and explicit TrueColor assertion; the
separate colored-UI run follows those gates without concurrent verification. Six shapes × five fresh worker
processes; [raw results](gatec-results.txt). The separate near-capacity probe runs
shared-label and distinct-model shapes five times each; [raw capacity results](gatec-capacity-results.txt). Its longer diagnostic
deadline measures capacity only, not the 2 s exceptional-reader acceptance.
Generation warms filesystem caches; fresh processes are not cold-I/O evidence.
The [colored-UI output](gatec-colored-ui-results.txt) separately records TrueColor
RGB bytes observed in baseline and concurrent renderer output. Earlier full-pipeline
UI data used the non-terminal sink's no-color profile and is labeled accordingly.
The full-pipeline artifact contains 30 workers (256.264 s), colored-UI artifact
30 workers (86.607 s), and capacity artifact ten observations in five repetitions
(91.596 s); all opt-in invocations exited 0. Scale measurements remain macOS-only. No hosted Ubuntu/amd64 CI result is claimed;
no branch push or workflow dispatch was performed. Step 2 remains IN PROGRESS.
