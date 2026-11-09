//go:build mock

package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestMockHistoryNeverUsesUserFiles(t *testing.T) {
	config, data := t.TempDir(), t.TempDir()
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("XDG_DATA_HOME", data)
	if err := os.MkdirAll(filepath.Join(config, "ocm"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(config, "ocm/config.yaml"), []byte("malformed: ["), 0600); err != nil {
		t.Fatal(err)
	}
	opts, err := loadData(false, "")
	if err != nil || opts.History == nil {
		t.Fatal(err)
	}
	r, err := opts.History.Query(context.Background(), nil)
	if err != nil || r.Cost == nil || r.Rows != 8 {
		t.Fatal(r, err)
	}
	if _, err = os.Stat(filepath.Join(data, "ocm")); !os.IsNotExist(err) {
		t.Fatal("mock created user archive directory", err)
	}
	if _, err = os.Stat(filepath.Join(state, "ocm")); !os.IsNotExist(err) {
		t.Fatal("mock created diagnostic files", err)
	}
}

func TestMockInitialImportAndCapabilities(t *testing.T) {
	opts, err := loadData(false, "")
	if err != nil {
		t.Fatal(err)
	}
	defer opts.Close()
	preview := opts.History.(*mockHistory)
	ctx, cancel := context.WithCancel(context.Background())
	state := preview.Refresh(ctx)
	if state.Err != nil || !state.ImportPending {
		t.Fatal(state)
	}
	cancel()
	if err := preview.Import(ctx); err == nil {
		t.Fatal("canceled preview completed")
	}
	if err := preview.Import(context.Background()); err != nil {
		t.Fatal(err)
	}
	caps := state.App.Capabilities()
	if !caps.Resume || !caps.Delete || !caps.Vacuum.Available || !caps.Orphans.Available {
		t.Fatal(caps)
	}
	state = preview.Refresh(context.Background())
	if state.ImportPending {
		t.Fatal("preview did not finish setup")
	}
}
