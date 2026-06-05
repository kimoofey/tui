package prq

import (
	"path/filepath"
	"strings"
)

const (
	weightCode       = 1.0
	weightTest       = 0.45
	weightConfig     = 0.6
	weightDocs       = 0.25
	weightDeps       = 0.15
	weightBinary     = 0.0
	weightFileSwitch = 1.0

	defaultBaseOverhead       = 2.0
	defaultReviewThroughput   = 30.0
	defaultContextFreeFiles   = 3.0
	defaultContextSwitchMins  = 0.75
	deletionsWeight           = 0.4
	minCoverageForFileOnlyEst = 0.5
	maxCoverageScaleUp        = 1.8
)

func fileTypeWeight(path string) float64 {
	lowerPath := strings.ToLower(path)
	base := strings.ToLower(filepath.Base(path))
	ext := strings.ToLower(filepath.Ext(path))

	if isBinaryOrAssetExt(ext) {
		return weightBinary
	}
	if isDependencyOrGeneratedFile(lowerPath, base, ext) {
		return weightDeps
	}
	if isTestPath(lowerPath, base) {
		return weightTest
	}
	if isDocsExt(ext) {
		return weightDocs
	}
	if isConfigFile(lowerPath, base, ext) {
		return weightConfig
	}
	return weightCode
}

func fileSwitchWeight(path string) float64 {
	if fileTypeWeight(path) == weightBinary {
		return 0
	}
	return weightFileSwitch
}

func isBinaryOrAssetExt(ext string) bool {
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".pdf", ".ico", ".ttf", ".woff", ".woff2", ".zip", ".jar", ".mp4", ".wav":
		return true
	default:
		return false
	}
}

func isDocsExt(ext string) bool {
	switch ext {
	case ".md", ".mdx", ".txt", ".puml", ".drawio", ".rst":
		return true
	default:
		return false
	}
}

func isTestPath(lowerPath, base string) bool {
	if strings.Contains(lowerPath, "__tests__") ||
		strings.Contains(lowerPath, "/test/") ||
		strings.Contains(lowerPath, "/tests/") ||
		strings.Contains(lowerPath, "cypress") ||
		strings.Contains(lowerPath, "e2e") ||
		strings.Contains(lowerPath, "integration") {
		return true
	}
	return strings.Contains(base, ".test.") || strings.Contains(base, ".spec.") || strings.HasSuffix(base, "_test.go")
}

func isConfigFile(lowerPath, base, ext string) bool {
	if base == "dockerfile" || base == "makefile" || base == "codeowners" {
		return true
	}
	if strings.Contains(lowerPath, ".github/workflows/") || strings.Contains(lowerPath, ".circleci/") || strings.Contains(lowerPath, "charts/") {
		return true
	}
	switch ext {
	case ".yml", ".yaml", ".json", ".tf", ".tfvars", ".hcl", ".toml", ".properties", ".xml", ".groovy", ".conf", ".cfg":
		return true
	default:
		return false
	}
}

func isDependencyOrGeneratedFile(lowerPath, base, ext string) bool {
	if base == "pnpm-lock.yaml" || base == "package-lock.json" || base == "yarn.lock" || base == "go.sum" || base == "cargo.lock" {
		return true
	}
	if strings.HasSuffix(base, ".snap") || strings.HasSuffix(base, ".baseline") || strings.HasSuffix(base, ".tsbuildinfo") {
		return true
	}
	if strings.Contains(lowerPath, "generated") || strings.Contains(lowerPath, "gen/") {
		return true
	}
	return ext == ".lock"
}
