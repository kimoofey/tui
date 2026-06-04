package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/kimoofey/tui/internal/ui"
	"github.com/kimoofey/tui/ocm"
)

var version = "dev"

func main() {
	sessionsOnly := flag.Bool("sessions", false, "show only top-level sessions, hide subagent workflows")
	costPeriodFlag := flag.String("cost-period", "", "show cost stats for period: week|month|year")
	versionFlag := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("ocm %s %s/%s\n", version, runtime.GOOS, runtime.GOARCH)
		os.Exit(0)
	}

	var err error
	var costPeriod ocm.CostPeriod
	var opts ocm.Options
	if strings.TrimSpace(*costPeriodFlag) != "" {
		costPeriod, err = ocm.ParseCostPeriod(*costPeriodFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ocm: %v\n", err)
			os.Exit(2)
		}
	}

	opts, err = loadData(*sessionsOnly, costPeriod)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ocm: %v\n", err)
		os.Exit(1)
	}

	if len(opts.Sessions) == 0 {
		fmt.Println("No OpenCode sessions found.")
		os.Exit(0)
	}

	m := ocm.New(opts, 0, 0)

	_, _ = os.Stdout.WriteString(ui.PrePaint)

	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "ocm: %v\n", err)
		os.Exit(1)
	}
}
