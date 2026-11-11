package gatehermetic

import (
	"context"
	"slices"
	"strings"
	"testing"
)

const sx = `"` + sysexecPath + `"`

const initEdge = "init"

func gatesFile(imports, body string) map[string]string {
	return map[string]string{"internal/gates/gates.go": "package gates\n\nimport (\n" + imports + "\n)\n\n" + body}
}

func byRoot(rs []Refusal) map[string][]Refusal {
	out := map[string][]Refusal{}
	for _, r := range rs {
		out[r.Root.Func] = append(out[r.Root.Func], r)
	}
	return out
}

func rootsOf(fns ...string) []Root {
	var out []Root
	for _, f := range fns {
		out = append(out, gateRoot("gates", f))
	}
	return out
}

func TestRefusals_ChainsAHelperGitCallWithTheEdgeKindOfEachStep(t *testing.T) {
	t.Parallel()
	files := map[string]string{
		"internal/gates/gates.go":   "package gates\n\nimport \"" + modulePath + "/internal/helper\"\n\nfunc Run() { helper.Mid() }\n",
		"internal/helper/helper.go": "package helper\n\nimport \"os/exec\"\n\nfunc Mid() { leaf() }\n\nfunc leaf() { _ = exec.Command(\"git\", \"log\").Run() }\n",
	}

	got := refuseFixture(t, files, gateRoot("gates", "Run"))

	if len(got) != 1 || got[0].Class != "git" || !strings.Contains(got[0].Site, "helper.go:7:") {
		t.Fatalf("refusals = %+v", got)
	}
	if !slices.Equal(edges(got[0]), []string{"", "call", "call"}) || got[0].Root.Func != "Run" {
		t.Errorf("edges = %v root = %+v", edges(got[0]), got[0].Root)
	}
	if first := (Step{Func: modulePath + "/internal/gates.Run"}); got[0].Chain[0] != first {
		t.Errorf("root step = %+v, want %+v", got[0].Chain[0], first)
	}
	if want := "helper.Mid > helper.leaf"; !strings.Contains(lastFuncs(got[0]), want) {
		t.Errorf("chain = %s, want it to hold %s", lastFuncs(got[0]), want)
	}
}

func TestRefusals_GivesGitAndMtimeForThePreG2TestdupShape(t *testing.T) {
	t.Parallel()
	files := gatesFile(`"context"
	"os"
	`+sx, `func gitLines(ctx context.Context, run sysexec.RunFunc) {
	_, _ = sysexec.Output(ctx, run, "", "git", "log", "--format=@%ct")
}

func isOld(dir string) bool {
	fi, err := os.Stat(dir)
	return err == nil && fi.ModTime().IsZero()
}

func Scan(ctx context.Context, run sysexec.RunFunc) bool {
	gitLines(ctx, run)
	return isOld(".")
}
`)

	got := refuseFixture(t, files, gateRoot("gates", "Scan"))

	wantClasses(t, got, "git", "mtime")
}

func TestRefusals_FollowsMethodValuesRunFuncParametersAndPackageVarInitializers(t *testing.T) {
	t.Parallel()
	files := gatesFile(`"context"
	`+sx+`
	"`+modulePath+`/internal/cachepkg"`, `type G struct{ run sysexec.RunFunc }

func (g G) log(ctx context.Context) {
	_, _ = g.run(ctx, "git", "", nil, nil, nil, nil, nil)
}

func ViaMethodValue(ctx context.Context) {
	f := G{}.log
	f(ctx)
}

func ViaParameter(ctx context.Context, run sysexec.RunFunc) {
	_, _ = run(ctx, "git", "", nil, nil, nil, nil, nil)
}

func ViaInitializer() int { return cachepkg.Cached }
`)

	files["internal/cachepkg/cachepkg.go"] = "package cachepkg\n\nimport \"os/exec\"\n\nfunc readGit() int {\n\t_ = exec.Command(\"git\", \"status\")\n\treturn 1\n}\n\nvar Cached = readGit()\n"

	by := byRoot(refuseFixture(t, files, rootsOf("ViaMethodValue", "ViaParameter", "ViaInitializer")...))

	for _, name := range []string{"ViaMethodValue", "ViaParameter", "ViaInitializer"} {
		if len(by[name]) != 1 || by[name][0].Class != "git" {
			t.Errorf("%s = %+v", name, by[name])
		}
	}
	if got := edges(by["ViaMethodValue"][0]); !slices.Equal(got, []string{"", "value"}) {
		t.Errorf("method value edges = %v", got)
	}
	if got := edges(by["ViaInitializer"][0]); !slices.Equal(got, []string{"", initEdge, "call"}) {
		t.Errorf("initializer edges = %v", got)
	}
}

