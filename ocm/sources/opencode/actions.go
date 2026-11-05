package opencode

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func runDelete(ctx context.Context, storage storageContext, id string) error {
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

func sessionArgs(storage storageContext, args ...string) []string {
	if storage.V2CLI {
		args = append(args, "--standalone")
	}
	return args
}

func storageEnvironment(storage storageContext) []string {
	env := make([]string, 0, len(os.Environ())+2)
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "OPENCODE_DB=") && !strings.HasPrefix(value, "XDG_DATA_HOME=") {
			env = append(env, value)
		}
	}
	return append(env, "OPENCODE_DB="+storage.DBPath, "XDG_DATA_HOME="+filepath.Dir(storage.DataDir))
}
