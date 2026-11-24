package repostate

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

func PresentFiles(ctx context.Context, g gitexec.Git) ([]string, error) {
	listed, err := g.Output(ctx, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	if err != nil {
		return nil, fmt.Errorf("git ls-files --cached --others: %w", err)
	}
	deleted, err := g.Output(ctx, "ls-files", "-z", "--deleted")
	if err != nil {
		return nil, fmt.Errorf("git ls-files --deleted: %w", err)
	}
	gone := map[string]bool{}
	for _, path := range splitNUL(deleted) {
		gone[path] = true
	}
	var present []string
	for _, path := range splitNUL(listed) {
		if !gone[path] {
			present = append(present, path)
		}
	}
	slices.Sort(present)
	return present, nil
}

func splitNUL(out string) []string {
	var paths []string
	for _, path := range strings.Split(out, "\x00") {
		if path != "" {
			paths = append(paths, path)
		}
	}
	return paths
}
