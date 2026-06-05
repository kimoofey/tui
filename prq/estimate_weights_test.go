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