func TestRefusals_ReachesTheMethodOfALiveTypeThatSortSortCalls(t *testing.T) {
	t.Parallel()
	files := gatesFile(`"os/exec"
	"sort"`, `type byGit []string

func (b byGit) Len() int {
	_ = exec.Command("git", "log")
	return len(b)
}

func (b byGit) Less(i, j int) bool { return false }

func (b byGit) Swap(i, j int) {}

func Run() { sort.Sort(byGit(nil)) }
`)

	got := refuseFixture(t, files, gateRoot("gates", "Run"))

	if len(got) != 1 || got[0].Class != "git" || !slices.Equal(edges(got[0]), []string{"", "live-type"}) {
		t.Fatalf("refusals = %+v", got)
	}
}

func TestRefusals_RefusesAGitDirSegmentFoldedFromConstants(t *testing.T) {
	t.Parallel()
	files := gatesFile(`"path/filepath"`, `const dot = ".git"

func Folded(root string) string { return filepath.Join(root, ".gi"+"t", "HEAD") }

func Named(root string) string { return filepath.Join(root, dot) }

func Nested(root string) string { return root + "/" + (".git") + "/x" }

func Ignore(root string) string { return filepath.Join(root, ".gitignore") }
`)

	by := byRoot(refuseFixture(t, files, rootsOf("Folded", "Named", "Nested", "Ignore")...))

	for _, name := range []string{"Folded", "Named", "Nested"} {
		if len(by[name]) != 1 || by[name][0].Class != "git-dir" {
			t.Errorf("%s = %+v", name, by[name])
		}
	}
	if len(by["Ignore"]) != 0 {
		t.Errorf("Ignore = %+v", by["Ignore"])
	}
}

func TestRefusals_JudgesTheExecEntriesAndLetsTheBoundaryPass(t *testing.T) {
	t.Parallel()
	files := gatesFile(`"context"
	"go/build"
	"os/exec"
	`+sx+`
	"`+modulePath+`/internal/gotool"`, `func ListDirect(ctx context.Context, run sysexec.RunFunc) {
	_, _ = sysexec.Output(ctx, run, "", "go", "list")
}

func ViaGotool(ctx context.Context) { _ = gotool.List(ctx, gotool.Call{}) }

func GoRun() { _ = exec.Command("go", "run", "x.go") }

func ShC() { _ = exec.Command("sh", "-c", "git log") }

func EnvGit() { _ = exec.Command("env", "git", "log") }

func AbsGit() { _ = exec.Command("/usr/bin/git", "log") }

func VarProg(name string) { _ = exec.Command(name) }

func Lookup() { _, _ = exec.LookPath("git") }

func Unlisted() { sysexec.Unlisted() }

func Inert(run sysexec.RunFunc, cmd *exec.Cmd) {
	_ = sysexec.WithEnv(run, nil)
	sysexec.CancelKillsTheProcessGroup(cmd)
}

func ImportFn() { _, _ = build.Import("x", "", 0) }

func ImportMethod() { _, _ = build.Default.Import("x", "", 0) }

func Generic(cmd *exec.Cmd) {
	cmd.Dir = "x"
}
`)

	by := byRoot(refuseFixture(t, files, rootsOf("ListDirect", "ViaGotool", "GoRun", "ShC", "EnvGit", "AbsGit", "VarProg", "Lookup", "Unlisted", "Inert", "ImportFn", "ImportMethod", "Generic")...))

	want := map[string]string{
		"ListDirect": "exec", "GoRun": "exec", "ShC": "exec", "EnvGit": "exec", "AbsGit": "exec",
		"VarProg": "exec", "Lookup": "git", "Unlisted": "exec", "ImportFn": "exec", "ImportMethod": "exec",
	}
	for name, class := range want {
		if len(by[name]) != 1 || by[name][0].Class != class {
			t.Errorf("%s = %+v, want one %s", name, by[name], class)
		}
	}
	for _, name := range []string{"ViaGotool", "Inert", "Generic"} {
		if len(by[name]) != 0 {
			t.Errorf("%s = %+v, want none", name, by[name])
		}
	}
}

