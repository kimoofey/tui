# tui

Small TUI tools for personal workflows. Built with Bubble Tea v2, Lipgloss v2, and a Nord color palette.

## Tools

| Tool                   | Description                                                                                                                                            |
| ---------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------ |
| [prq](./prq/README.md) | Interactive GitHub PR review queue. It surfaces PRs waiting for your review. These PRs come from direct assignments, team requests, and watched repos. |
| [ocm](./ocm/README.md) | OpenCode session manager. You can browse, open, and clean up your OpenCode sessions from the terminal.                                                 |

## Install

```bash
# Both tools via Homebrew
brew install kimoofey/tap/prq
brew install kimoofey/tap/ocm

# Or build from source
make build
make install
```

## Development

```bash
make build   # build both binaries
make lint    # run golangci-lint
```

Requires Go 1.25+, macOS, and [`gh`](https://cli.github.com/) (for prq). Both tools use `osascript` for terminal launch and are macOS-only.
