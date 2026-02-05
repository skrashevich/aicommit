package main

import (
	"bytes"
	"errors"
	"os/exec"
	"strconv"
	"strings"
)

func ensureGit() error {
	_, err := exec.LookPath("git")
	if err != nil {
		return errors.New("git is not available in PATH")
	}
	return nil
}

func gitOutput(args ...string) (string, error) {
	out, err := gitBytes(args...)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(out), "\n"), nil
}

func gitBytes(args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	return cmd.Output()
}

func collectChanges() ([]Change, []Change, error) {
	stagedRaw, err := gitBytes("diff", "--cached", "--name-status", "-z")
	if err != nil {
		return nil, nil, err
	}
	unstagedRaw, err := gitBytes("diff", "--name-status", "-z")
	if err != nil {
		return nil, nil, err
	}
	untrackedRaw, err := gitBytes("ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, nil, err
	}

	staged := parseNameStatus(stagedRaw, ModeStaged)
	unstaged := parseNameStatus(unstagedRaw, ModeUnstaged)
	untracked := parseUntracked(untrackedRaw)
	unstaged = append(unstaged, untracked...)
	return staged, unstaged, nil
}

func parseNameStatus(data []byte, source Mode) []Change {
	if len(data) == 0 {
		return nil
	}
	fields := bytes.Split(data, []byte{0})
	var out []Change
	for i := 0; i < len(fields); {
		entry := string(fields[i])
		if entry == "" {
			i++
			continue
		}

		if strings.Contains(entry, "\t") {
			parts := strings.SplitN(entry, "\t", 2)
			if len(parts) < 2 {
				i++
				continue
			}
			status := parts[0]
			statusChar := status
			if len(status) > 0 {
				statusChar = status[:1]
			}
			if statusChar == "R" || statusChar == "C" {
				oldPath := parts[1]
				if i+1 >= len(fields) {
					break
				}
				newPath := string(fields[i+1])
				out = append(out, Change{Path: newPath, OldPath: oldPath, Status: statusChar, Source: source})
				i += 2
				continue
			}
			path := parts[1]
			out = append(out, Change{Path: path, Status: statusChar, Source: source})
			i++
			continue
		}

		status := entry
		statusChar := status
		if len(status) > 0 {
			statusChar = status[:1]
		}
		if statusChar == "R" || statusChar == "C" {
			if i+2 >= len(fields) {
				break
			}
			oldPath := string(fields[i+1])
			newPath := string(fields[i+2])
			if oldPath != "" && newPath != "" {
				out = append(out, Change{Path: newPath, OldPath: oldPath, Status: statusChar, Source: source})
			}
			i += 3
			continue
		}
		if i+1 >= len(fields) {
			break
		}
		path := string(fields[i+1])
		if path != "" {
			out = append(out, Change{Path: path, Status: statusChar, Source: source})
		}
		i += 2
	}
	return out
}

func parseUntracked(data []byte) []Change {
	if len(data) == 0 {
		return nil
	}
	fields := bytes.Split(data, []byte{0})
	var out []Change
	for _, f := range fields {
		path := strings.TrimSpace(string(f))
		if path == "" {
			continue
		}
		out = append(out, Change{Path: path, Status: "U", Source: ModeUnstaged})
	}
	return out
}

func selectChanges(mode Mode, staged, unstaged []Change) (Mode, []Change) {
	switch mode {
	case ModeStaged:
		return ModeStaged, staged
	case ModeUnstaged:
		return ModeUnstaged, unstaged
	case ModeAll:
		return ModeAll, mergeChanges(staged, unstaged)
	default:
		if len(staged) > 0 {
			return ModeStaged, staged
		}
		return ModeUnstaged, unstaged
	}
}

func mergeChanges(staged, unstaged []Change) []Change {
	byPath := map[string]Change{}
	for _, ch := range staged {
		byPath[ch.Path] = ch
	}
	for _, ch := range unstaged {
		if existing, ok := byPath[ch.Path]; ok {
			existing.Source = ModeAll
			byPath[ch.Path] = existing
			continue
		}
		ch.Source = ModeAll
		byPath[ch.Path] = ch
	}
	out := make([]Change, 0, len(byPath))
	for _, ch := range byPath {
		out = append(out, ch)
	}
	return out
}

func collectDiff(mode Mode) (string, error) {
	switch mode {
	case ModeStaged:
		return gitOutput("diff", "--cached", "-U0")
	case ModeUnstaged:
		return gitOutput("diff", "-U0")
	case ModeAll:
		unstaged, _ := gitOutput("diff", "-U0")
		staged, _ := gitOutput("diff", "--cached", "-U0")
		if unstaged == "" {
			return staged, nil
		}
		if staged == "" {
			return unstaged, nil
		}
		return unstaged + "\n" + staged, nil
	default:
		return "", nil
	}
}

func collectNumstat(mode Mode) ([]FileStat, error) {
	var combined []FileStat
	appendStats := func(stats []FileStat) {
		if len(stats) == 0 {
			return
		}
		byPath := map[string]FileStat{}
		for _, st := range combined {
			byPath[st.Path] = st
		}
		for _, st := range stats {
			existing, ok := byPath[st.Path]
			if !ok {
				byPath[st.Path] = st
				continue
			}
			existing.Added += st.Added
			existing.Deleted += st.Deleted
			existing.Binary = existing.Binary || st.Binary
			byPath[st.Path] = existing
		}
		combined = combined[:0]
		for _, st := range byPath {
			combined = append(combined, st)
		}
	}

	switch mode {
	case ModeStaged:
		out, err := gitOutput("diff", "--cached", "--numstat")
		if err != nil {
			return nil, err
		}
		return parseNumstat(out), nil
	case ModeUnstaged:
		out, err := gitOutput("diff", "--numstat")
		if err != nil {
			return nil, err
		}
		return parseNumstat(out), nil
	case ModeAll:
		unstagedRaw, _ := gitOutput("diff", "--numstat")
		stagedRaw, _ := gitOutput("diff", "--cached", "--numstat")
		appendStats(parseNumstat(unstagedRaw))
		appendStats(parseNumstat(stagedRaw))
		return combined, nil
	default:
		return nil, nil
	}
}

func parseNumstat(raw string) []FileStat {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	lines := strings.Split(raw, "\n")
	var out []FileStat
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 {
			continue
		}
		addStr := parts[0]
		delStr := parts[1]
		path := parts[2]
		stat := FileStat{Path: path}
		if addStr == "-" && delStr == "-" {
			stat.Binary = true
			out = append(out, stat)
			continue
		}
		added, err1 := strconvAtoiSafe(addStr)
		deleted, err2 := strconvAtoiSafe(delStr)
		if err1 != nil || err2 != nil {
			continue
		}
		stat.Added = added
		stat.Deleted = deleted
		out = append(out, stat)
	}
	return out
}

func strconvAtoiSafe(raw string) (int, error) {
	val, err := strconv.Atoi(raw)
	if err != nil {
		return 0, err
	}
	return val, nil
}
