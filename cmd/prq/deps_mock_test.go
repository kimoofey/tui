//go:build mock

package main

import "testing"

func TestMockCheckDependenciesWithoutGH(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if err := checkDependencies(); err != nil {
		t.Fatalf("mock startup must work without gh: %v", err)
	}
}
