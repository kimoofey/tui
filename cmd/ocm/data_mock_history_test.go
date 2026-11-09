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
