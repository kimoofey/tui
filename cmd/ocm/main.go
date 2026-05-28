package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	tea "charm.land/bubbletea/v2"
	"github.com/kimoofey/tui/internal/ui"
	"github.com/kimoofey/tui/ocm"
	"github.com/kimoofey/tui/ocm/db"
)

// version is set at build time via -ldflags "-X main.version=..."
var version = "dev"

func main() {
	sessionsOnly := flag.Bool("sessions", false, "show only top-level sessions, hide subagent workflows")
	versionFlag := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("ocm %s %s/%s\n", version, runtime.GOOS, runtime.GOARCH)
		os.Exit(0)
	}

	dbPath, err := db.DBPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ocm: %v\n", err)
		os.Exit(1)
	}

	sessions, err := db.LoadSessions(dbPath, *sessionsOnly)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ocm: failed to load sessions from %s: %v\n", dbPath, err)
		os.Exit(1)
	}

	totalCount, err := db.SessionCount(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ocm: failed to count sessions: %v\n", err)
		os.Exit(1)
	}

	dbUsed, dbTotal, err := db.DBStats(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ocm: failed to read db stats: %v\n", err)
		os.Exit(1)
	}

	orphanCount, orphanBytes, err := db.OrphanStats(dbPath)
	if err != nil {
		orphanCount, orphanBytes = 0, 0
	}

	if len(sessions) == 0 {
		fmt.Println("No OpenCode sessions found.")
		os.Exit(0)
	}

	m := ocm.New(sessions, totalCount, dbUsed, dbTotal, orphanCount, orphanBytes, dbPath, *sessionsOnly, 0, 0)

	_, _ = os.Stdout.WriteString(ui.PrePaint)

	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "ocm: %v\n", err)
		os.Exit(1)
	}
}
