package shipmanifest

import (
	"strconv"
	"strings"
)

func UnquoteGitPath(tok string) string {
	if len(tok) < 2 || tok[0] != '"' || tok[len(tok)-1] != '"' {
		return tok
	}
	if p, err := strconv.Unquote(tok); err == nil {
		return p
	}
	return tok
}

func splitPorcelainRename(path string) []string {
	for i := 0; i < len(path); i++ {
		if path[i] == '"' {
			for i++; i < len(path); i++ {
				if path[i] == '\\' {
					i++
					continue
				}
				if path[i] == '"' {
					break
				}
			}
			continue
		}
		if strings.HasPrefix(path[i:], " -> ") {
			return []string{path[:i], path[i+4:]}
		}
	}
	return []string{path}
}

func ChangedPaths(out string) []string {
	seen := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		if len(line) <= 3 {
			continue
		}
		for _, part := range splitPorcelainRename(line[3:]) {
			if p := UnquoteGitPath(strings.TrimSpace(part)); p != "" {
				seen[p] = true
			}
		}
	}
	return sortedKeys(seen)
}

func gonePaths(porcelain string) map[string]bool {
	gone := map[string]bool{}
	for _, line := range strings.Split(porcelain, "\n") {
		if len(line) <= 3 {
			continue
		}
		switch {
		case strings.HasPrefix(line, "D "):
			gone[UnquoteGitPath(strings.TrimSpace(line[3:]))] = true
		case line[0] == 'R':
			if parts := splitPorcelainRename(line[3:]); len(parts) == 2 {
				gone[UnquoteGitPath(strings.TrimSpace(parts[0]))] = true
			}
		}
	}
	return gone
}
