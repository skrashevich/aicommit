package main

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"unicode/utf8"
)

func validFormat(format Format) bool {
	switch format {
	case FormatConventional, FormatPlain, FormatGitmoji:
		return true
	default:
		return false
	}
}

func validBody(body BodyMode) bool {
	switch body {
	case BodyAuto, BodyNone, BodyFiles, BodyStats, BodySummary:
		return true
	default:
		return false
	}
}

func validMode(mode Mode) bool {
	switch mode {
	case ModeAuto, ModeStaged, ModeUnstaged, ModeAll:
		return true
	default:
		return false
	}
}

func detectLang() string {
	for _, key := range []string{"LC_ALL", "LANG"} {
		val := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
		if val == "" {
			continue
		}
		if strings.HasPrefix(val, "ru") || strings.Contains(val, "_ru") {
			return "ru"
		}
		return "en"
	}
	return "en"
}

func buildSubject(commitType, scope string, changes []Change, opts Options) string {
	verb, defaultTarget := verbForType(commitType, opts.Lang)
	target := inferTarget(changes, scope)
	if target == "" {
		target = defaultTarget
	}
	if target == "" {
		if opts.Lang == "ru" {
			target = "изменения"
		} else {
			target = "changes"
		}
	}
	subject := strings.TrimSpace(verb + " " + target)
	return subject
}

func inferTarget(changes []Change, scope string) string {
	if len(changes) == 1 {
		return primaryArea(changes[0].Path)
	}
	if scope != "" {
		return scope
	}
	counts := map[string]int{}
	for _, ch := range changes {
		area := primaryArea(ch.Path)
		if area != "" {
			counts[area]++
		}
	}
	if len(counts) == 0 {
		return ""
	}
	best := ""
	bestCount := 0
	tie := false
	for area, count := range counts {
		if count > bestCount {
			best = area
			bestCount = count
			tie = false
			continue
		}
		if count == bestCount {
			tie = true
		}
	}
	if tie {
		return ""
	}
	return best
}

func verbForType(commitType, lang string) (string, string) {
	ct := strings.ToLower(commitType)
	if lang == "ru" {
		switch ct {
		case "feat":
			return "Добавь", "функциональность"
		case "fix":
			return "Исправь", "ошибки"
		case "docs":
			return "Обнови", "документацию"
		case "test":
			return "Добавь", "тесты"
		case "refactor":
			return "Улучши", "структуру кода"
		case "perf":
			return "Оптимизируй", "производительность"
		case "style":
			return "Приведи", "стиль"
		case "build":
			return "Обнови", "сборку"
		case "ci":
			return "Обнови", "CI"
		case "chore":
			return "Обнови", "инструменты"
		default:
			return "Обнови", "изменения"
		}
	}

	switch ct {
	case "feat":
		return "Add", "feature"
	case "fix":
		return "Fix", "bug"
	case "docs":
		return "Update", "docs"
	case "test":
		return "Add", "tests"
	case "refactor":
		return "Refactor", "code"
	case "perf":
		return "Optimize", "performance"
	case "style":
		return "Format", "code"
	case "build":
		return "Update", "build"
	case "ci":
		return "Update", "CI"
	case "chore":
		return "Update", "tooling"
	default:
		return "Update", "changes"
	}
}

func formatMessage(commitType, scope, subject, body string, opts Options, breaking bool) string {
	prefix := ""
	subj := subject
	if opts.Format == FormatConventional || opts.Format == FormatGitmoji {
		subj = lowerFirst(subj)
	}
	subj = trimSubject(subj, opts.MaxSubject)

	if opts.Format == FormatConventional || opts.Format == FormatGitmoji {
		prefix = strings.ToLower(commitType)
		if scope != "" {
			prefix += "(" + scope + ")"
		}
		if breaking {
			prefix += "!"
		}
		prefix += ": "
	}
	if opts.Emoji || opts.Format == FormatGitmoji {
		if code := emojiCode(commitType); code != "" {
			prefix = code + " " + prefix
		}
	}

	msg := prefix + subj
	if body != "" {
		msg += "\n\n" + body
	}
	return msg
}

func emojiCode(commitType string) string {
	switch strings.ToLower(commitType) {
	case "feat":
		return ":sparkles:"
	case "fix":
		return ":bug:"
	case "docs":
		return ":memo:"
	case "style":
		return ":art:"
	case "refactor":
		return ":recycle:"
	case "perf":
		return ":zap:"
	case "test":
		return ":white_check_mark:"
	case "build":
		return ":package:"
	case "ci":
		return ":construction_worker:"
	case "chore":
		return ":wrench:"
	default:
		return ""
	}
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return s
	}
	return strings.ToLower(string(r)) + s[size:]
}

func trimSubject(s string, max int) string {
	if max <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	runes = runes[:max]
	cut := len(runes)
	for i := len(runes) - 1; i >= 0; i-- {
		if runes[i] == ' ' {
			cut = i
			break
		}
	}
	if cut < 3 {
		cut = max
	}
	return strings.TrimSpace(string(runes[:cut]))
}

