package core

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/covchanged"
)

const blockNoteCap = 12

var infallibleCalls = []string{"json.MarshalIndent(", "json.Marshal("}

type blockNoter struct {
	moduleDir  string
	repoPrefix string
	protected  func(string) bool
}

func newBlockNoter(worktree, moduleDir string, protected func(string) bool) blockNoter {
	n := blockNoter{moduleDir: moduleDir, protected: protected}
	if worktree == "" {
		return n
	}
	if rel, err := filepath.Rel(worktree, moduleDir); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
		n.repoPrefix = filepath.ToSlash(rel) + "/"
	}
	return n
}

func (n blockNoter) notes(profilePath, pkg string) []string {
	module, err := covchanged.ModulePath(n.moduleDir)
	if err != nil {
		return nil
	}
	raw, err := os.ReadFile(profilePath)
	if err != nil {
		return nil
	}
	profile, err := covchanged.ParseProfile(raw, module)
	if err != nil {
		return nil
	}
	blocks := uncoveredBlocks(profile, pkg)
	var out []string
	for i, b := range blocks {
		if i == blockNoteCap {
			return append(out, fmt.Sprintf("  … %d more uncovered block(s)", len(blocks)-blockNoteCap))
		}
		out = append(out, n.blockLines(b)...)
	}
	return out
}

func uncoveredBlocks(profile covchanged.Profile, pkg string) []covchanged.Block {
	var blocks []covchanged.Block
	for file, fileBlocks := range profile {
		if path.Dir(file) != pkg {
			continue
		}
		for _, b := range fileBlocks {
			if !b.Hit {
				b.File = file
				blocks = append(blocks, b)
			}
		}
	}
	sort.Slice(blocks, func(i, j int) bool {
		if blocks[i].File != blocks[j].File {
			return blocks[i].File < blocks[j].File
		}
		return blocks[i].StartLine < blocks[j].StartLine
	})
	return blocks
}

func (n blockNoter) blockLines(b covchanged.Block) []string {
	repoPath := n.repoPrefix + b.File
	lines := []string{fmt.Sprintf("  uncovered %s:%d-%d (%d statement(s))", repoPath, b.StartLine, b.EndLine, b.Statements)}
	if n.protected != nil && n.protected(repoPath) {
		lines = append(lines, "    "+protectedFileNote(fmt.Sprintf("%s:%d", repoPath, b.StartLine)))
	}
	src, err := os.ReadFile(filepath.Join(n.moduleDir, filepath.FromSlash(b.File)))
	if err != nil {
		return lines
	}
	if call := infallibleCallBefore(strings.Split(string(src), "\n"), b.StartLine); call != "" {
		lines = append(lines, "    note: this block is the error return after "+call+", which cannot fail for a value of plain fields. A test cannot reach it: add a seam (an injectable function) or remove the dead branch.")
	}
	return lines
}

func protectedFileNote(at string) string {
	file, _, _ := strings.Cut(at, ":")
	return "note: " + file + " is a protected control-plane path (ADR-0064), so the lane cannot edit it. Do not test around it: declare it in the build-report verdict as the failure defect `BLOCKER protected-path " + at + " <why the fix needs this file>`. The floor proves the claim and routes the item to the console."
}

func infallibleCallBefore(src []string, startLine int) string {
	if startLine < 1 || startLine > len(src) {
		return ""
	}
	window := src[max(0, startLine-3):startLine]
	if !slices.ContainsFunc(window, func(l string) bool { return strings.Contains(l, "err != nil") }) {
		return ""
	}
	for _, l := range window {
		for _, call := range infallibleCalls {
			if strings.Contains(l, call) {
				return strings.TrimSuffix(call, "(")
			}
		}
	}
	return ""
}

var goSourceTokenAt = regexp.MustCompile(`([A-Za-z0-9_./-]+\.go)(:[0-9]+)?`)

func withProtectedFileNotes(failures []string, protected func(string) bool) []string {
	out := slices.Clone(failures)
	if protected == nil {
		return out
	}
	for i, f := range out {
		if acsPredicateFailure.MatchString(f) {
			out[i] = f + protectedNotesFor(f, protected)
		}
	}
	return out
}

func protectedNotesFor(f string, protected func(string) bool) string {
	var notes strings.Builder
	var seen []string
	for _, m := range goSourceTokenAt.FindAllStringSubmatch(f, -1) {
		repo := repoPathOf(m[1])
		if repo == "" || strings.HasSuffix(repo, "_test.go") || slices.Contains(seen, repo) || !protected(repo) {
			continue
		}
		seen = append(seen, repo)
		notes.WriteString("\n  " + protectedFileNote(repo+m[2]))
	}
	return notes.String()
}

func repoPathOf(token string) string {
	if strings.HasPrefix(token, "go/") {
		return token
	}
	if i := strings.LastIndex(token, "/go/"); i >= 0 {
		return token[i+1:]
	}
	return ""
}
