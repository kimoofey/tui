//go:build !mock

package ocm

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/kimoofey/tui/internal/ui"
	"github.com/kimoofey/tui/ocm/db"
)

const cmdTimeout = 30 * time.Second

func bulkDeleteSessions(ids []string, dbPath string, rootOnly bool) tea.Cmd {
	return func() tea.Msg {
		var deleteErrs []string
		for _, id := range ids {
			ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
			if err := exec.CommandContext(ctx, "opencode", "session", "delete", id).Run(); err != nil {
				deleteErrs = append(deleteErrs, id+": "+err.Error())
			}
			cancel()
		}
		sessions, err := db.LoadSessions(dbPath, rootOnly)
		if err != nil {
			return bulkDeletedMsg{count: len(ids), err: err}
		}
		total, _ := db.SessionCount(dbPath)
		if len(deleteErrs) > 0 {
			return bulkDeletedMsg{
				count:    len(ids) - len(deleteErrs),
				sessions: sessions,
				newTotal: total,
				err:      errors.New(strings.Join(deleteErrs, "; ")),
			}
		}
		return bulkDeletedMsg{count: len(ids), sessions: sessions, newTotal: total}
	}
}

func deleteSession(sessionID, dbPath string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		cmd := exec.CommandContext(ctx, "opencode", "session", "delete", sessionID)
		if err := cmd.Run(); err != nil {
			return deletedMsg{sessionID: sessionID, err: err}
		}
		total, _ := db.SessionCount(dbPath)
		return deletedMsg{sessionID: sessionID, newTotal: total}
	}
}

func openSession(sessionID string) tea.Cmd {
	return func() tea.Msg {
		term := ui.ResolveTerminal("")
		shellCmd := "opencode -s " + sessionID

		var cmd *exec.Cmd
		switch term {
		case "terminal":
			script := `tell application "Terminal" to do script "` + shellCmd + `"`
			cmd = exec.Command("osascript",
				"-e", script,
				"-e", `tell application "Terminal" to activate`,
			)
		case "ghostty":
			cmd = exec.Command("open", "-na", "Ghostty",
				"--args", "--initial-command", shellCmd,
			)
		case "iterm2":
			script := `tell application "iTerm2" to create window with default profile command "` + shellCmd + `"`
			cmd = exec.Command("osascript", "-e", script)
		default:
			parts := strings.Fields(term)
			parts = append(parts, "opencode", "-s", sessionID)
			cmd = exec.Command(parts[0], parts[1:]...)
		}

		cmd.Env = os.Environ()
		if err := cmd.Start(); err != nil {
			return openedMsg{sessionID: sessionID, err: err}
		}
		return openedMsg{sessionID: sessionID}
	}
}

func pruneOrphans(dbPath string) tea.Cmd {
	return func() tea.Msg {
		count, bytes, err := db.PruneOrphans(dbPath)
		return prunedMsg{count: count, bytes: bytes, err: err}
	}
}

func vacuumDB(dbPath string, beforeTotal int64) tea.Cmd {
	return func() tea.Msg {
		used, total, err := db.VacuumDB(dbPath)
		return vacuumedMsg{dbUsed: used, dbTotal: total, beforeTotal: beforeTotal, err: err}
	}
}