func TestRefusals_PassesTheBoundaryAndRefusesItsSibling(t *testing.T) {
	t.Parallel()
	files := gatesFile(`"context"
	"`+modulePath+`/internal/repostate"
	"`+modulePath+`/internal/sibling"`, `func ViaBoundary(ctx context.Context) { repostate.Log(ctx) }

func ViaSibling(ctx context.Context) { sibling.Log(ctx) }
`)
	files["internal/sibling/sibling.go"] = strings.Replace(stubFiles["internal/repostate/repostate.go"], "package repostate", "package sibling", 1)

	by := byRoot(refuseFixture(t, files, rootsOf("ViaBoundary", "ViaSibling")...))

	if len(by["ViaBoundary"]) != 0 || len(by["ViaSibling"]) != 1 || by["ViaSibling"][0].Class != "git" {
		t.Errorf("refusals = %+v", by)
	}
}

func TestRefusals_ReachesTheMethodOfAFieldTypeThatJSONMarshalCalls(t *testing.T) {
	t.Parallel()
	files := gatesFile(`"encoding/json"
	"os/exec"`, `type Inner struct{}

func (Inner) MarshalJSON() ([]byte, error) {
	_ = exec.Command("git", "log")
	return []byte("1"), nil
}

type Outer struct{ I Inner }

func Run() { _, _ = json.Marshal(Outer{}) }
`)

	got := refuseFixture(t, files, gateRoot("gates", "Run"))

	if len(got) != 1 || got[0].Class != "git" || !slices.Equal(edges(got[0]), []string{"", "live-type"}) {
		t.Fatalf("refusals = %+v", got)
	}
}

func TestRefusals_ReachesInitFunctionsAndStoredFunctionValuesOnlyThroughAReachedRead(t *testing.T) {
	t.Parallel()
	files := map[string]string{
		"internal/gates/gates.go": `package gates

import (
	"` + modulePath + `/internal/readinit"
	"` + modulePath + `/internal/stored"
)

func ReadsInit() int { return readinit.Table }

func NoRead() func() { return stored.Plain() }

func ReadsStored() func() { return stored.Hook }
`,
		"internal/readinit/readinit.go": "package readinit\n\nimport \"os/exec\"\n\nvar Table = 1\n\nfunc init() { _ = exec.Command(\"git\", \"status\") }\n",
		"internal/stored/stored.go":     "package stored\n\nimport \"os/exec\"\n\nfunc gitRead() { _ = exec.Command(\"git\", \"log\") }\n\nvar Hook = gitRead\n\nfunc Plain() func() { return nil }\n",
	}

	by := byRoot(refuseFixture(t, files, rootsOf("ReadsInit", "NoRead", "ReadsStored")...))

	if len(by["ReadsInit"]) != 1 || !slices.Equal(edges(by["ReadsInit"][0]), []string{"", initEdge}) {
		t.Errorf("ReadsInit = %+v", by["ReadsInit"])
	}
	if len(by["NoRead"]) != 0 {
		t.Errorf("NoRead = %+v", by["NoRead"])
	}
	if len(by["ReadsStored"]) != 1 || !slices.Equal(edges(by["ReadsStored"][0]), []string{"", "value", "value"}) {
		t.Errorf("ReadsStored = %+v", by["ReadsStored"])
	}
}

func TestRefusals_ClassifiesAReadOfAnAssignedFunctionVariableAsInject(t *testing.T) {
	t.Parallel()
	files := gatesFile(`"sort"
	"`+modulePath+`/internal/reg"`, `var hook func()

var assigned func()

var counts []int

func Set(f func()) { assigned = f }

func Count() { counts = append(counts, 1) }

func Bump() { counts[0]++ }

func Pass() {
	if hook != nil {
		hook()
	}
}

func Read() { assigned() }

func ViaMap() { reg.Registry["a"]() }

func Plain() int { return len(counts) + len(sort.IntSlice{}) }
`)

	files["internal/reg/reg.go"] = "package reg\n\nvar Registry = map[string]func(){}\n\nfunc init() { Registry[\"a\"] = func() {} }\n"

	by := byRoot(refuseFixture(t, files, rootsOf("Pass", "Read", "ViaMap", "Plain", "Set")...))

	if len(by["Pass"]) != 0 || len(by["Plain"]) != 0 || len(by["Set"]) != 0 {
		t.Errorf("Pass = %+v Plain = %+v Set = %+v", by["Pass"], by["Plain"], by["Set"])
	}
	for _, name := range []string{"Read", "ViaMap"} {
		if len(by[name]) == 0 || by[name][0].Class != "inject" {
			t.Errorf("%s = %+v", name, by[name])
		}
	}
}

