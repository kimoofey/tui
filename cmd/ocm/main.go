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
	"github.com/kimoofey/tui/ocm/service"
)

var version = "dev"

func main() { os.Exit(run()) }

func run() int {
	sessionsOnly := flag.Bool("sessions", false, "show only top-level sessions, hide subagent workflows")
	costPeriodFlag := flag.String("cost-period", "", "show cost stats for period: week|month|year")
	versionFlag := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("ocm %s %s/%s\n", version, runtime.GOOS, runtime.GOARCH)
		return 0
	}

	var err error
	var costPeriod service.CostPeriod
	var opts ocm.Options
	if strings.TrimSpace(*costPeriodFlag) != "" {
		costPeriod, err = service.ParseCostPeriod(*costPeriodFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ocm: %v\n", err)
			return 2
		}
	}

	opts, err = loadData(*sessionsOnly, costPeriod)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ocm: %v\n", err)
		return 1
	}
	return runBrowser(opts, func(model tea.Model) error {
		_, _ = os.Stdout.WriteString(ui.PrePaint)
		_, err := tea.NewProgram(model).Run()
		return err
	})
}

func runBrowser(opts ocm.Options, program func(tea.Model) error) int {
	if opts.Close != nil {
		defer func() { _ = opts.Close() }()
	}
	if len(opts.Sessions) == 0 && opts.History == nil {
		fmt.Println("No OpenCode sessions found.")
		return 0
	}
	if err := program(ocm.New(opts, 0, 0)); err != nil {
		fmt.Fprintf(os.Stderr, "ocm: %v\n", err)
		return 1
	}
	return 0
}
