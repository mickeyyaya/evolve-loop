package changedpkgs

import (
	"context"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

type ChangedFile struct {
	Path  string
	Added bool
}

func ChangedFilesChecked(repoRoot, baseRef string) ([]ChangedFile, bool) {
	if repoRoot == "" || baseRef == "" {
		return nil, false
	}
	g := gitexec.Default(repoRoot)
	ctx := context.Background()
	out, err := g.Output(ctx, "diff", "--name-status", baseRef)
	if err != nil {
		return nil, false
	}
	var files []ChangedFile
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Split(strings.TrimRight(line, "\r"), "\t")
		if len(fields) < 2 || strings.TrimSpace(fields[0]) == "" {
			continue
		}
		status := fields[0][0]
		files = append(files, ChangedFile{Path: fields[len(fields)-1], Added: status == 'A' || status == 'R' || status == 'C'})
	}
	out, err = g.Output(ctx, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, false
	}
	for _, p := range strings.Split(out, "\n") {
		if p = strings.TrimSpace(p); p != "" {
			files = append(files, ChangedFile{Path: p, Added: true})
		}
	}
	return files, true
}

func PackagesOf(files []ChangedFile) []string {
	set := map[string]struct{}{}
	for _, f := range files {
		if pkg, ok := FileToPackage(f.Path); ok {
			set[pkg] = struct{}{}
		}
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
