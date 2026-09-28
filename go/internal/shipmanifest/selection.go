package shipmanifest

import (
	"context"
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

type GitRead func(args ...string) (string, error)

type Selection struct {
	Paths    []string
	Ignored  []string
	Manifest []string
	Changed  int
	ProbeErr error
}

func ReportFiles() []string {
	return []string{
		phasecontract.ArtifactName(string(cyclestate.PhaseBuild)),
		phasecontract.ArtifactName(string(cyclestate.PhaseTDD)),
	}
}

func RawPathRead(args ...string) []string {
	return append([]string{"-c", "core.quotePath=false"}, args...)
}

func Select(read GitRead, root, workspace string) (Selection, error) {
	porcelain, err := read(RawPathRead("status", "--porcelain", "-uall")...)
	if err != nil {
		return Selection{}, err
	}
	sel := Selection{Changed: len(ChangedPaths(porcelain))}
	paths := withoutGone(porcelain, ChangedPaths(porcelain))
	if workspace != "" {
		sel.Manifest = Declared(workspace, ReportFiles())
		paths = Stageable(porcelain, sel.Manifest, RegularFileIn(root))
	}
	sel.Paths, sel.Ignored, sel.ProbeErr = withoutIgnored(read, paths)
	return sel, nil
}

func withoutIgnored(read GitRead, paths []string) (kept, ignored []string, err error) {
	if len(paths) == 0 {
		return paths, nil, nil
	}
	out, err := read(RawPathRead(append([]string{"check-ignore", "--"}, paths...)...)...)
	if err != nil {
		return paths, nil, err
	}
	isIgnored := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		if p := UnquoteGitPath(strings.TrimSpace(line)); p != "" {
			isIgnored[p] = true
		}
	}
	for _, p := range paths {
		if isIgnored[p] {
			ignored = append(ignored, p)
		} else {
			kept = append(kept, p)
		}
	}
	return kept, ignored, nil
}

func GitIn(ctx context.Context, dir string) GitRead {
	g := gitexec.Default(dir)
	return func(args ...string) (string, error) {
		out, stderr, code, err := g.Capture(ctx, append([]string{"--no-optional-locks"}, args...)...)
		if err == nil && code > 1 {
			err = fmt.Errorf("git %s exited %d: %s", strings.Join(args, " "), code, strings.TrimSpace(stderr))
		}
		return out, err
	}
}
