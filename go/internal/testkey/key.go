package testkey

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/overlap"
)

func Compute(in Input) (Key, error) {
	pkgs, err := closure(in)
	if err != nil {
		return Key{}, err
	}
	if err := in.Toolchain.check(); err != nil {
		return Key{}, err
	}
	s := newScope(in, pkgs)
	reads, seen := observe(Context{RepoRoot: in.RepoRoot, WorkDir: in.WorkDir, TempDirs: slices.Clone(in.TempDirs)}, in.Log)
	whole := in.TreeReading || in.Log.Unknown || in.unresolved(pkgs) || outside(s, seen) || in.symlinked(s)
	names := in.envNames()
	e := newEncoder()
	e.field("package", in.Package)
	in.Toolchain.encode(e)
	e.list("envnames", names)
	if whole {
		err = encodeWhole(e, in)
	} else {
		err = encodeClosure(e, in, pkgs)
	}
	if err != nil {
		return Key{}, err
	}
	return seal(e.sum(), whole, in.envValues(names), reads), nil
}

func fullKey(file string, env []EnvValue) string {
	e := newEncoder()
	e.field("file", file)
	for _, v := range env {
		e.field(v.Name, v.Value)
		e.field("set", fmt.Sprint(v.Set))
	}
	return e.sum()
}

func seal(file string, whole bool, env []EnvValue, reads Reads) Key {
	return Key{File: file, Full: fullKey(file, env), Whole: whole, Env: env, Reads: reads}
}

func (k Key) Valid() bool {
	return k.Full == fullKey(k.File, k.Env)
}

func (t Toolchain) encode(e *encoder) {
	e.field("goversion", t.GoVersion)
	for _, name := range BuildEnvNames() {
		e.field("buildenv:"+name, t.BuildEnv[name])
	}
	e.list("tags", sortedCopy(t.Tags))
	e.list("flags", t.Flags)
}

func encodeWhole(e *encoder, in Input) error {
	if in.WholeTreeID == "" {
		return fmt.Errorf("testkey: %s keys on the whole tree but the whole tree id is empty", in.Package)
	}
	e.field("whole", in.WholeTreeID)
	return nil
}

func encodeClosure(e *encoder, in Input, pkgs []overlap.Package) error {
	for _, f := range []string{goModPath, goSumPath} {
		if err := encodeID(e, in.IDs, f); err != nil {
			return err
		}
	}
	roots := map[string]bool{}
	for _, p := range pkgs {
		e.field("pkg", p.ImportPath)
		for _, f := range sortedCopy(p.Files) {
			if err := encodeID(e, in.IDs, f); err != nil {
				return err
			}
		}
		if in.TestdataIDs[p.Dir] == "" {
			return fmt.Errorf("testkey: no testdata tree id for %s", p.Dir)
		}
		e.field("testdata:"+p.Dir, in.TestdataIDs[p.Dir])
		for _, r := range in.DataRoots[p.ImportPath] {
			roots[r] = true
		}
	}
	return encodeRoots(e, in.RootIDs, roots)
}

func encodeRoots(e *encoder, ids map[string]string, roots map[string]bool) error {
	names := make([]string, 0, len(roots))
	for r := range roots {
		names = append(names, r)
	}
	for _, r := range sortedCopy(names) {
		if err := encodeID(e, ids, r); err != nil {
			return err
		}
	}
	return nil
}

func encodeID(e *encoder, ids map[string]string, name string) error {
	id, ok := ids[name]
	if !ok {
		return fmt.Errorf("testkey: no content id for %s", name)
	}
	e.field(name, id)
	return nil
}

func (in Input) unresolved(pkgs []overlap.Package) bool {
	return slices.ContainsFunc(pkgs, func(p overlap.Package) bool {
		return slices.Contains(in.UnresolvedRoots, p.ImportPath)
	})
}

func (in Input) envNames() []string {
	set := map[string]bool{}
	for _, n := range append(slices.Clone(in.Log.Getenv), in.ChildVars...) {
		set[n] = true
	}
	names := make([]string, 0, len(set))
	for n := range set {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func (in Input) envValues(names []string) []EnvValue {
	out := make([]EnvValue, 0, len(names))
	for _, n := range names {
		v, set := in.Env[n]
		out = append(out, EnvValue{Name: n, Value: v, Set: set})
	}
	return out
}

func sortedCopy(xs []string) []string {
	out := slices.Clone(xs)
	sort.Strings(out)
	return out
}

var buildEnvNames = []string{
	"GOOS", "GOARCH", "GOFLAGS", "GOEXPERIMENT", "CGO_ENABLED", "CC", "CXX",
	"CGO_CFLAGS", "CGO_CXXFLAGS", "CGO_LDFLAGS", "GOAMD64", "GOARM", "GO386", "GOARM64", "GOWASM",
}

func BuildEnvNames() []string {
	return slices.Clone(buildEnvNames)
}

func (t Toolchain) check() error {
	for _, name := range buildEnvNames {
		if _, ok := t.BuildEnv[name]; !ok {
			return fmt.Errorf("testkey: the build environment has no value for %s", name)
		}
	}
	return checkShuffle(t.Flags)
}

func checkShuffle(flags []string) error {
	for i, flag := range flags {
		name, value, found := strings.Cut(strings.TrimLeft(flag, "-"), "=")
		if name != "shuffle" && name != "test.shuffle" {
			continue
		}
		if !found && i+1 < len(flags) {
			value = flags[i+1]
		}
		if _, err := strconv.ParseInt(value, 10, 64); err != nil && value != "off" {
			return fmt.Errorf("testkey: the shuffle flag %q has no fixed seed, so equal keys can run different orders", flag)
		}
	}
	return nil
}

func (in Input) symlinked(s scope) bool {
	return slices.ContainsFunc(in.Symlinks, func(p string) bool { return s.covers(p, "stat") })
}
