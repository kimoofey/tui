package terminal

import (
	"fmt"
	"os/exec"
	"strings"
)

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

// Build constructs a terminal invocation without starting a process.
func Build(term string, spec Command) (*exec.Cmd, error) {
	args, err := spec.Argv()
	if err != nil {
		return nil, err
	}
	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = shellQuote(arg)
	}
	shellCmd := strings.Join(quoted, " ")
	if spec.Dir != "" {
		shellCmd = "cd " + shellQuote(spec.Dir) + " && " + shellCmd
	}
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
		if len(parts) == 0 {
			return nil, fmt.Errorf("empty terminal executable")
		}
		if spec.Dir != "" {
			args = []string{"/bin/sh", "-c", shellCmd}
		}
		parts = append(parts, args...)
		cmd = exec.Command(parts[0], parts[1:]...)
	}
	cmd.Env = spec.Environment()
	return cmd, nil
}

// Launcher starts terminal processes and reaps them independently of requests.
type Launcher struct{ Terminal string }

func (l Launcher) Launch(spec Command) error {
	cmd, err := Build(l.Terminal, spec)
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
