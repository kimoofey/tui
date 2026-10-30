# ocm

Interactive TUI for browsing and managing your OpenCode sessions. Lists all sessions from the local OpenCode database with cost, directory, and timestamps — and lets you open, delete, and clean up directly from the table.

![ocm screenshot](../screenshots/assets/ocm.png)

## Requirements

OpenCode V1 or V2 installed and at least one session created. OCM asks the
`opencode` executable on your `PATH` for its storage locations using
`opencode debug paths`. The usual database path is:

```
~/.local/share/opencode/opencode.db
```

Custom `XDG_DATA_HOME`, `OPENCODE_DB`, and release-channel paths are discovered
automatically. Relative `OPENCODE_DB` overrides resolve under OpenCode's data
directory. In-memory databases and remote servers are not supported.

OCM reads `session_v2` whenever that table exists, including migrated V1
sessions. Older databases use `session`. Legacy rows left behind by V2 are not
merged into the list, so deleted sessions stay deleted. Untitled V2 sessions
appear as “Untitled session.” Browsing opens the database read-only and never
runs migrations.

Resume and delete use the same executable and database that OCM reads. With V2
custom storage, those actions use `--standalone` to avoid connecting to a
background service using another database.

## Install

```bash
brew install kimoofey/tap/ocm
```

Or build from source:

```bash
go install github.com/kimoofey/tui/cmd/ocm@latest
```

## Usage

![ocm demo](../screenshots/assets/ocm.gif)

```
ocm [OPTIONS]

  --sessions    Show only top-level sessions, hide subagent workflows
  --cost-period Show period cost in title bar: week|month|year
  --version     Print version and exit
```

## What the table shows

| Column    | Description                                                  |
| --------- | ------------------------------------------------------------ |
| (mark)    | `●` when selected for bulk action                            |
| Directory | Working directory of the session (home dir shortened to `~`) |
| Title     | Session title as set by OpenCode                             |
| Cost      | Accumulated LLM cost                                         |
| Created   | Session creation time                                        |
| Updated   | Last activity time                                           |

`--cost-period` uses the active schema's recorded costs. V2 period totals include
assistant messages and compaction usage, excluding history copied into forks.
The period total includes subagent usage even with `--sessions`. Deleting a
session removes its cost history from these totals.

## Keybindings

| Key               | Action                                                                  |
| ----------------- | ----------------------------------------------------------------------- |
| `↑/k` `↓/j`       | Navigate                                                                |
| `PgUp/b` `PgDn/f` | Page up / down                                                          |
| `Space`           | Mark / unmark row for bulk action                                       |
| `Enter`           | Open session in OpenCode                                                |
| `r`               | Reload sessions and stats from the database                             |
| `d`               | Delete session (or all marked) — press `d` again to confirm             |
| `v`               | Vacuum database — reclaims free pages (skipped if < 1 MB reclaimable)   |
| `p`               | Prune orphaned diff files — removes leftover files for deleted sessions |
| `esc`             | Clear marks / close help / cancel pending action                        |
| `?`               | Toggle full help                                                        |
| `q`               | Quit                                                                    |

## Database maintenance

**Vacuum** runs SQLite's `VACUUM` command to compact the database file and reclaim space freed by deleted sessions. Only offered when there is at least 1 MB to reclaim.

**Prune orphans** deletes `storage/session_diff` JSON files in OpenCode's data
directory whose session no longer exists in the active session table. Pruning
is available only for the default `opencode.db` in that directory. It is
disabled for custom and channel-specific database files because legacy diff
files do not identify which database owns them. Missing legacy diff directories
require no cleanup.
