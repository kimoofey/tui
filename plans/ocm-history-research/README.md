# Step 2 evidence pack

Started 2026-11-05 against `0c710a69db06d4007b94d333516c34333024b1d8`.
Status: **IN PROGRESS; V1 fork inaccuracy accepted; steps 3–5 remain gated.**
This pack contains extraction/reconciliation fixtures and synthetic measurements
for increments A–C. It is not a collector, approved history
contract, or claim that the milestone's exit gates pass. The
[decision record](decisions.md) resolves the V1 support blocker; remaining
contracts and measurements are unfinished.

2026-11-06: maintainer-authorized discovery and a private SQLite backup now
provide [real-data evidence](real-data.md). Repeated extraction passes on the
available 37.24 MiB V2 snapshot. The approved route is this sample plus generated
workloads for scale; a roughly 2 GB real database is no longer a prerequisite.
Full performance, native behavior and contract gates remain open.

The [scale continuation](scale-results.md) now covers variable-size synthetic
V1/V2 records, including 3 MiB JSON, across 30 fresh processes. Complete prototype
imports, memory, cancellation, controlled writer/checkpoint and headless browser
results are recorded. Full-source 100k multi-session reads miss the proposed
reader budget; bounded scheduling/staging and remaining contracts still gate C/D.

## Reproduce

From the repository root:

```sh
GOTOOLCHAIN=go1.25.0 go test ./ocm/sources/opencode/...
GOTOOLCHAIN=go1.25.0 go test -v ./plans/ocm-history-research
GOTOOLCHAIN=go1.25.0 go test -v ./plans/ocm-history-research -run '^TestHistoryScale$' -count=1 -timeout=10m -args -history-scale -history-samples=5
```

The default test suite creates temporary on-disk SQLite fixtures and separate
read-only readers. No native OpenCode command runs. No user database is discovered,
opened, copied, or modified by default. An optional explicit-path read-only test
is documented in [results](results.md); it skips unless the operator supplies a
path. Fixture generation is inline in the test files; all IDs, directory labels,
and JSON used by ordinary tests/benchmarks are synthetic.
Both normal and mock repository test discovery include this package. Production
does not import it. No dependencies or runtime files were changed.

Environment measured by `TestHistoryEnvironment`: Go **1.25.0**, SQLite
**3.53.1** (module driver), **darwin/arm64**. Workspace filesystem reports APFS
volume `/dev/disk3s5`; physical medium and cold-I/O behavior were not measured.
Initial default toolchain was 1.26.4; it was not used for verification. The
required toolchain was downloaded after an initial sandbox cache-write denial.

## Evidence classification

| Claim | Class | Evidence / limitation |
| --- | --- | --- |
| Foundation anchors match | Repository-verified | HEAD equals plan baseline; requested `git diff --stat 0c710a6..HEAD -- ocm cmd/ocm go.mod` is empty. |
| V2 selection, including empty V2, is authoritative | Repository-verified | Existing migration/schema tests; production remains unchanged. |
| V2 period oracle is 6.5 / 3 / 0 | Repository-verified + synthetic extraction | Existing test passes; new independent extraction and aggregate fixture also checks those windows. |
| V1 fork approximation accepted | Maintainer decision, 2026-11-05 | Keep V1 support; include recorded assistant usage and persist/display approximation. Exact fork accounting deferred; see [decisions](decisions.md). |
| V1 messages may contain copied spend | Upstream-verified + synthetic experiment | Pinned fork implementation and `TestHistoryV1ForkEvidence`, below. |
| Session timestamps alone cannot certify freshness | Upstream-verified + synthetic experiment | V1 message conflict updates set data; fixture changes cost 3→1 while session timestamp remains 3000. Not a native integration test. |
| One read transaction avoids mixed committed states | Synthetic experiment | `TestHistorySnapshotAcrossWriterAndMigration`; deterministic WAL writer interleaving, no timing sleeps. |
| Conservative publication preserves ambiguous scopes | Synthetic experiment | Loss/replacement/migration/interruption keeps old accounting; injected correction authority permits decreases/zero. Native correction authority remains unresolved. |
| Real database schema/performance | Private snapshot observation, 2026-11-06 | V2 extraction, repeated warm/fresh-process timings and process RSS recorded in [real-data evidence](real-data.md); no live contention or full performance approval. |
| V1/V2 scale pipeline, memory and controlled contention | Synthetic fresh-process experiment | [Scale results](scale-results.md): import/cancellation components fit proposals; source-wide reader bound fails. Headless read-phase UI and a single WAL writer are measured, not full UI/native recovery. |
| UTC, background triggers, identity resolution, retained schema | Proposals only | These choices remain proposals; the separate V1 fork exception is accepted. |

