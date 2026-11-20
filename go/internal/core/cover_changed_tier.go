package core

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/covchanged"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

type baseNames struct {
	read  func(file string) ([]byte, error)
	warn  io.Writer
	cache map[string]map[string]bool
}

func (b *baseNames) has(f covchanged.Func) bool {
	names, seen := b.cache[f.File]
	if !seen {
		names = b.declared(f.File)
		if b.cache == nil {
			b.cache = map[string]map[string]bool{}
		}
		b.cache[f.File] = names
	}
	return names[f.Name]
}

func (b *baseNames) declared(file string) map[string]bool {
	data, err := b.read(file)
	if err == nil {
		var names map[string]bool
		if names, err = covchanged.DeclaredFuncs(file, data); err == nil {
			return names
		}
	}
	fmt.Fprintf(b.warn, "[build-floor] WARN cover-changed could not read %s at the base (%v); each function of it counts as new\n", file, err)
	return nil
}

func gitBaseReader(ctx context.Context, in ReviewInput, prefix string) func(file string) ([]byte, error) {
	git := gitexec.Git{Dir: in.Worktree, Exec: gitRunner}
	base := floorBase(in)
	return func(file string) ([]byte, error) {
		out, err := git.Output(ctx, "show", base+":"+path.Join(prefix, file))
		return []byte(out), err
	}
}

func splitNotMeasured(refused []covchanged.Func, atBase func(covchanged.Func) bool) (kept, unmeasured []covchanged.Func) {
	for _, f := range refused {
		if ciparity.EnvExclusivePkg(path.Dir(f.File)) && f.Profiled && f.Covered == 0 && atBase(f) {
			unmeasured = append(unmeasured, f)
			continue
		}
		kept = append(kept, f)
	}
	return kept, unmeasured
}

func (in ReviewInput) noteNotMeasured(unmeasured []covchanged.Func, prefix string) {
	for _, f := range unmeasured {
		tag, _ := ciparity.EnvExclusiveTagOf(path.Dir(f.File))
		line := fmt.Sprintf("[build-floor] WARN cover-changed: %s is not measured at the floor: no test in the floor build covers it; the %s tier did not run; CI judges it", f, tag)
		fmt.Fprintln(in.errOut(), line)
		in.residuals.add(FloorResidual{Class: FloorResidualCoverChanged, Failure: line, Source: path.Join(prefix, f.File), Line: f.Line, Func: f.Key})
	}
}

func noteCoverEnvExclusive(pkgs []string, warn io.Writer) {
	var tiered []string
	for _, p := range pkgs {
		if ciparity.EnvExclusivePkg(p) {
			tiered = append(tiered, p)
		}
	}
	if len(tiered) > 0 {
		fmt.Fprintf(warn, "[build-floor] NOTE: env-exclusive package(s) %s run in the cover run under -tags %s, which leaves their tmux tier out of the build; an old changed function at zero statements is listed as not measured. Backstop: %s\n",
			strings.Join(tiered, ", "), strings.Join(ciparity.EnvExclusiveTags(), ","), ciparity.EnvExclusiveBackstopNote(tiered))
	}
}
