// Package gitexec runs the git CLI behind one small, injectable type.
// See docs/architecture/packages/internal-gitexec.md.
package gitexec

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const gitBin = "git"

const (
	porcelainStatusPrefixLen = len("XY ")
	minPorcelainLineLen      = porcelainStatusPrefixLen + 1
	porcelainRenameArrow     = " -> "
)

type Git struct {
	Dir  string
	Exec sysexec.RunFunc
}

func Default(dir string) Git {
	return Git{Dir: dir, Exec: sysexec.DefaultRunner}
}

func (g Git) Capture(ctx context.Context, args ...string) (stdout, stderr string, exitCode int, err error) {
	return sysexec.Capture(ctx, g.Exec, g.Dir, gitBin, args...)
}

func (g Git) Output(ctx context.Context, args ...string) (string, error) {
	return sysexec.Output(ctx, g.Exec, g.Dir, gitBin, args...)
}

func (g Git) Run(ctx context.Context, args ...string) error {
	_, err := g.Output(ctx, args...)
	return err
}

func (g Git) HEAD(ctx context.Context) (string, error) {
	return g.Output(ctx, "rev-parse", "HEAD")
}

func (g Git) DirtyPaths(ctx context.Context) ([]string, error) {
	out, stderr, code, err := g.Capture(ctx, "status", "--porcelain", "-uall")
	if err != nil {
		return nil, fmt.Errorf("gitexec: git status: %w", err)
	}
	if code != 0 {
		return nil, fmt.Errorf("gitexec: git status exit=%d: %s", code, strings.TrimSpace(stderr))
	}
	set := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		if len(line) < minPorcelainLineLen {
			continue
		}
		set[PorcelainPath(line)] = true
		if old := PorcelainOldPath(line); old != "" {
			set[old] = true
		}
	}
	paths := make([]string, 0, len(set))
	for p := range set {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths, nil
}

func PorcelainPath(line string) string {
	if len(line) < minPorcelainLineLen {
		return ""
	}
	p := strings.TrimSpace(line[porcelainStatusPrefixLen:])
	if i := strings.Index(p, porcelainRenameArrow); i >= 0 {
		p = p[i+len(porcelainRenameArrow):]
	}
	return strings.Trim(p, "\"")
}

func PorcelainOldPath(line string) string {
	if len(line) < minPorcelainLineLen {
		return ""
	}
	p := strings.TrimSpace(line[porcelainStatusPrefixLen:])
	i := strings.Index(p, porcelainRenameArrow)
	if i < 0 {
		return ""
	}
	return strings.Trim(p[:i], "\"")
}
