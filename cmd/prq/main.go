package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	tea "charm.land/bubbletea/v2"

	"github.com/kimoofey/tui/internal/ui"
	"github.com/kimoofey/tui/prq"
)

// version is set at build time via -ldflags "-X main.version=..."
var version = "dev"

func usage() {
	fmt.Fprintf(os.Stderr, `Usage: prq [OPTIONS]

Find open GitHub PRs that need your review.

Sources:
  [direct]  PRs where you were directly requested to review
  [team]    PRs where your team or code-owners group was requested
  [watch]   PRs from watch_repos you want to volunteer on

Options:
  --repo ORG/REPO        Add a watch repo for this run (repeatable)
  --days N               Look back N days for all sources (default: 30)
  --include-reviewed     Include PRs you've already reviewed
  --include-bots         Include bot-authored PRs
  --debug                Write diagnostic logs to /tmp/prq-debug.log
  --init                 Scaffold config at %s
  --version              Print version and exit
  -h, --help             Show this help

Config file:
  %s
`, prq.GlobalConfigPath(), prq.GlobalConfigPath())
}

type stringSlice []string

func (s *stringSlice) String() string { return "" }
func (s *stringSlice) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func main() {
	cfg, err := prq.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	var (
		extraRepos      stringSlice
		includeReviewed bool
		includeBots     bool
		initFlag        bool
		versionFlag     bool
		debugFlag       bool
	)

	flag.Usage = usage
	flag.Var(&extraRepos, "repo", "add a watch repo for this run (repeatable)")
	flag.IntVar(&cfg.DaysAgo, "days", cfg.DaysAgo, "look back N days for all sources")
	flag.BoolVar(&includeReviewed, "include-reviewed", false, "include PRs you've already reviewed")
	flag.BoolVar(&includeBots, "include-bots", false, "include bot-authored PRs")
	flag.BoolVar(&debugFlag, "debug", false, "write diagnostic logs to /tmp/prq-debug.log")
	flag.BoolVar(&initFlag, "init", false, "scaffold config and exit")
	flag.BoolVar(&versionFlag, "version", false, "print version and exit")
	flag.Parse()

	if versionFlag {
		fmt.Printf("prq %s %s/%s\n", version, runtime.GOOS, runtime.GOARCH)
		os.Exit(0)
	}
	if initFlag {
		if err := prq.ScaffoldConfig(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	if includeReviewed {
		cfg.SkipAlreadyReviewed = false
	}
	if includeBots {
		cfg.SkipBots = false
	}
	cfg.WatchRepos = append(cfg.WatchRepos, extraRepos...)

	log.SetOutput(io.Discard)
	if debugFlag {
		prq.DebugEnabled = true
		logPath := filepath.Join(os.TempDir(), "prq-debug.log")
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not open debug log: %v\n", err)
		} else {
			log.SetOutput(f)
			log.SetFlags(log.Ltime | log.Lmicroseconds)
			defer func() { _ = f.Close() }()
			fmt.Fprintf(os.Stderr, "debug log: %s\n", logPath)
			log.Printf("[prq] version=%s", version)
			log.Printf("[prq] config=%+v", cfg)
		}
	}

	checkDep("gh")

	_, _ = os.Stdout.WriteString(ui.PrePaint)

	p := tea.NewProgram(prq.NewModel(cfg))
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func checkDep(name string) {
	if _, err := exec.LookPath(name); err != nil {
		fmt.Fprintf(os.Stderr, "Error: '%s' is required but not installed.\n", name)
		os.Exit(1)
	}
}
