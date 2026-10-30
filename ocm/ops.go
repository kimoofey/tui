//go:build !mock

package ocm

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/kimoofey/tui/internal/ui"
	"github.com/kimoofey/tui/ocm/db"
)

const cmdTimeout = 30 * time.Second

func bulkDeleteSessions(ids []string, store *db.Store, rootOnly bool, costPeriod CostPeriod) tea.Cmd {
	return func() tea.Msg {
		var deleteErrs []string
		for _, id := range ids {
			ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
			if err := runDelete(ctx, store.Storage(), id); err != nil {
				deleteErrs = append(deleteErrs, id+": "+err.Error())
			}
			cancel()
		}
		data := readData(store, rootOnly, costPeriod)
		var deleteErr error
		if len(deleteErrs) != 0 {
			deleteErr = errors.New(strings.Join(deleteErrs, "; "))
		}
		return bulkDeletedMsg{count: len(ids) - len(deleteErrs), data: &data, err: errors.Join(deleteErr, data.err)}
	}
}

func deleteSession(sessionID string, store *db.Store, rootOnly bool, costPeriod CostPeriod) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		err := runDelete(ctx, store.Storage(), sessionID)
		data := readData(store, rootOnly, costPeriod)
		return deletedMsg{sessionID: sessionID, data: &data, err: errors.Join(err, data.err)}
	}
}

func runDelete(ctx context.Context, storage db.StorageContext, id string) error {
	args := sessionArgs(storage, "session", "delete", id)
	cmd := exec.CommandContext(ctx, storage.Executable, args...)
	cmd.WaitDelay = 250 * time.Millisecond
	cmd.Env = storageEnvironment(storage)
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return fmt.Errorf("opencode session delete: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func sessionArgs(storage db.StorageContext, args ...string) []string {
	if storage.V2CLI {
		args = append(args, "--standalone")
	}
	return args
}

func storageEnvironment(storage db.StorageContext) []string {
	env := make([]string, 0, len(os.Environ())+2)
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "OPENCODE_DB=") && !strings.HasPrefix(value, "XDG_DATA_HOME=") {
			env = append(env, value)
		}
	}
	return append(env, "OPENCODE_DB="+storage.DBPath, "XDG_DATA_HOME="+filepath.Dir(storage.DataDir))
}

// Terminal apps may have a different environment from OCM. Forward only path
// settings in the shell command; provider credentials stay out of arguments.
func terminalSessionArgs(storage db.StorageContext, id string) []string {
	args := []string{"/usr/bin/env"}
	var settings []string
	for _, key := range []string{"XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME", "OPENCODE_CONFIG", "OPENCODE_CONFIG_DIR"} {
		if value, ok := os.LookupEnv(key); ok {
			settings = append(settings, key+"="+value)
		} else {
			args = append(args, "-u", key)
		}
	}
	args = append(args, settings...)
	args = append(args, "OPENCODE_DB="+storage.DBPath, "XDG_DATA_HOME="+filepath.Dir(storage.DataDir), storage.Executable)
	return append(args, sessionArgs(storage, "-s", id)...)
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func appleScriptQuote(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	value = strings.ReplaceAll(value, "\r", `\r`)
	value = strings.ReplaceAll(value, "\n", `\n`)
	return `"` + value + `"`
}

func terminalCommand(term string, storage db.StorageContext, id string) *exec.Cmd {
	args := terminalSessionArgs(storage, id)
	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = shellQuote(arg)
	}
	shellCmd := strings.Join(quoted, " ")
	var cmd *exec.Cmd
	switch term {
	case "terminal":
		script := `tell application "Terminal" to do script ` + appleScriptQuote(shellCmd)
		cmd = exec.Command("osascript", "-e", script, "-e", `tell application "Terminal" to activate`)
	case "ghostty":
		cmd = exec.Command("open", "-na", "Ghostty", "--args", "--initial-command", shellCmd)
	case "iterm2":
		script := `tell application "iTerm2" to create window with default profile command ` + appleScriptQuote(shellCmd)
		cmd = exec.Command("osascript", "-e", script)
	default:
		parts := strings.Fields(term)
		parts = append(parts, args...)
		cmd = exec.Command(parts[0], parts[1:]...)
	}
	cmd.Env = storageEnvironment(storage)
	return cmd
}

func openSession(sessionID string, store *db.Store) tea.Cmd {
	return func() tea.Msg {
		cmd := terminalCommand(ui.ResolveTerminal(""), store.Storage(), sessionID)
		if err := cmd.Start(); err != nil {
			return openedMsg{sessionID: sessionID, err: err}
		}
		go func() { _ = cmd.Wait() }()
		return openedMsg{sessionID: sessionID}
	}
}

func pruneOrphans(store *db.Store) tea.Cmd {
	return func() tea.Msg {
		count, bytes, err := store.PruneOrphans()
		return prunedMsg{count: count, bytes: bytes, err: err}
	}
}

func vacuumDB(store *db.Store, beforeTotal int64) tea.Cmd {
	return func() tea.Msg {
		used, total, err := store.VacuumDB()
		return vacuumedMsg{dbUsed: used, dbTotal: total, beforeTotal: beforeTotal, err: err}
	}
}

func reloadData(store *db.Store, rootOnly bool, costPeriod CostPeriod) tea.Cmd {
	return func() tea.Msg {
		return readData(store, rootOnly, costPeriod)
	}
}

func readData(store *db.Store, rootOnly bool, costPeriod CostPeriod) reloadedMsg {
	sessions, err := store.LoadSessions(rootOnly)
	if err != nil {
		return reloadedMsg{err: fmt.Errorf("loading sessions: %w", err)}
	}

	totalCount, err := store.SessionCount()
	if err != nil {
		return reloadedMsg{err: fmt.Errorf("counting sessions: %w", err)}
	}

	dbUsed, dbTotal, err := store.DBStats()
	if err != nil {
		return reloadedMsg{err: fmt.Errorf("reading db stats: %w", err)}
	}

	orphanCount, orphanBytes, _ := store.OrphanStats()

	var periodCost float64
	if costPeriod != "" {
		start, end, err := CostPeriodBounds(costPeriod, time.Now())
		if err != nil {
			return reloadedMsg{err: fmt.Errorf("computing period bounds (%s): %w", costPeriod, err)}
		}
		periodCost, err = store.PeriodCost(start.UnixMilli(), end.UnixMilli())
		if err != nil {
			return reloadedMsg{err: fmt.Errorf("reading period cost (%s): %w", costPeriod, err)}
		}
	}

	return reloadedMsg{
		sessions:    sessions,
		newTotal:    totalCount,
		dbUsed:      dbUsed,
		dbTotal:     dbTotal,
		orphanCount: orphanCount,
		orphanBytes: orphanBytes,
		periodCost:  periodCost,
	}
}
