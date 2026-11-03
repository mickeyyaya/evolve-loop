package reviewclass

import (
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/commentaudit"
	"github.com/mickeyyaya/evolve-loop/go/internal/gateclass"
)

type Git func(args ...string) (string, error)

type ReadFile func(path string) ([]byte, error)

func nameArgs(base string) []string {
	return []string{"-c", "core.quotePath=false", "diff", "--name-only", "-z", "--no-renames", base}
}

func Read(git Git, readFile ReadFile, base string) (Diff, error) {
	in, err := gateclass.Read(git, base)
	if err != nil {
		return Diff{}, err
	}
	names, err := git(nameArgs(base)...)
	if err != nil {
		return Diff{}, err
	}
	d := Diff{Paths: append(splitNUL(names), in.Untracked...)}
	slices.Sort(d.Paths)
	d.Paths = slices.Compact(d.Paths)
	for _, f := range gateclass.Classify(in) {
		if f.Pattern == gateclass.PatternStageOffEnforce && !slices.Contains(d.StageFlips, f.Path) {
			d.StageFlips = append(d.StageFlips, f.Path)
		}
	}
	for _, p := range d.Paths {
		if provenCommentOnly(git, readFile, base, p) {
			d.CommentOnly = append(d.CommentOnly, p)
		}
	}
	return d, nil
}

func splitNUL(s string) []string {
	var out []string
	for _, p := range strings.Split(s, "\x00") {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func provenCommentOnly(git Git, readFile ReadFile, base, p string) bool {
	if !strings.HasSuffix(p, ".go") || inTestdata(p) {
		return false
	}
	before, err := git("show", base+":"+p)
	if err != nil {
		return false
	}
	after, err := readFile(p)
	if err != nil {
		return false
	}
	ok, _, err := commentaudit.Equivalent(p, []byte(before), after)
	return err == nil && ok
}
