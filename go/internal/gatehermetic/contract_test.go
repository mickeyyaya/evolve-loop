package gatehermetic

import (
	"context"
	"go/types"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/goload"
)

func TestContract_HoldsTheRefusedListAndTheClosedAllowedList(t *testing.T) {
	t.Parallel()
	var c Rules = Contract()

	if !slices.Equal(c.Exempt, []string{sysexecPath}) || !slices.Equal(c.Boundary, []string{repostatePath, gotoolPath}) {
		t.Errorf("exempt = %v boundary = %v", c.Exempt, c.Boundary)
	}
	if want := []Command{{Name: "go", Verb: "list"}, {Name: "go", Verb: "test"}}; !slices.Equal(c.Allowed, want) {
		t.Errorf("allowed = %v, want %v", c.Allowed, want)
	}
	if want := (ToolchainFunc{Package: gotoolPath, Name: "command", Verb: "verb"}); c.Toolchain != want {
		t.Errorf("toolchain = %+v", c.Toolchain)
	}
	for _, name := range []string{sysexecPath + ".WithEnv", sysexecPath + ".CancelKillsTheProcessGroup", "go/build.ImportDir", "go/build.MatchFile"} {
		if !slices.Contains(c.Inert, name) {
			t.Errorf("inert lacks %s: %v", name, c.Inert)
		}
	}
	if want := []string{"go/build", "go/importer", "net/http/cgi", "net/http/fcgi"}; !slices.Equal(c.ExecStd, want) {
		t.Errorf("exec std = %v, want %v", c.ExecStd, want)
	}
	for _, e := range []ExecEntry{
		{Package: "os/exec", Name: "Command", Param: "name"},
		{Package: "os/exec", Name: "LookPath", Param: "file"},
		{Package: sysexecPath, Name: "RunFunc", Param: "name"},
		{Package: "go/build", Name: "Context.Import"},
		{Package: "os", Name: "StartProcess"},
	} {
		if !slices.Contains(c.Exec, e) {
			t.Errorf("exec entries lack %+v", e)
		}
	}
	if !slices.Contains(c.Mtime, "syscall.Stat") || !slices.Contains(c.History, "runtime/debug.ReadBuildInfo") || !slices.Contains(c.History, "debug/buildinfo.ReadFile") || !slices.Contains(c.History, "debug/buildinfo.Read") || len(c.GitWords) != 7 {
		t.Errorf("mtime = %v history = %v words = %v", c.Mtime, c.History, c.GitWords)
	}
}

func importable(path string) bool {
	return !slices.Contains(strings.Split(path, "/"), "internal") && !strings.HasPrefix(path, "vendor/")
}

func TestContract_EachStdPackageWhoseImportsHoldOsExecIsJudged(t *testing.T) {
	t.Parallel()
	prog, err := realLoader.Load(context.Background(), goload.Request{Dir: "../..", Patterns: []string{"std"}, Env: os.Environ()})
	if err != nil {
		t.Fatal(err)
	}
	rules := Contract()
	derived := execClosure(prog)

	var unjudged []string
	for path := range derived {
		entry := slices.ContainsFunc(rules.Exec, func(e ExecEntry) bool { return e.Package == path })
		if importable(path) && !entry && !slices.Contains(rules.ExecStd, path) {
			unjudged = append(unjudged, path)
		}
	}
	slices.Sort(unjudged)

	if len(unjudged) != 0 {
		t.Errorf("std packages that reach os/exec and no judgement names: %v", unjudged)
	}
	for _, path := range rules.ExecStd {
		if !derived[path] {
			t.Errorf("ExecStd names %s, which no longer reaches os/exec", path)
		}
	}
	if derived["os/exec"] || !derived["go/importer"] {
		t.Errorf("os/exec = %v go/importer = %v", derived["os/exec"], derived["go/importer"])
	}
}

func TestContract_TheInertListNamesOnlyFunctionsOfAPackageThatReachesOsExec(t *testing.T) {
	t.Parallel()
	for _, name := range Contract().Inert {
		pkg := name[:strings.LastIndex(name, ".")]
		if strings.HasPrefix(pkg, modulePath) {
			continue
		}
		if !slices.Contains(Contract().ExecStd, pkg) {
			t.Errorf("inert %s names a package that ExecStd lacks", name)
		}
	}
}

func TestExecClosure_FollowsImportsTransitivelySkipsUnlistedPackagesAndSurvivesACycle(t *testing.T) {
	t.Parallel()
	pkg := func(path string, standard bool, imports ...*types.Package) *goload.Package {
		p := types.NewPackage(path, path)
		p.SetImports(imports)
		return &goload.Package{Path: path, Standard: standard, Types: p}
	}
	osExec := pkg("os/exec", true)
	direct := pkg("direct", true, osExec.Types)
	via := pkg("via", true, direct.Types)
	plain := pkg("plain", true, types.NewPackage("ghost", "ghost"))
	loop := pkg("loop", true)
	loop.Types.SetImports([]*types.Package{loop.Types})
	module := pkg("module", false, osExec.Types)
	prog := goload.Program{Packages: map[string]*goload.Package{}}
	for _, p := range []*goload.Package{osExec, direct, via, plain, loop, module} {
		prog.Packages[p.Path] = p
	}

	got := execClosure(prog)

	if want := map[string]bool{"direct": true, "via": true}; len(got) != 2 || !got["direct"] || !got["via"] {
		t.Errorf("closure = %v, want %v", got, want)
	}
}
