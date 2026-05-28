package prq

import (
	"fmt"
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"
	ui "github.com/kimoofey/tui/internal/ui"
)

// openLaunchErrMsg is sent when OpenInBrowser or OpenInTerminal fails to
// launch the external process. The model surfaces it in the status bar.
type openLaunchErrMsg struct{ Err error }

// OpenInBrowser opens url in the system default browser.
func OpenInBrowser(url string) tea.Cmd {
	return func() tea.Msg {
		if !strings.HasPrefix(url, "https://github.com/") {
			return openLaunchErrMsg{Err: fmt.Errorf("unexpected URL: %s", url)}
		}
		cmd := exec.Command("open", url)
		if err := cmd.Start(); err != nil {
			return openLaunchErrMsg{Err: err}
		}
		return nil
	}
}

// OpenInTerminal opens opencode in a new terminal window (fire-and-forget).
func OpenInTerminal(url string, cfg Config) tea.Cmd {
	return func() tea.Msg {
		term := ui.ResolveTerminal(cfg.OpencodeTerminal)
		prompt := fmt.Sprintf("Review this pull request: %s", url)
		safePrompt := strings.ReplaceAll(prompt, "'", `'\''`)
		var cmd *exec.Cmd

		switch term {
		case "terminal":
			script := fmt.Sprintf(
				`tell application "Terminal" to do script "opencode --prompt '%s'"`,
				safePrompt,
			)
			cmd = exec.Command("osascript",
				"-e", script,
				"-e", `tell application "Terminal" to activate`,
			)

		case "ghostty":
			cmd = exec.Command("open", "-na", "Ghostty",
				"--args", "--initial-command",
				fmt.Sprintf("opencode --prompt '%s'", safePrompt),
			)

		case "iterm2":
			script := fmt.Sprintf(
				`tell application "iTerm2" to create window with default profile command "opencode --prompt '%s'"`,
				safePrompt,
			)
			cmd = exec.Command("osascript", "-e", script)

		default:
			parts := strings.Fields(term)
			parts = append(parts, "opencode", "--prompt", prompt)
			cmd = exec.Command(parts[0], parts[1:]...)
		}

		if err := cmd.Start(); err != nil {
			return openLaunchErrMsg{Err: err}
		}
		return nil
	}
}

// IsOpencodeAvailable returns true if the opencode binary is found in PATH.
func IsOpencodeAvailable() bool {
	_, err := exec.LookPath("opencode")
	return err == nil
}
