// Package changedpkgs derives the Go packages a change touches, and those it can break, so tests run scoped to the change.
// See docs/architecture/packages/internal-changedpkgs.md.
package changedpkgs

import (
	"encoding/json"
	"os"
	"path"
	"sort"
	"strings"
)

// FileToPackage maps a repo path under go/ to its test pattern ("./internal/foo/..."), or ("", false) for anything else.
func FileToPackage(file string) (string, bool) {
	p := strings.TrimSpace(path.Clean(strings.ReplaceAll(file, "\\", "/")))
	if !strings.HasSuffix(p, ".go") {
		return "", false
	}
	p = strings.TrimPrefix(p, "./")
	if !strings.HasPrefix(p, "go/") {
		return "", false
	}
	// CHANGED_PACKAGES is space-joined and iterated unquoted, so whitespace would split a pattern.
	if strings.ContainsAny(p, " \t\n") {
		return "", false
	}
	dir := path.Dir(strings.TrimPrefix(p, "go/"))
	if dir == "." || dir == "" {
		return "./...", true
	}
	return "./" + dir + "/...", true
}

type handoffDoc struct {
	Thrusts []struct {
		FilesModified []string `json:"files_modified"`
		FilesNew      []string `json:"files_new"`
	} `json:"thrusts"`
}

// ChangedPackages returns the sorted, deduped test patterns of the Go files a handoff-build.json lists; nil when unreadable or empty.
func ChangedPackages(handoffPath string) []string {
	data, err := os.ReadFile(handoffPath)
	if err != nil {
		return nil
	}
	var d handoffDoc
	if err := json.Unmarshal(data, &d); err != nil {
		return nil
	}
	set := map[string]struct{}{}
	add := func(files []string) {
		for _, f := range files {
			if pkg, ok := FileToPackage(f); ok {
				set[pkg] = struct{}{}
			}
		}
	}
	for _, th := range d.Thrusts {
		add(th.FilesModified)
		add(th.FilesNew)
	}
	if len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// FromGit is FromGitChecked without the derivability signal: a git failure yields an empty list.
func FromGit(repoRoot, baseRef string) []string {
	pkgs, _ := FromGitChecked(repoRoot, baseRef)
	return pkgs
}

// FromGitChecked returns the test patterns the working tree changes versus baseRef; derivable is false when git failed.
func FromGitChecked(repoRoot, baseRef string) ([]string, bool) {
	files, ok := ChangedFilesChecked(repoRoot, baseRef)
	if !ok {
		return nil, false
	}
	return PackagesOf(files), true
}
