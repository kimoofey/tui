//go:build !mock

package main

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kimoofey/tui/ocm/service"
)

func TestConfigPrecedesDiscovery(t *testing.T) {
	config := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("PATH", t.TempDir())
	if err := os.MkdirAll(filepath.Join(config, "ocm"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(config, "ocm/config.yaml"), []byte("history:\n  unknown: true"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := loadData(false, service.CostPeriod(""))
	if err == nil || !strings.Contains(err.Error(), "history config") {
		t.Fatal(err)
	}
}
func TestHelpVersionBypassConfig(t *testing.T) {
	if arg := os.Getenv("OCM_FLAGS_HELPER"); arg != "" {
		os.Args = []string{"ocm", arg}
		flag.CommandLine = flag.NewFlagSet("ocm", flag.ExitOnError)
		os.Exit(run())
	}
	config := t.TempDir()
	if err := os.MkdirAll(filepath.Join(config, "ocm"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(config, "ocm/config.yaml"), []byte("malformed: ["), 0600); err != nil {
		t.Fatal(err)
	}
	for _, arg := range []string{"--help", "--version"} {
		cmd := exec.Command(os.Args[0], "-test.run=^TestHelpVersionBypassConfig$")
		cmd.Env = append(os.Environ(), "OCM_FLAGS_HELPER="+arg, "XDG_CONFIG_HOME="+config, "XDG_DATA_HOME="+t.TempDir(), "PATH="+t.TempDir())
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s: %s: %v", arg, out, err)
		}
	}
}
