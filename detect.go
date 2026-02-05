package main

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	catDocs  = "docs"
	catTest  = "test"
	catCI    = "ci"
	catBuild = "build"
	catChore = "chore"
	catCode  = "code"
)

var (
	goExportedRe   = regexp.MustCompile(`^(func\s+(?:\([^)]+\)\s+)?|type\s+|var\s+|const\s+)([A-Z][A-Za-z0-9_]*)`)
	jsExportedRe   = regexp.MustCompile(`^export\s+(?:default\s+)?(?:function|class|const|let|var|interface|type)\s+([A-Z][A-Za-z0-9_]*)`)
	rustExportedRe = regexp.MustCompile(`^(?:pub\s+)?(?:fn|struct|enum|trait)\s+([A-Z][A-Za-z0-9_]*)`)
)

func detectType(changes []Change, diff string, opts Options) (string, []string) {
	if opts.Type != "" {
		return strings.ToLower(opts.Type), []string{"type override"}
	}
	counts := map[string]int{}
	var hasNewCodeFile bool
	var hasPerfHint bool
	var hasRefactorHint bool
	var hasStyleHint bool

	for _, ch := range changes {
		cat := categorizePath(ch.Path)
		counts[cat]++
		if cat == catCode && (ch.Status == "A" || ch.Status == "U" || ch.Status == "C") {
			hasNewCodeFile = true
		}
		lower := strings.ToLower(ch.Path)
		if strings.Contains(lower, "perf") || strings.Contains(lower, "optimiz") {
			hasPerfHint = true
		}
		if strings.Contains(lower, "refactor") || strings.Contains(lower, "cleanup") {
			hasRefactorHint = true
		}
		if strings.Contains(lower, "lint") || strings.Contains(lower, "format") || strings.Contains(lower, "style") {
			hasStyleHint = true
		}
	}

	reasons := []string{}
	if counts[catCode] == 0 {
		t := dominantNonCode(counts)
		reasons = append(reasons, "only non-code files")
		return t, reasons
	}

	if hasPerfHint || diffHasKeyword(diff, []string{"perf", "optimiz", "speed"}) {
		reasons = append(reasons, "performance hints")
		return "perf", reasons
	}
	if hasRefactorHint || diffHasKeyword(diff, []string{"refactor", "cleanup", "restructure"}) {
		reasons = append(reasons, "refactor hints")
		return "refactor", reasons
	}
	if hasStyleHint || diffHasKeyword(diff, []string{"format", "lint", "style"}) {
		reasons = append(reasons, "style hints")
		return "style", reasons
	}
	if hasNewCodeFile || len(findExportedNames(diff, '+')) > 0 {
		reasons = append(reasons, "new code or exported symbols")
		return "feat", reasons
	}
	reasons = append(reasons, "defaulted to fix")
	return "fix", reasons
}

func detectBreaking(changes []Change, diff string, opts Options) (bool, string) {
	if opts.Breaking {
		return true, ""
	}
	if diffHasKeyword(diff, []string{"breaking change", "breaking-change"}) {
		return true, ""
	}
	removed := findExportedNames(diff, '-')
	if len(removed) > 0 {
		return true, "removed exported symbols: " + strings.Join(removed, ", ")
	}
	return false, ""
}

func detectScope(changes []Change, override string) string {
	if strings.TrimSpace(override) != "" {
		return sanitizeScope(override)
	}
	if len(changes) == 0 {
		return ""
	}
	if len(changes) == 1 {
		return sanitizeScope(scopeFromPath(changes[0].Path))
	}

	var scope string
	for i, ch := range changes {
		candidate := topLevel(ch.Path)
		if candidate == "" {
			return ""
		}
		if i == 0 {
			scope = candidate
			continue
		}
		if scope != candidate {
			return ""
		}
	}
	return sanitizeScope(scope)
}