func buildBody(changes []Change, mode Mode, opts Options, breaking bool, breakingNote string) string {
	bodyMode := opts.Body
	if bodyMode == BodyAuto {
		if len(changes) == 0 {
			bodyMode = BodyNone
		} else if len(changes) <= opts.MaxItems {
			bodyMode = BodyFiles
		} else {
			bodyMode = BodySummary
		}
	}

	var content []string
	switch bodyMode {
	case BodyFiles:
		content = buildFileLines(changes, opts.MaxItems, opts.Lang)
	case BodyStats:
		stats, _ := collectNumstat(mode)
		if len(stats) == 0 {
			content = []string{summaryLine(changes, opts.Lang)}
		} else {
			content = buildStatLines(stats, opts.MaxItems, opts.Lang)
		}
	case BodySummary:
		content = []string{summaryLine(changes, opts.Lang)}
	}

	var footers []string
	if breaking {
		footers = append(footers, breakingFooter(breakingNote, opts.Lang))
	}
	if len(opts.Refs) > 0 {
		footers = append(footers, fmt.Sprintf("Refs: %s", strings.Join(opts.Refs, ", ")))
	}
	if len(opts.Closes) > 0 {
		footers = append(footers, fmt.Sprintf("Closes: %s", strings.Join(opts.Closes, ", ")))
	}

	lines := content
	if len(footers) > 0 {
		if len(content) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, footers...)
	}
	return strings.Join(lines, "\n")
}

func buildFileLines(changes []Change, maxItems int, lang string) []string {
	sorted := append([]Change{}, changes...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Path < sorted[j].Path
	})
	limit := len(sorted)
	if maxItems > 0 && limit > maxItems {
		limit = maxItems
	}
	var lines []string
	for i := 0; i < limit; i++ {
		ch := sorted[i]
		path := ch.Path
		if ch.Status == "R" && ch.OldPath != "" {
			path = ch.OldPath + " -> " + ch.Path
		}
		lines = append(lines, fmt.Sprintf("- %s %s", statusLabel(ch.Status, lang), path))
	}
	if limit < len(sorted) {
		remaining := len(sorted) - limit
		if lang == "ru" {
			lines = append(lines, fmt.Sprintf("- и еще %d", remaining))
		} else {
			lines = append(lines, fmt.Sprintf("- and %d more", remaining))
		}
	}
	return lines
}

func buildStatLines(stats []FileStat, maxItems int, lang string) []string {
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Path < stats[j].Path
	})
	limit := len(stats)
	if maxItems > 0 && limit > maxItems {
		limit = maxItems
	}
	var lines []string
	for i := 0; i < limit; i++ {
		st := stats[i]
		if st.Binary {
			lines = append(lines, fmt.Sprintf("- %s (binary)", st.Path))
			continue
		}
		lines = append(lines, fmt.Sprintf("- %s (+%d -%d)", st.Path, st.Added, st.Deleted))
	}
	if limit < len(stats) {
		remaining := len(stats) - limit
		if lang == "ru" {
			lines = append(lines, fmt.Sprintf("- и еще %d", remaining))
		} else {
			lines = append(lines, fmt.Sprintf("- and %d more", remaining))
		}
	}
	return lines
}

func summaryLine(changes []Change, lang string) string {
	counts := map[string]int{}
	for _, ch := range changes {
		counts[ch.Status]++
	}
	added := counts["A"] + counts["U"]
	modified := counts["M"]
	deleted := counts["D"]
	total := len(changes)
	if lang == "ru" {
		return fmt.Sprintf("Файлов изменено: %d (добавлено %d, удалено %d, изменено %d)", total, added, deleted, modified)
	}
	return fmt.Sprintf("Files changed: %d (added %d, removed %d, modified %d)", total, added, deleted, modified)
}

func statusLabel(status string, lang string) string {
	if lang == "ru" {
		switch status {
		case "A":
			return "добавл"
		case "M":
			return "изм"
		case "D":
			return "удал"
		case "R":
			return "переим"
		case "C":
			return "коп"
		case "U":
			return "нов"
		default:
			return "изм"
		}
	}
	switch status {
	case "A":
		return "add"
	case "M":
		return "mod"
	case "D":
		return "del"
	case "R":
		return "ren"
	case "C":
		return "cpy"
	case "U":
		return "new"
	default:
		return "mod"
	}
}

func breakingFooter(note string, lang string) string {
	if note == "" {
		if lang == "ru" {
			note = "несовместимые изменения API"
		} else {
			note = "incompatible API changes"
		}
	}
	return "BREAKING CHANGE: " + note
}

func printExplain(w io.Writer, opts Options, mode Mode, commitType, scope string, breaking bool, llmUsed bool, reasons []string, changes []Change) {
	fmt.Fprintf(w, "mode: %s (%d files)\n", mode, len(changes))
	fmt.Fprintf(w, "type: %s\n", commitType)
	if len(reasons) > 0 {
		fmt.Fprintf(w, "reasons: %s\n", strings.Join(reasons, "; "))
	}
	if scope != "" {
		fmt.Fprintf(w, "scope: %s\n", scope)
	}
	fmt.Fprintf(w, "breaking: %v\n", breaking)
	fmt.Fprintf(w, "llm: %v\n", llmUsed)
	fmt.Fprintf(w, "format: %s\n", opts.Format)
	fmt.Fprintf(w, "body: %s\n", opts.Body)
	fmt.Fprintf(w, "lang: %s\n", opts.Lang)
}
