package prq

import "testing"

func TestFileTypeWeight(t *testing.T) {
	tests := []struct {
		path string
		want float64
	}{
		{"src/app.tsx", weightCode},
		{"src/hooks/use-user.spec.ts", weightTest},
		{"src/__tests__/widget.test.tsx", weightTest},
		{"docs/architecture.md", weightDocs},
		{"charts/app/values.yaml", weightConfig},
		{".github/workflows/ci.yml", weightConfig},
		{"pnpm-lock.yaml", weightDeps},
		{"src/generated/types.ts", weightDeps},
		{"assets/logo.svg", weightBinary},
	}

	for _, tt := range tests {
		if got := fileTypeWeight(tt.path); got != tt.want {
			t.Fatalf("fileTypeWeight(%q) = %v; want %v", tt.path, got, tt.want)
		}
	}
}

func TestFileSwitchWeight_BinaryIgnored(t *testing.T) {
	if got := fileSwitchWeight("assets/banner.webp"); got != 0 {
		t.Fatalf("fileSwitchWeight(binary) = %v; want 0", got)
	}
	if got := fileSwitchWeight("src/main.ts"); got != 1 {
		t.Fatalf("fileSwitchWeight(code) = %v; want 1", got)
	}
}

func TestIsPureRename(t *testing.T) {
	tests := []struct {
		name string
		file PRFile
		want bool
	}{
		{name: "pure rename", file: PRFile{Path: "src/a.ts", ChangeType: "RENAMED", Additions: 0, Deletions: 0}, want: true},
		{name: "rename with edits", file: PRFile{Path: "src/a.ts", ChangeType: "RENAMED", Additions: 3, Deletions: 0}, want: false},
		{name: "modified no edits", file: PRFile{Path: "src/a.ts", ChangeType: "MODIFIED", Additions: 0, Deletions: 0}, want: false},
	}

	for _, tt := range tests {
		if got := isPureRename(tt.file); got != tt.want {
			t.Fatalf("%s: isPureRename() = %v; want %v", tt.name, got, tt.want)
		}
	}
}
