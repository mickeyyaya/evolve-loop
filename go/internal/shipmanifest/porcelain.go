package shipmanifest

import (
	"strconv"
	"strings"
)

// UnquoteGitPath decodes one C-quoted path token from git's output into the
// literal on-disk path. SSOT for every reader that classifies paths out of git
// output (ChangedPaths here, ship's dropIgnoredPaths ignored set) — the
// isRepoRelative/manifestCovers shared-helper pattern.
//
// git quotes a path (core.quotePath, default true) whenever it contains a
// non-ASCII byte, a quote, a backslash, or a control char, escaping the payload
// with the SAME grammar Go string literals use: `\\`, `\"`, `\t`/`\n`/`\r`/…,
// and per-BYTE octal `\NNN` (`café.txt` → `"caf\303\251.txt"`, two escapes for
// one rune). strconv.Unquote is therefore the exact decoder, not an
// approximation. Cycle-1108: leaving it undecoded yielded the 15-byte literal
// `caf\303\251.txt`, a path that exists on no disk — it matched no manifest
// entry and staged nothing.
//
// Decoding is CONDITIONAL on the token being wrapped in quotes on both ends:
// an unquoted token is a path git did not escape, so its backslashes are
// literal (`not\quoted.txt`) and touching it would corrupt the common case. A
// token that fails to decode is likewise returned verbatim — never dropped.
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

// ChangedPaths parses `git status --porcelain` output into the sorted
// set of repo-relative paths it names. A rename entry ("R  old -> new") yields
// BOTH sides, so an explicit staging pathspec records the deletion as well as
// the addition. Quoted entries are decoded (UnquoteGitPath) so a non-ASCII path
// is classified as the file that exists on disk.
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

// GonePaths returns the paths a `git add` pathspec must NOT name, because
// they exist in neither the worktree nor the index under that name. Naming one
// is fatal rc=128 ("did not match any files") and git fails the ENTIRE add, so
// one such path fails the whole ship.
//
// Two porcelain shapes produce it, and they are spelled differently — which is
// why this is a function and not a one-line prefix check:
//
//	"D  <path>"           a staged deletion, worktree side clean
//	"R  <old> -> <new>"   a staged rename; <old> is gone, <new> is a real file
//
// Only the INDEX column (line[0]) decides. An unstaged deletion (" D") still
// has an index entry for `add` to remove, so it stays in the pathspec. A copy
// ("C  <src> -> <dst>") leaves <src> on disk, so it is not gone. "DD" is the
// both-deleted MERGE CONFLICT state, where `git add <path>` is the resolution
// and must not be filtered — hence the "D " prefix test rather than line[0]=='D'.
//
// The rename shape is the one that was missing, and it is not reachable from
// the deletion shape: porcelain NEVER reports a staged move as "D  <old>" plus
// "A  <new>", so a filter that knows only "D " misses every move. Inbox
// reconciliation produces moves by construction — an item is rewritten with one
// field appended and relocated to consumed/, which git scores as a rename — and
// this had been carried as an operator gotcha ("ship-staging RENAME rc=128")
// rather than fixed, failing every boundary ship that consumed a queue item.
func gonePaths(porcelain string) map[string]bool {
	gone := map[string]bool{}
	for _, line := range strings.Split(porcelain, "\n") {
		if len(line) <= 3 {
			continue
		}
		// UnquoteGitPath: keys are compared against the DECODED paths
		// Pathspec produced, so a quoted entry must decode too.
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