## Pinned upstream candidates

Resolved using:

```sh
git ls-remote https://github.com/anomalyco/opencode.git refs/tags/v1.2.27 refs/tags/v2.1.4
```

| Candidate | Resolved commit | Inspected implementation |
| --- | --- | --- |
| V1 v1.2.27 | `4ee426ba549131c4903a71dfb6259200467aca81` | [message schema](https://raw.githubusercontent.com/anomalyco/opencode/4ee426ba549131c4903a71dfb6259200467aca81/packages/opencode/src/session/message-v2.ts), [fork/update/delete](https://raw.githubusercontent.com/anomalyco/opencode/4ee426ba549131c4903a71dfb6259200467aca81/packages/opencode/src/session/index.ts) |
| V2 v2.1.4 | `68bc26b7da1990ce10ffecfbc1e09eb13aa5d824` | [stats](https://raw.githubusercontent.com/anomalyco/opencode/68bc26b7da1990ce10ffecfbc1e09eb13aa5d824/packages/core/src/session/stats.ts), [projection schema](https://raw.githubusercontent.com/anomalyco/opencode/68bc26b7da1990ce10ffecfbc1e09eb13aa5d824/packages/core/src/session/sql.ts) |

These are candidate evidence points, **not a supported-version range**. Existing
browser schema compatibility does not establish history accounting compatibility.
Published-tag contents were inspected; commit IDs above pin future reproduction.

V1 assistant fields include cost, top-level providerID/modelID, optional variant,
and input/output/reasoning/cache token fields. Forking clones message information
under new IDs. Message updates can change data independently of session timestamps.
Deletion recursively visits children, then removes the parent; do not infer one
atomic transaction for the whole tree from that implementation.

V2 stats select nested model references and assistant usage, exclude pre-fork
copies, and separately add compaction usage events. Its projection schema has
session and time indexes. This is evidence for the existing period query, not
proof of source completeness or deletion recovery. Native V2 CLI deletion,
event/projector cleanup, migration, compaction/removal, and surviving-child
semantics still need pinned end-to-end tracing before enabled deletion is designed.

## Fixture shape and expected results

`legacySchema` defines `session` with the browser's seven required columns and
`message(id, session_id, time_created, time_updated, data)`. Indexes: primary-key
autoindexes plus `message_session_id_idx`. It deliberately does not represent a
full upstream migration, cascade schema, or supported native installation.

| Test | Shape / operation | Independent expectation |
| --- | --- | --- |
| V1 fork characterization | Two sessions; original cost 3, clone of cost 3, new call cost 1; two provider/model pairs | Actual calls cost **4**; unchanged legacy period API returns **7**. The separate new extraction test checks **7 approximate**, p/m=6, q/n=1. Original evidence test unchanged. |
| Old correction | One session/message; update cost 3→1 only in message data | Period changes **3→1**, session.time_updated stays **3000**. |
| Snapshot | One session, WAL; writer deletes session and creates incompatible V2 while reader transaction is open | Reader still sees **1** legacy row and **0** V2 tables; next snapshot sees **1** V2 table. Read-only handle rejects INSERT. |

The fork fixture demonstrates the accepted difference between recorded V1 usage
and actual calls. Keep both expectations; do not rewrite the actual-call reference
to suggest the duplicated spend is accurate. A pre-session timestamp is a possible
diagnostic, not an approved exclusion rule. Imports, timestamp edits, and other
supported V1 versions have not been validated. Never repair `--cost-period` to
force it to match history. History still needs assistant/session filtering and
field diagnostics, so the existing period query is not its extractor.

Minimum extraction evidence needed next: native session/usage IDs; authoritative
schema; per-usage timestamp, cost presence/type, provider/model/variant presence;
V2 fork provenance (exact V1 fork provenance deferred); token semantics; authoritative compaction usage source. Probe
required tables/columns inside the extraction snapshot. Missing usage tables or
columns must produce unavailable/unsupported, not complete zero. Unknown schemas
must not fall back to frozen V1. Malformed/missing fields require diagnostics;
their exact publication policy is still blocked. A migration after snapshot
opening belongs to a later snapshot, but archive publication also needs an
approved migration/lineage validation and coordination rule.

## Gates and next work

| Gate | Status | Outstanding evidence |
| --- | --- | --- |
| A | Partial | Pins and candidate shape matrix recorded; CLI/core/projector/bus/migration tracing advanced in [native trace](native-trace.md). Full supported-version/compaction validation remains open. |
| B | Experiment implemented; contract gate open | Independent day/model/token/quality oracles, diagnostics, session scopes, interruption, writer/migration, calendar and conservative retention tests pass. Native correction authority, lineage mapping and final completeness contract remain unresolved. |
| C | Expanded scale measurements delivered; exit open | [Scale results](scale-results.md) add full prototype import/unchanged rewrite, heap/RSS, cancellation, controlled WAL and headless UI. Source-wide reader target fails; bounded scheduling/staging, write-phase/real UI, sustained contention and accepted budgets remain open. |
| D | Open | V1 exception settled in [decisions](decisions.md); remaining identity, reconciliation, deletion, signature/schema/path and budget contracts need resolution. Older local HISTORY checklist is superseded for V1 support only. |

Continue step 2 with the accepted V1 recorded-usage rule and existing V2
accounting checks. Exact V1 fork accounting is deferred. Implement the extraction
experiment and independent policy oracle, trace deletion and record-loss semantics,
and measure synthetic workloads before choosing budgets. Do not implement a
production collector in this research milestone. Source availability is resolved
by the authorized private snapshot; ordinary harness runs still require an
explicit path and must not discover user databases automatically.
Synthetic numbers are explicitly separated from real-data evidence; proposed
budgets have not been approved. The current whole-observation candidate writer
is a capacity experiment, not the per-session transactional collector.
See [decisions](decisions.md) for the sequence and remaining gates.

## Verification

See [verification.md](verification.md) for command results and the existing PRQ
mock-test failure. Linux CI and manual macOS terminal execution were not run.
No source data, generated binaries, or profiles are included in this pack.

## Bounded collection continuation — 2026-11-07

[Bounded collection evidence](bounded-results.md) now measures separate session
snapshots and atomic disk staging across 18 fresh generated workers. Multi-session
100k reads fit the proposed per-reader bound; huge-session staging exhausts the
2 s deadline and safely defers. Completion/liveness and other gate C evidence
remain open. Review the consolidated [implementation contract proposal](proposal.md)
before freezing interfaces or starting step 3. It is a proposal, not approval.

2026-11-07 closure review: Docker Linux/arm64 runtime vet, race, normal builds,
repository mock tests and mock builds now pass. Hosted CI for current changes
remains unverified. The [current closure checklist](step2-status.md) separates resolved
items from huge-session completion, remaining performance/native evidence,
actionable recovery and final contract decisions. Step 2 remains IN PROGRESS.
