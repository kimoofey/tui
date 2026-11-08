> **Historical reference — superseded 2026-11-08.** The current release contract
> is [plan 001](../001-ocm-history-contracts.md); execution starts at
> [plan 002](../002-ocm-history-storage.md), as listed in the [index](../README.md).
> Measurements and fixtures below remain evidence for their original experiments.
> Old open gates, approval requirements, proposed precision/coordination/resource
> contracts and “production out of scope” statements do not govern implementation.
> Do not resume this research checklist. The accepted V1 approximation is retained
> in plan 001; other conflicting policies are replaced there.

# Local database observation — 2026-11-06

The maintainer authorized locating and copying their local OpenCode database.
This resolves the missing-source-path obstacle. The approved measurement route
is one private real-data snapshot plus generated workloads for scale; the old
assumption that a roughly 2 GB real database must be supplied is superseded.
No downloaded database is required. Full performance and contract gates remain
open; this observation does not unlock production collection.

## Snapshot and environment

Source: `~/.local/share/opencode/opencode.db`, located using the repository's
documented storage path and checking local overrides. `OPENCODE_DB` and
`XDG_DATA_HOME` were unset. Discovery was explicitly requested by the maintainer;
ordinary tests still never discover user databases.

Python SQLite **3.43.2** opened the live source with `mode=ro` and used its online
backup API, in 256-page steps with a 30-second deadline. Only the private
destination had its journal mode changed to DELETE, then all backup connections
were closed before read-only observation. This produces a standalone snapshot;
it does not measure WAL-mode contention. Backup/finalization took **96.152 ms**.
`PRAGMA quick_check` on the snapshot returned `ok`.

The snapshot is private and temporary:
`/private/tmp/ocm-history-20261106-tj193pol/opencode.snapshot.db`.
Directory mode is **0700**, database mode **0600**. The snapshot, compiled test
binary, cache and auxiliary measurements are outside the repository. Temporary
files may be removed by the OS; this path is not a durable artifact or CI input.
No conversation content, native IDs, titles, directories, provider/model names,
or real usage amounts are included in this report.

Extraction used **Go 1.25.0**, module SQLite **3.53.1**, **darwin/arm64**.
The default Go build cache was sandbox-denied; an isolated temporary `GOCACHE`
resolved that failure without escalation.

## Dataset shape and extraction

| Observation | Result |
| --- | ---: |
| Snapshot bytes | 39,047,168 (37.24 MiB) |
| Legacy sessions / messages | 30 / 514 |
| Authoritative V2 sessions / messages | 32 / 601 |
| V2 assistant rows / extracted usage items | 460 / 460 |
| V2 events / fork sessions / orphan messages | 0 / 0 / 0 |
| Malformed V2 message JSON rows | 0 |
| V2 message JSON bytes, total / largest record | 7,259,309 / 2,641,210 |
| Largest session by message count | 179 |
| Largest session JSON bytes | 4,055,854 |

Schema probes and extraction succeeded with V2 authority despite legacy tables
being present. Every run returned `v2-assistant-compaction/1`, `complete=true`
and an empty diagnostics map. Completion describes the read scope; it is not
proof of complete historical spending. An independent SQL membership count also
found 460 eligible assistant rows. No independent real-data monetary oracle was
established; exact accounting assertions remain in the synthetic fixtures.

Recorded session version metadata ranges from 1.15.10 to 2.2.5 and includes
2.1.4. Such metadata is not proof of the currently installed executable version
or a supported-version range. This snapshot has neither fork sessions nor usage
events, so it cannot validate native fork/compaction/deletion behavior. Those
cases continue to require targeted fixtures and pinned native evidence.

## Repeated observations

| Run | Five extraction samples, ms | Min / median / max, ms |
| --- | --- | --- |
| One test process, repeated reads | 34.263 / 19.308 / 18.198 / 18.032 / 18.102 | 18.032 / 18.198 / 34.263 |
| Five fresh test processes | 22.236 / 21.019 / 21.792 / 20.096 / 20.587 | 20.096 / 21.019 / 22.236 |

Fresh-process peak RSS samples were **19,857,408 / 20,070,400 / 19,742,720 /
19,906,560 / 19,939,328 bytes**: min/median/max **18.83 / 18.98 / 19.14 MiB**.
These are whole-test-process high-water marks reported by `wait4` on macOS,
including the Go runtime and test machinery. They are not additional collector
live heap or memory measurements at synthetic scale. `/usr/bin/time -l` could
not report RSS because the sandbox denied `sysctl kern.clockrate`; `wait4`
provided the completed measurements without escalation.