func TestRefusals_RefusesAPackageWithAnExcludedFileOrABodylessFunction(t *testing.T) {
	t.Parallel()
	files := map[string]string{
		"internal/gates/gates.go":   "package gates\n\nimport (\n\t\"" + modulePath + "/internal/tagged\"\n\t\"" + modulePath + "/internal/clean\"\n\t\"" + modulePath + "/internal/asm\"\n)\n\nfunc Tagged() int { return tagged.A() }\n\nfunc Clean() int { return clean.C() }\n\nfunc Asm() int { return asm.Other() }\n",
		"internal/tagged/a.go":      "package tagged\n\nfunc A() int { return 1 }\n",
		"internal/clean/clean.go":   "package clean\n\nfunc C() int { return 1 }\n",
		"internal/clean/x_test.go":  "//go:build never\n\npackage clean\n",
		"internal/tagged/b.go":      "//go:build never\n\npackage tagged\n\nfunc B() int { return 2 }\n",
		"internal/asm/asm.go":       "package asm\n\nfunc stub() int\n\nfunc Other() int { return 1 }\n",
		"internal/asm/asm_other.go": "//go:build never\n\npackage asm\n",
	}

	by := byRoot(refuseFixture(t, files, rootsOf("Tagged", "Asm", "Clean")...))

	if len(by["Tagged"]) != 1 || by["Tagged"][0].Class != "exec" || !strings.HasSuffix(by["Tagged"][0].Site, "b.go:1:1") {
		t.Errorf("Tagged = %+v", by["Tagged"])
	}
	if len(by["Clean"]) != 0 {
		t.Errorf("Clean = %+v, want none: a tagged test file is not an excluded source file", by["Clean"])
	}
	if len(by["Asm"]) != 2 {
		t.Errorf("Asm = %+v", by["Asm"])
	}
}

func TestRefusals_ReachesAGenericFunctionThroughItsInstance(t *testing.T) {
	t.Parallel()
	files := gatesFile(`"os/exec"`, `func Wrap[T any](v T) T {
	_ = exec.Command("git", "log")
	return v
}

func Run() int { return Wrap[int](1) }

func Infer() string { return Wrap("a") }
`)

	by := byRoot(refuseFixture(t, files, rootsOf("Run", "Infer")...))

	for _, name := range []string{"Run", "Infer"} {
		if len(by[name]) != 1 || by[name][0].Class != "git" || !slices.Equal(edges(by[name][0]), []string{"", "call"}) {
			t.Errorf("%s = %+v", name, by[name])
		}
	}
}

func TestRefusals_AnExecEntryWithAMissingParameterIsAnError(t *testing.T) {
	t.Parallel()
	g := loadFixture(t, gatesFile(`"os/exec"`, "func Run() { _ = exec.Command(\"git\") }\n"), gateRoot("gates", "Run"))
	rules := Contract()
	rules.Exec = append(slices.Clone(rules.Exec), ExecEntry{Package: "os/exec", Name: "Command", Param: "nope"})

	got, err := Refusals(g, rules)

	if err == nil || !strings.Contains(err.Error(), `"nope"`) || got != nil {
		t.Errorf("got = %+v err = %v", got, err)
	}
}

