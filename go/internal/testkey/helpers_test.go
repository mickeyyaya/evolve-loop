package testkey

import (
	"maps"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/overlap"
)

const (
	modulePath = "example.com/m"
	repoRoot   = "/repo"
)

func importPath(name string) string { return modulePath + "/internal/" + name }

func srcFile(name string) string { return "go/internal/" + name + "/" + name + ".go" }

func pkg(name string, deps ...string) overlap.Package {
	full := make([]string, 0, len(deps))
	for _, d := range deps {
		full = append(full, importPath(d))
	}
	return overlap.Package{
		ImportPath: importPath(name),
		Dir:        "go/internal/" + name,
		Files:      []string{srcFile(name), "go/internal/" + name + "/" + name + "_test.go"},
		Deps:       full,
	}
}

func fixtureModule() overlap.Module {
	return overlap.Module{
		Path: modulePath,
		Packages: []overlap.Package{
			pkg("a", "b"),
			pkg("b"),
			pkg("c"),
			pkg("h", "c"),
		},
	}
}

func fixtureIDs(m overlap.Module) map[string]string {
	ids := map[string]string{"go/go.mod": "id-go.mod", "go/go.sum": "id-go.sum"}
	for _, p := range m.Packages {
		for _, f := range p.Files {
			ids[f] = "id-" + f
		}
	}
	return ids
}

func baseInput() Input {
	m := fixtureModule()
	return Input{
		Package: importPath("a"),
		Module:  m,
		Toolchain: Toolchain{
			GoVersion: "go1.23.4",
			Tags:      []string{"integration"}, Flags: []string{"-count=1"},
			BuildEnv: fixtureBuildEnv(),
		},
		IDs:         fixtureIDs(m),
		TestdataIDs: fixtureTestdata(m),
		RootIDs:     map[string]string{},
		WholeTreeID: "tree-1",
		Log:         ParseLog("# test log\nopen /repo/go/internal/a/testdata/x.json\n"),
		RepoRoot:    repoRoot,
		WorkDir:     "/repo/go/internal/a",
		TempDirs:    []string{"/tmp/run-1"},
		Env:         map[string]string{},
	}
}

func mustCompute(t *testing.T, in Input) Key {
	t.Helper()
	k, err := Compute(in)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	return k
}

func withIDs(in Input, edits map[string]string) Input {
	in.IDs = maps.Clone(in.IDs)
	maps.Copy(in.IDs, edits)
	return in
}

func withEnv(in Input, edits map[string]string) Input {
	in.Env = maps.Clone(in.Env)
	maps.Copy(in.Env, edits)
	return in
}

func withRoot(in Input, pkgName, root, id string) Input {
	in.DataRoots = map[string][]string{importPath(pkgName): {root}}
	in.RootIDs = map[string]string{root: id}
	return in
}

func assertChanged(t *testing.T, before, after Key) {
	t.Helper()
	if before.Full == after.Full {
		t.Fatalf("key stayed %s, want it to change", before.Full)
	}
}

func assertSame(t *testing.T, before, after Key) {
	t.Helper()
	if before.Full != after.Full || before.File != after.File {
		t.Fatalf("key changed from %+v to %+v, want it to stay", before, after)
	}
}

func fixtureBuildEnv() map[string]string {
	env := map[string]string{}
	for _, name := range BuildEnvNames() {
		env[name] = "v-" + name
	}
	return env
}

func fixtureTestdata(m overlap.Module) map[string]string {
	ids := map[string]string{}
	for _, p := range m.Packages {
		ids[p.Dir] = "td-" + p.Dir
	}
	return ids
}

func withBuildEnv(in Input, name, value string) Input {
	in.Toolchain.BuildEnv = maps.Clone(in.Toolchain.BuildEnv)
	in.Toolchain.BuildEnv[name] = value
	return in
}