All reads followed backup/shape inspection and used the OS file cache. Even
fresh-process samples are **warm I/O**, not cold-storage measurements. Extraction
timing excludes Go compilation and process startup. It includes snapshot/schema
detection, enumeration, SQL projection, JSON decoding and transaction completion;
it excludes grouping, archive writes and rendering.

The snapshot's SHA-256 was identical before and after the observations. Source
access was read-only; no native OpenCode executable, deletion, maintenance,
migration, checkpoint or schema/index change was invoked on the live source.

## Reproduce extraction

With the retained private snapshot, run from the repository root:

```sh
export OCM_HISTORY_PRIVATE=/private/tmp/ocm-history-20261106-tj193pol
export GOTOOLCHAIN=go1.25.0
export GOCACHE="$OCM_HISTORY_PRIVATE/go-build"
go test -v ./plans/ocm-history-research -run '^TestHistoryExplicitPath$' -count=5 -args -history-db "$OCM_HISTORY_PRIVATE/opencode.snapshot.db"
go test -c -o "$OCM_HISTORY_PRIVATE/history-research.test" ./plans/ocm-history-research
```

For fresh-process samples, invoke the compiled test five times with
`-test.v -test.run='^TestHistoryExplicitPath$' -test.count=1 -history-db PATH`.
Read `elapsed` from each test log. For RSS, launch each child with `os.fork` /
`os.execv`, collect its output through a pipe, and obtain `ru_maxrss` from
`os.wait4(child_pid, 0)`. On this macOS host that value is bytes. Auxiliary
per-process timing/RSS records remain in the private directory.

For a new snapshot, create a directory with `tempfile.mkdtemp` under
`/private/tmp` (mode 0700), exclusively create a 0600 destination file, open the
source URI using `mode=ro`, and call Python `Connection.backup` into a separate
writable destination connection. Finalize DELETE journal mode **on the copy
only**, close both connections with `contextlib.closing`, and verify the copy
before supplying its explicit path. Do not use an ordinary file copy of a live
WAL database or enable immutable mode on the live source.

## Remaining gate C work

Real-data availability and repeated extraction on the available layout are now
evidenced. The initial [synthetic measurements](results.md) covered only V1 with
fixed 1 KiB padding. A subsequent [V1/V2 scale continuation](scale-results.md) now
adds variable-size/3 MiB JSON, full prototype import, memory and controlled
contention evidence. Source-wide readers exceed the proposed bound at 100k rows;
bounded scheduling/staging and other contract/validation gates remain open.
Generated data is synthetic and does not clone private source content.

See that continuation for measured heap/RSS, unchanged full rewrite, huge-session
and full import timing, cancellation and a controlled WAL writer/checkpoint.
Still required: bounded collection/publication, changed/new-session refresh,
sustained writer impact, blocked-lock cancellation, write-phase UI contention,
end-to-end startup/input latency and accepted budgets. Identity, reconciliation,
ordering, process recovery and other product contracts remain unresolved.

## Native fork observation — 2026-11-07

The maintainer reported creating a V2 fork and authorized checking/updating the
private experimental copy. A new SQLite online backup was made from the live
source opened mode=ro, with DELETE journal finalization on the destination only.
Private snapshot: `/private/tmp/ocm-fork-20261107-folkjz5m/snapshot.db`;
directory/file permissions 0700/0600; quick_check returned ok. Earlier snapshots
and their measurements remain historical and were not overwritten.

This snapshot contains one V2 fork with an existing parent. Its six assistant
projections consist of five timestamped before fork creation and one at/after
creation, with no null timestamps. All five excluded projections have positive
recorded cost. There are no usage-recorded events for this fork.

An ephemeral Go test invoked the existing experiment extractor for the fork's
session scope. It returned one eligible item, complete=true and no diagnostics.
Its cost and UTC aggregate total matched an independent SQL sum of assistant
projections at/after the fork boundary; no pre-fork item appeared in extraction.
No identifiers, amounts or conversation content were printed or retained here.
The temporary test was removed after execution; the database remains private
outside the repository. Production code and persistent fixtures are unchanged.

Command: Go 1.25.0, isolated temporary GOCACHE,
`go test -v ./plans/ocm-history-research -run '^TestHistoryForkObservationTemporary$' -count=1`:
PASS, exit 0. This validates the expected V2 exclusion on one real fork, not a
supported-version range, event/compaction lifecycle, migration or deletion
recovery contract. The new snapshot supersedes the earlier no-fork sample for
future fork observations; earlier performance numbers retain their original
snapshot scope. No native OpenCode command or live-source write was invoked.