func realTreeRefusals(t *testing.T, pkg, fn string) []Refusal {
	t.Helper()
	g, err := Load(context.Background(), realLoader, Source{ModuleDir: "../..", Roots: []Root{{Package: modulePath + "/internal/" + pkg, Func: fn}}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := Refusals(g, Contract())
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestRefusals_ARealRootThatOnlyMentionsAGitReaderTypeIsNotRefusedAndARealCallIs(t *testing.T) {
	t.Parallel()
	if got := realTreeRefusals(t, "gitexec", "Default"); len(got) != 0 {
		t.Errorf("Default = %+v, want none", got)
	}
	var found bool
	for _, r := range realTreeRefusals(t, "testdup", "Scan") {
		found = found || r.Class == "git"
	}
	if !found {
		t.Error("testdup.Scan has no git refusal")
	}
}

func TestRefusals_AReferenceToDefaultRunnerPassesAndACallOfItsVariableIsExec(t *testing.T) {
	t.Parallel()
	files := gatesFile(`"context"
	`+sx, `var run sysexec.RunFunc = sysexec.DefaultRunner

func Ref() { _ = run }

func Call(ctx context.Context) { _, _ = run(ctx, "go", "", nil, nil, nil, nil, nil) }
`)

	by := byRoot(refuseFixture(t, files, rootsOf("Ref", "Call")...))

	if len(by["Ref"]) != 0 || len(by["Call"]) != 1 || by["Call"][0].Class != "exec" {
		t.Errorf("refusals = %+v", by)
	}
}

func TestRefusals_RefusesAThirdPartyBodyAndThePackageClassesOfTheCmd(t *testing.T) {
	t.Parallel()
	files := map[string]string{
		"go.mod":                  "module " + modulePath + "\n\ngo 1.23\n\nrequire example.com/dep v0.0.0\n\nreplace example.com/dep => ./dep\n",
		"dep/go.mod":              "module example.com/dep\n\ngo 1.23\n",
		"dep/dep.go":              "package dep\n\nimport \"os/exec\"\n\nvar Cache = 1\n\nfunc Git() { _ = exec.Command(\"git\", \"log\") }\n\nfunc init() { _ = exec.Command(\"git\", \"init\") }\n",
		"internal/gates/gates.go": "package gates\n\nimport \"example.com/dep\"\n\nfunc Run() { dep.Git() }\n",
	}

	got := refuseFixture(t, files, gateRoot("gates", "Run"))

	if len(got) != 2 || got[0].Class != "git" || !strings.Contains(got[0].Site, "dep.go") {
		t.Errorf("refusals = %+v", got)
	}
}

func TestRefusals_JudgesHistoryMtimeAndTheFieldsOfExecCmd(t *testing.T) {
	t.Parallel()
	files := gatesFile(`"io/fs"
	"os"
	"os/exec"
	"runtime/debug"`, `func History() { _, _ = debug.ReadBuildInfo() }

func Format(fi fs.FileInfo) string { return fs.FormatFileInfo(fi) }

func SysCall(fi fs.FileInfo) any { return fi.Sys() }

func Modified(fi fs.FileInfo) int64 { return fi.ModTime().Unix() }

func Stat() int64 {
	fi, _ := os.Stat(".")
	return fi.Size()
}

func SetPath(cmd *exec.Cmd) { cmd.Path = "x" }

func Alias(cmd *exec.Cmd) []string {
	a := cmd.Args
	return a
}

func Literal() *exec.Cmd { return &exec.Cmd{} }

func New() *exec.Cmd { return new(exec.Cmd) }

func ReadPath(cmd *exec.Cmd) string { return cmd.Path }

func SetDir(cmd *exec.Cmd) { cmd.Dir = "x" }

func Helper() int { return len([]int{}) }
`)

	by := byRoot(refuseFixture(t, files, rootsOf("History", "Format", "SysCall", "Modified", "Stat", "SetPath", "Alias", "Literal", "New", "ReadPath", "SetDir", "Helper")...))

	want := map[string]string{
		"History": "history", "Format": "mtime", "SysCall": "mtime", "Modified": "mtime", "SetPath": "exec", "Alias": "exec",
		"Literal": "exec", "New": "exec", "ReadPath": "exec",
	}
	for name, class := range want {
		if len(by[name]) != 1 || by[name][0].Class != class {
			t.Errorf("%s = %+v, want one %s", name, by[name], class)
		}
	}
	for _, name := range []string{"Stat", "SetDir", "Helper"} {
		if len(by[name]) != 0 {
			t.Errorf("%s = %+v, want none", name, by[name])
		}
	}
}

func TestRefusals_ReachesTheMethodsOfEachTypeInTheStructureOfALiveType(t *testing.T) {
	t.Parallel()
	files := gatesFile(`"os/exec"`, `type A struct{}

func (A) M() { _ = exec.Command("git", "a") }

type B struct{}

func (B) M() { _ = exec.Command("git", "b") }

type C struct{}

func (C) M() { _ = exec.Command("git", "c") }

type D struct{}

func (D) M() { _ = exec.Command("git", "d") }

type E struct{}

func (E) M() { _ = exec.Command("git", "e") }

type F struct{}

func (F) M() { _ = exec.Command("git", "f") }

type G[T any] struct{ v T }

func (G[T]) M() { _ = exec.Command("git", "g") }

type H struct{}

func (H) M() { _ = exec.Command("git", "h") }

type Outer struct {
	a []A
	b [2]B
	c chan C
	d map[D]E
	p *F
	g G[H]
	s struct{ n A }
}

func Run() { var o any = Outer{}; _ = o }
`)

	got := refuseFixture(t, files, gateRoot("gates", "Run"))

	if len(got) != 8 {
		t.Errorf("refusals = %d, want 8 (A to H): %+v", len(got), got)
	}
}

func TestRefusals_CallFormsParenIndexListAndErrorMethodsAreJudged(t *testing.T) {
	t.Parallel()
	files := gatesFile(`"errors"
	"os/exec"
	"`+modulePath+`/internal/shr"`, `func gitFn() { _ = exec.Command("git", "log") }

func Pair[K, V any](k K, v V) { gitFn() }

func apply(f func()) { f() }

func ViaParen() { (gitFn)() }

func ViaIndexList() { Pair[int, string](1, "a") }

func ViaArgument() { apply(gitFn) }

func ViaCallResult() { get()() }

func get() func() { return gitFn }

func ErrorMethod() string { return errors.New("x").Error() }

func Cmds() { _ = new(*exec.Cmd); _ = []exec.Cmd{{}} }

func ReadShared() *exec.Cmd { return shr.Shared }
`)
	files["internal/shr/shr.go"] = "package shr\n\nimport \"os/exec\"\n\nvar Shared = exec.Command(\"git\", \"status\")\n\nvar Other = 1\n\nvar Alias = Other\n"

	by := byRoot(refuseFixture(t, files, rootsOf("ViaParen", "ViaIndexList", "ViaArgument", "ViaCallResult", "ErrorMethod", "Cmds", "ReadShared")...))

	for _, name := range []string{"ViaParen", "ViaIndexList", "ViaArgument", "ReadShared"} {
		if len(by[name]) == 0 || by[name][0].Class != "git" {
			t.Errorf("%s = %+v", name, by[name])
		}
	}
	if got := edges(by["ViaIndexList"][0]); !slices.Equal(got, []string{"", "call", "call"}) {
		t.Errorf("ViaIndexList edges = %v", got)
	}
	if len(by["ReadShared"]) != 1 {
		t.Errorf("ReadShared = %+v, want one deduplicated refusal", by["ReadShared"])
	}
	if len(by["ErrorMethod"]) != 0 || len(by["Cmds"]) != 0 {
		t.Errorf("ErrorMethod = %+v Cmds = %+v", by["ErrorMethod"], by["Cmds"])
	}
}

func TestRefusals_ChecksTheParameterOfAMethodEntryAndSkipsAnEntryTheProgramLacks(t *testing.T) {
	t.Parallel()
	g := loadFixture(t, gatesFile(`"go/build"`, "func Run() { _, _ = build.Default.Import(\"x\", \"\", 0) }\n"), gateRoot("gates", "Run"))
	rules := Contract()
	rules.Exec = []ExecEntry{{Package: "go/build", Name: "Context.Import", Param: "path"}, {Package: "go/build", Name: "Absent", Param: "x"}}

	got, err := Refusals(g, rules)

	if err != nil || len(got) != 1 || got[0].Class != "exec" {
		t.Errorf("got = %+v err = %v", got, err)
	}
	rules.Exec = []ExecEntry{{Package: "go/build", Name: "Context.Import", Param: "nope"}}
	if _, err := Refusals(g, rules); err == nil {
		t.Error("a missing method parameter was accepted")
	}
}

const runnerSig = `func(ctx context.Context, name, dir string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error)`

func TestRefusals_InitializerLiteralsAreScannedWhenTheyRunOrAreReached(t *testing.T) {
	t.Parallel()
	pkg := func(body string) string {
		return "package PKG\n\nimport (\n\t\"os/exec\"\n\t\"sort\"\n)\n\nvar _ = sort.Ints\n\nfunc gitFn() { _ = exec.Command(\"git\", \"log\") }\n\nfunc apply(f func()) int { f(); return 0 }\n\ntype T struct{}\n\nfunc Run() int { return 0 }\n\n" + body
	}
	cases := map[string]struct {
		body string
		want int
	}{
		"invoked":       {"var x = func() T { _ = exec.Command(\"git\", \"log\"); return T{} }()\n", 1},
		"parenthesized": {"var x = (func() int { gitFn(); return 0 })()\n", 1},
		"invokedValue":  {"var x = func() int { f := gitFn; f(); return 0 }()\n", 1},
		"nested":        {"var x = func() int { f := func() { gitFn() }; f(); return 0 }()\n", 1},
		"argument":      {"var _ = apply(gitFn)\n", 1},
		"stored":        {"var stored = gitFn\n", 0},
		"unreadHook":    {"var Hook = func() { _ = exec.Command(\"git\", \"log\") }\n", 0},
		"readHook":      {"var Hook = func() { _ = exec.Command(\"git\", \"log\") }\n\nfunc Run2() { Hook() }\n", 1},
		"uncalled":      {"var lit = func() int { gitFn(); return 0 }\n", 0},
	}
	files := map[string]string{}
	var roots []Root
	for name, tc := range cases {
		files["internal/"+name+"/"+name+".go"] = strings.ReplaceAll(pkg(tc.body), "PKG", name)
		fn := "Run"
		if name == "readHook" {
			fn = "Run2"
		}
		roots = append(roots, gateRoot(name, fn))
	}

	got := refuseFixture(t, files, roots...)

	count := map[string]int{}
	for _, r := range got {
		count[r.Root.Package[strings.LastIndex(r.Root.Package, "/")+1:]]++
	}
	for name, tc := range cases {
		if count[name] != tc.want {
			t.Errorf("%s = %d refusals, want %d: %+v", name, count[name], tc.want, got)
		}
	}
}

func TestRefusals_ModTimeAndSysAreMatchedByNameAndSignatureOnAnyReceiver(t *testing.T) {
	t.Parallel()
	files := gatesFile(`"time"`, `type clock struct{}

func (clock) ModTime() time.Time { return time.Time{} }

func (clock) Sys() any { return nil }

type other struct{}

func (other) ModTime() int { return 0 }

func (other) Sys() int { return 0 }

func ModAny(c clock) time.Time { return c.ModTime() }

func SysAny(c clock) any { return c.Sys() }

func Unrelated(o other) int { return o.ModTime() + o.Sys() }
`)

	by := byRoot(refuseFixture(t, files, rootsOf("ModAny", "SysAny", "Unrelated")...))

	for _, name := range []string{"ModAny", "SysAny"} {
		if got := classes(by[name]); !slices.Equal(got, []string{"mtime"}) {
			t.Errorf("%s = %v", name, got)
		}
	}
	if len(by["Unrelated"]) != 0 {
		t.Errorf("Unrelated = %+v", by["Unrelated"])
	}
}

func TestRefusals_ACallThroughATypeParameterWhoseTypeSetHoldsRunFuncIsJudged(t *testing.T) {
	t.Parallel()
	files := gatesFile(`"context"
	"io"
	`+sx, `type altA `+runnerSig+`

type altB `+runnerSig+`

func run[F sysexec.RunFunc](ctx context.Context, f F) {
	_, _ = f(ctx, "git", "", nil, nil, nil, nil, nil)
}

func viaUnion[F interface{ altA | altB }](ctx context.Context, f F) {
	_, _ = f(ctx, "git", "", nil, nil, nil, nil, nil)
}

func viaNested[F interface{ nested }](ctx context.Context, f F) {
	_, _ = f(ctx, "go", "", nil, nil, nil, nil, nil)
}

type nested interface{ ~`+runnerSig+` }

func unrelated[F ~func() | ~int](f F) {
	if g, ok := any(f).(func()); ok {
		g()
	}
}

func inert[F ~func(io.Reader)](r io.Reader, f F) { f(r) }

func Generic(ctx context.Context) { run(ctx, sysexec.RunFunc(sysexec.DefaultRunner)) }

func Union(ctx context.Context) { viaUnion(ctx, altA(sysexec.RunFunc(sysexec.DefaultRunner))) }

func Nested(ctx context.Context) { viaNested(ctx, sysexec.RunFunc(sysexec.DefaultRunner)) }

func Unrelated() { unrelated(func() {}) }

func Inert(r io.Reader) { inert(r, func(io.Reader) {}) }
`)

	by := byRoot(refuseFixture(t, files, rootsOf("Generic", "Union", "Nested", "Unrelated", "Inert")...))

	want := map[string][]string{"Generic": {"git"}, "Union": {"git"}, "Nested": {"exec"}}
	for name, exp := range want {
		if got := classes(by[name]); !slices.Equal(got, exp) {
			t.Errorf("%s = %v, want %v", name, got, exp)
		}
	}
	for _, name := range []string{"Unrelated", "Inert"} {
		if len(by[name]) != 0 {
			t.Errorf("%s = %+v, want none", name, by[name])
		}
	}
}

func TestRefusals_ACompoundAssignmentReadsAPackageVariableAndReachesItsInitializer(t *testing.T) {
	t.Parallel()
	files := map[string]string{
		"internal/gates/gates.go": "package gates\n\nimport . \"" + modulePath + "/internal/pkg\"\n\nfunc Compound() { V += 1 }\n\nfunc Plain() { V = 1 }\n\nfunc Local() int {\n\tv := 1\n\tv += 1\n\treturn v\n}\n",
		"internal/pkg/pkg.go":     "package pkg\n\nimport \"os/exec\"\n\nvar V = gitCount()\n\nfunc gitCount() int {\n\t_ = exec.Command(\"git\", \"log\").Run()\n\treturn 0\n}\n",
	}

	by := byRoot(refuseFixture(t, files, rootsOf("Compound", "Plain", "Local")...))

	if got := classes(by["Compound"]); !slices.Equal(got, []string{"git"}) {
		t.Errorf("Compound = %v", got)
	}
	if len(by["Plain"]) != 0 || len(by["Local"]) != 0 {
		t.Errorf("Plain = %+v Local = %+v", by["Plain"], by["Local"])
	}
}

func TestRefusals_TheStdlibPackagesWhoseImportsHoldOsExecAreRefusedAndTheInertListPasses(t *testing.T) {
	t.Parallel()
	files := gatesFile(`"go/build"
	"go/importer"
	"go/token"
	"net/http"
	"net/http/cgi"
	"net/http/fcgi"
	"debug/buildinfo"
	"os"`, `func Cgi(w http.ResponseWriter, r *http.Request) {
	h := cgi.Handler{Path: "/usr/bin/git"}
	h.ServeHTTP(w, r)
}

func Importer() {
	_, _ = importer.ForCompiler(token.NewFileSet(), "source", nil).Import("fmt")
}

func ReadFile() { _, _ = buildinfo.ReadFile("x") }

func Read() { _, _ = buildinfo.Read(os.Stdin) }

func Inert() {
	_, _ = build.ImportDir(".", 0)
	_, _ = build.Default.MatchFile(".", "x.go")
}

func Plain() { _ = http.StatusText(200) }

func CgiValue(w http.ResponseWriter, r *http.Request) {
	h := &cgi.Handler{}
	f := h.ServeHTTP
	f(w, r)
}

func CgiInert(env map[string]string) {
	_, _ = cgi.RequestFromMap(env)
	_, _ = cgi.Request()
	_ = fcgi.ProcessEnv
}
`)

	by := byRoot(refuseFixture(t, files, rootsOf("Cgi", "Importer", "ReadFile", "Read", "Inert", "Plain", "CgiValue", "CgiInert")...))

	want := map[string][]string{"Cgi": {"exec"}, "Importer": {"exec"}, "ReadFile": {"history"}, "Read": {"history"}, "CgiValue": {"exec"}}
	for name, exp := range want {
		if got := classes(by[name]); !slices.Equal(got, exp) {
			t.Errorf("%s = %v, want %v", name, got, exp)
		}
	}
	for _, name := range []string{"Inert", "Plain", "CgiInert"} {
		if len(by[name]) != 0 {
			t.Errorf("%s = %+v, want none", name, by[name])
		}
	}
}