func categorizePath(path string) string {
	lower := strings.ToLower(path)
	base := strings.ToLower(filepath.Base(path))
	ext := strings.ToLower(filepath.Ext(path))

	if lower == "readme" || strings.HasPrefix(lower, "readme.") || strings.HasPrefix(lower, "changelog") || strings.HasPrefix(lower, "license") || strings.HasPrefix(lower, "contributing") {
		return catDocs
	}
	if strings.HasPrefix(lower, "docs/") || ext == ".md" || ext == ".rst" || ext == ".adoc" {
		return catDocs
	}
	if strings.Contains(lower, "/test/") || strings.Contains(lower, "/tests/") || strings.HasSuffix(base, "_test.go") || strings.Contains(base, ".spec.") || strings.Contains(base, ".test.") {
		return catTest
	}
	if strings.HasPrefix(lower, ".github/workflows/") || strings.HasPrefix(lower, ".github/actions/") || strings.HasPrefix(lower, ".circleci/") || strings.HasPrefix(lower, ".gitlab-ci") || base == "jenkinsfile" || base == "azure-pipelines.yml" || base == "appveyor.yml" {
		return catCI
	}
	if base == "makefile" || base == "dockerfile" || base == "go.mod" || base == "go.sum" || base == "package.json" || base == "package-lock.json" || base == "pnpm-lock.yaml" || base == "yarn.lock" || base == "cargo.toml" || base == "cargo.lock" || base == "pom.xml" || base == "build.gradle" || base == "build.gradle.kts" || base == "settings.gradle" || base == "settings.gradle.kts" || base == "gradle.properties" || base == "cmakelists.txt" {
		return catBuild
	}
	if strings.HasPrefix(lower, "build/") || strings.HasPrefix(lower, "docker/") || strings.HasPrefix(lower, "vendor/") || strings.HasPrefix(lower, "third_party/") {
		return catBuild
	}
	if strings.HasPrefix(lower, "scripts/") || strings.HasPrefix(lower, "tools/") || strings.HasPrefix(lower, "config/") || strings.HasPrefix(lower, ".vscode/") {
		return catChore
	}
	if base == ".gitignore" || base == ".gitattributes" || base == ".editorconfig" || strings.HasPrefix(base, ".prettierrc") || strings.HasPrefix(base, ".eslintrc") || base == "tsconfig.json" || base == "eslint.config.js" || base == ".pre-commit-config.yaml" || base == "ruff.toml" {
		return catChore
	}
	return catCode
}

func dominantNonCode(counts map[string]int) string {
	order := []string{catDocs, catTest, catCI, catBuild, catChore}
	best := catChore
	bestCount := -1
	for _, cat := range order {
		count := counts[cat]
		if count > bestCount {
			bestCount = count
			best = cat
		}
	}
	if bestCount <= 0 {
		return "chore"
	}
	return best
}

func diffHasKeyword(diff string, keywords []string) bool {
	if diff == "" {
		return false
	}
	lowerKeywords := make([]string, len(keywords))
	for i, kw := range keywords {
		lowerKeywords[i] = strings.ToLower(kw)
	}
	lines := strings.Split(diff, "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		if isDiffHeader(line) {
			continue
		}
		if line[0] != '+' && line[0] != '-' {
			continue
		}
		content := strings.ToLower(strings.TrimSpace(line[1:]))
		for _, kw := range lowerKeywords {
			if strings.Contains(content, kw) {
				return true
			}
		}
	}
	return false
}

func findExportedNames(diff string, prefix byte) []string {
	if diff == "" {
		return nil
	}
	set := map[string]struct{}{}
	lines := strings.Split(diff, "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		if line[0] != prefix {
			continue
		}
		if isDiffHeader(line) {
			continue
		}
		content := strings.TrimSpace(line[1:])
		if m := goExportedRe.FindStringSubmatch(content); len(m) > 2 {
			set[m[2]] = struct{}{}
			continue
		}
		if m := jsExportedRe.FindStringSubmatch(content); len(m) > 1 {
			set[m[1]] = struct{}{}
			continue
		}
		if m := rustExportedRe.FindStringSubmatch(content); len(m) > 1 {
			set[m[1]] = struct{}{}
			continue
		}
	}
	var out []string
	for name := range set {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func isDiffHeader(line string) bool {
	return strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") || strings.HasPrefix(line, "@@") || strings.HasPrefix(line, "diff ") || strings.HasPrefix(line, "index ")
}

func topLevel(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return ""
	}
	return parts[0]
}

func scopeFromPath(path string) string {
	if top := topLevel(path); top != "" {
		return top
	}
	base := filepath.Base(path)
	if base == "" || base == "." {
		return ""
	}
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func primaryArea(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return strings.TrimSuffix(parts[0], filepath.Ext(parts[0]))
	}
	if parts[0] == "cmd" || parts[0] == "pkg" || parts[0] == "internal" || parts[0] == "src" || parts[0] == "lib" || parts[0] == "app" {
		if len(parts) >= 2 {
			return parts[0] + "/" + parts[1]
		}
	}
	return parts[0]
}

func sanitizeScope(scope string) string {
	scope = strings.TrimSpace(scope)
	scope = strings.ToLower(scope)
	scope = strings.ReplaceAll(scope, " ", "-")
	var b strings.Builder
	for _, r := range scope {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '/' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
