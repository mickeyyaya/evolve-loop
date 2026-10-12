package gc

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

const leftoverNamesShown = 3

var devRebuildableIgnored = map[string]string{
	"go/bin/":              "the output of make build",
	"go/evolve":            "the binary that go build writes in go/",
	"go/coverage*.txt":     "coverage profiles that make test-integration writes",
	"go/coverage.func.txt": "the function coverage report of make cover",
	"node_modules/":        "restored by the package manager from the lock file",
	".gocache/":            "a Go build cache kept inside the tree",
	".gomodcache/":         "a Go module cache kept inside the tree",
}

func rebuildableIgnored(entry string) bool {
	for pat := range devRebuildableIgnored {
		if ok, _ := path.Match(pat, entry); ok {
			return true
		}
		if strings.HasSuffix(pat, "/") && strings.Contains("/"+entry, "/"+pat) {
			return true
		}
	}
	return false
}

func (s *devSweep) gateLeftovers(ctx context.Context, it *DevItem) (string, error) {
	tree := gitexec.Git{Dir: it.Path, Exec: s.o.Exec}
	held, err := heldIgnored(ctx, tree)
	if err != nil {
		return "", err
	}
	if len(held) > 0 {
		return "holds ignored files: " + firstNames(held), nil
	}
	untracked, err := tree.Output(ctx, "ls-files", "--others", "--exclude-standard", "--directory", "-z")
	if err != nil {
		return "", err
	}
	for _, entry := range splitNul(untracked) {
		if strings.HasSuffix(entry, "/") {
			nested, err := nestedRepo(filepath.Join(it.Path, entry))
			if err != nil {
				return "", err
			}
			if nested != "" {
				return "holds a nested git repository: " + nested, nil
			}
		}
	}
	return "", nil
}

func heldIgnored(ctx context.Context, tree gitexec.Git) ([]string, error) {
	listed, err := tree.Output(ctx, "ls-files", "--others", "--ignored", "--exclude-standard", "--directory", "-z")
	if err != nil {
		return nil, err
	}
	var held []string
	for _, entry := range splitNul(listed) {
		switch {
		case rebuildableIgnored(entry):
		case strings.HasSuffix(entry, "/"):
			files, err := tree.Output(ctx, "ls-files", "--others", "--ignored", "--exclude-standard", "-z", "--", literalPathspec+entry)
			if err != nil {
				return nil, err
			}
			for _, f := range splitNul(files) {
				if !rebuildableIgnored(f) {
					held = append(held, f)
				}
			}
		default:
			held = append(held, entry)
		}
	}
	return held, nil
}

func nestedRepo(dir string) (string, error) {
	var found string
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Name() == ".git" {
			found = filepath.Dir(p)
			return filepath.SkipAll
		}
		return nil
	})
	return found, err
}

func firstNames(names []string) string {
	shown := names
	if len(shown) > leftoverNamesShown {
		shown = shown[:leftoverNamesShown]
	}
	text := strings.Join(shown, ", ")
	if len(names) > len(shown) {
		text += fmt.Sprintf(" and %d more", len(names)-len(shown))
	}
	return text
}
