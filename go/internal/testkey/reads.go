package testkey

import (
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

type observed struct {
	Kind   string
	Path   string
	Inside bool
}

func (c Context) underTemp(abs string) bool {
	return slices.ContainsFunc(c.TempDirs, func(dir string) bool {
		return abs == dir || strings.HasPrefix(abs, dir+"/")
	})
}

func (c Context) repoRelative(abs string) (string, bool) {
	rel, err := filepath.Rel(c.RepoRoot, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

func observe(c Context, log Log) (Reads, []observed) {
	var seen []observed
	cwd := c.WorkDir
	for _, op := range log.Ops {
		abs := op.Path
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(cwd, abs)
		}
		abs = filepath.Clean(abs)
		if op.Kind == "chdir" {
			cwd = abs
		}
		if c.underTemp(abs) {
			continue
		}
		rel, inside := c.repoRelative(abs)
		if !inside {
			rel = abs
		}
		seen = append(seen, observed{Kind: op.Kind, Path: rel, Inside: inside})
	}
	reads := Reads{Context: c, Unknown: log.Unknown}
	for _, o := range seen {
		reads.Opens = append(reads.Opens, o.Kind+" "+o.Path)
	}
	reads.Opens = uniqueSorted(reads.Opens)
	return reads, seen
}

func uniqueSorted(xs []string) []string {
	out := slices.Clone(xs)
	sort.Strings(out)
	return slices.Compact(out)
}

func outside(s scope, seen []observed) bool {
	return slices.ContainsFunc(seen, func(o observed) bool { return !o.Inside || !s.covers(o.Path, o.Kind) })
}

func (k Key) Consistent(log Log) bool {
	reads, _ := observe(k.Reads.Context, log)
	if !slices.Equal(reads.Opens, k.Reads.Opens) || reads.Unknown != k.Reads.Unknown {
		return false
	}
	if reads.Unknown && !k.Whole {
		return false
	}
	return !slices.ContainsFunc(uniqueSorted(log.Getenv), func(n string) bool {
		return !slices.ContainsFunc(k.Env, func(v EnvValue) bool { return v.Name == n })
	})
}
