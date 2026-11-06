//go:build !mock

package main

import (
	"strings"
	"testing"
)

func TestCheckDependenciesRequiresGH(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	err := checkDependencies()
	if err == nil || !strings.Contains(err.Error(), "'gh' is required") {
		t.Fatalf("checkDependencies() = %v; want missing gh error", err)
	}
}
