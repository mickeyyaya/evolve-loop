//go:build acs

package cycle2489

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fakecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/testaudit"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const libSource = `package fx

import "errors"

var ErrBad = errors.New("bad")

type Rec struct{ N int }

type Dup struct{ N int }

func Add(a, b int) int { return a + b }

func Make(n int) Rec { return Rec{N: n} }

func Fail() error { return ErrBad }

func Names() []string { return []string{"a", "b"} }

func Render() string { return "out" }

func Drive(run func(string)) {
	run("a")
	run("b")
}
`

type ruleCase struct {
	name   string
	rule   string
	red    string
	green  string
	cfg    func(dir string) testaudit.Config
	redCfg func(dir string) testaudit.Config
}

func noCfg(string) testaudit.Config { return testaudit.Config{} }

var ruleCases = []ruleCase{
	{
		name: "no-assertion",
		rule: "no-assertion",
		red: `package fx

import "testing"

func TestNothing(t *testing.T) {
	x := 1
	_ = x
}
`,
		green: `package fx

import "testing"

func TestChecks(t *testing.T) {
	if got := Add(1, 2); got != 3 {
		t.Fatalf("got %d", got)
	}
}
`,
	},
	{
		name: "log-only",
		rule: "log-only",
		red: `package fx

import "testing"

func TestLogs(t *testing.T) {
	t.Log(Add(1, 2))
}
`,
		green: `package fx

import "testing"

func TestLogsAndChecks(t *testing.T) {
	got := Add(1, 2)
	t.Logf("got %d", got)
	if got != 3 {
		t.Errorf("got %d", got)
	}
}
`,
	},
	{
		name: "asserts-own-setup",
		rule: "asserts-own-setup",
		red: `package fx

import "testing"

func TestSelf(t *testing.T) {
	r := Rec{N: 3}
	if r.N != 3 {
		t.Errorf("n")
	}
}
`,
		green: `package fx

import "testing"

func TestMakeSetsN(t *testing.T) {
	r := Make(3)
	if r.N != 3 {
		t.Errorf("n = %d", r.N)
	}
}
`,
	},
	{
		name: "both-branches-pass",
		rule: "both-branches-pass",
		red: `package fx

import "testing"

func TestBranches(t *testing.T) {
	got := Add(1, 2)
	if got == 3 {
		_ = got
	} else {
		_ = 0
	}
	_ = t
}
`,
		green: `package fx

import "testing"

func TestBranchFails(t *testing.T) {
	got := Add(1, 2)
	if got == 3 {
		t.Log("ok")
	} else {
		t.Fatalf("got %d", got)
	}
}
`,
	},
	{
		name: "skip-hides-error",
		rule: "skip-hides-error",
		red: `package fx

import "testing"

func TestSkipOnError(t *testing.T) {
	err := Fail()
	if err != nil {
		t.Skip("cannot")
	}
	if got := Add(1, 2); got != 3 {
		t.Fatal("x")
	}
}
`,
		green: `package fx

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/testhost"
)

func TestNeedsTool(t *testing.T) {
	testhost.Tool(t, "git")
	if got := Add(1, 2); got != 3 {
		t.Fatal("x")
	}
}

func TestShortSkip(t *testing.T) {
	if testing.Short() {
		t.Skip("short")
	}
	if got := Add(1, 2); got != 3 {
		t.Fatal("x")
	}
}
`,
	},
	{
		name: "skip-hides-error/blank-error-assign",
		rule: "skip-hides-error",
		red: `package fx

import "testing"

func TestSkipAfterBlank(t *testing.T) {
	cfg, _ := Fail(), 0
	if cfg != nil {
		t.Skip("no cfg")
	}
	if got := Add(1, 2); got != 3 {
		t.Fatal("x")
	}
}
`,
		green: `package fx

import "testing"

func TestNoSkip(t *testing.T) {
	cfg := Fail()
	if cfg != nil {
		t.Fatal(cfg)
	}
}
`,
	},
	{
		name: "literal-field-count",
		rule: "literal-field-count",
		red: `package fx

import (
	"reflect"
	"testing"
)

func TestFieldCount(t *testing.T) {
	if reflect.TypeOf(Rec{}).NumField() != 1 {
		t.Fatal("fields")
	}
}
`,
		green: `package fx

import (
	"reflect"
	"testing"
)

func TestFieldCountMatchesNames(t *testing.T) {
	want := len(Names())
	if reflect.TypeOf(Rec{}).NumField() != want {
		t.Fatal("fields")
	}
}
`,
	},
	{
		name: "literal-field-count/const",
		rule: "literal-field-count",
		red: `package fx

import (
	"reflect"
	"testing"
)

const wantFields = 1

func TestFieldCountConst(t *testing.T) {
	if reflect.TypeOf(Rec{}).NumField() != wantFields {
		t.Fatal("fields")
	}
}
`,
		green: `package fx

import (
	"reflect"
	"testing"
)

func TestFieldCountUnderScore(t *testing.T) {
	wantFields := len(Names())
	if reflect.TypeOf(Rec{}).NumField() != wantFields {
		t.Fatal("fields")
	}
}
`,
	},
	{
		name: "single-file-source-pin",
		rule: "single-file-source-pin",
		red: `package fx

import (
	"os"
	"strings"
	"testing"
)

func TestPin(t *testing.T) {
	b, err := os.ReadFile("lib.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "func Add") {
		t.Fatal("missing")
	}
}
`,
		green: `package fx

import (
	"os"
	"strings"
	"testing"
)

func TestFixtureText(t *testing.T) {
	b, err := os.ReadFile("testdata/in.txt")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "hello") {
		t.Fatal("missing")
	}
}
`,
	},
	{
		name: "single-file-source-pin/const-join",
		rule: "single-file-source-pin",
		red: `package fx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const libFile = "lib.go"

func TestPinJoined(t *testing.T) {
	b, err := os.ReadFile(filepath.Join(".", libFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "func Add") {
		t.Fatal("missing")
	}
}
`,
		green: `package fx

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNoBannedAnywhere(t *testing.T) {
	err := filepath.WalkDir(".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".go") {
			return nil
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		if strings.Contains(string(b), "banned") {
			t.Errorf("%s has banned", p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
`,
	},
	{
		name: "single-file-source-pin/acsassert",
		rule: "single-file-source-pin",
		red: `package fx

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestPinViaAssert(t *testing.T) {
	acsassert.FileContains(t, "lib.go", "func Add")
}
`,
		green: `package fx

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestDataViaAssert(t *testing.T) {
	acsassert.FileContains(t, "testdata/in.txt", "hello")
}
`,
	},
	{
		name: "single-file-ast-pin",
		rule: "single-file-ast-pin",
		red: `package fx

import (
	"go/parser"
	"go/token"
	"testing"
)

func TestAstPin(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "lib.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Decls) != 9 {
		t.Fatalf("decls %d", len(f.Decls))
	}
}
`,
		green: `package fx

import (
	"go/parser"
	"go/token"
	"testing"
)

func TestAstTree(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) == 0 {
		t.Fatal("no packages")
	}
}
`,
	},
	{
		name: "unverified-fake/unlisted-install",
		rule: "unverified-fake",
		red: `package fx

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
)

func TestInstallsGit(t *testing.T) {
	bin := t.TempDir()
	fakeclitest.Install(t, filepath.Join(bin, "git"), "#!/bin/sh\nexit 0\n")
	if got := Add(1, 2); got != 3 {
		t.Fatal("x")
	}
}
`,
		green: `package fx

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
)

func TestInstallsListedGit(t *testing.T) {
	bin := t.TempDir()
	fakeclitest.Install(t, filepath.Join(bin, "git"), "#!/bin/sh\nexit 0\n")
	if got := Add(1, 2); got != 3 {
		t.Fatal("x")
	}
}
`,
		cfg: func(dir string) testaudit.Config {
			return testaudit.Config{Fakes: []testaudit.Fake{{Tool: "git", RealSide: "none", Reason: "no real side in the fixture"}}}
		},
		redCfg: noCfg,
	},
	{
		name: "unverified-fake/const-traced-install",
		rule: "unverified-fake",
		red: `package fx

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
)

const shimName = "git"

func TestInstallsConstGit(t *testing.T) {
	fakeclitest.Install(t, filepath.Join(t.TempDir(), shimName), "#!/bin/sh\nexit 0\n")
	if got := Add(1, 2); got != 3 {
		t.Fatal("x")
	}
}
`,
		green: `package fx

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
)

const shimName = "git"

func TestInstallsConstListedGit(t *testing.T) {
	fakeclitest.Install(t, filepath.Join(t.TempDir(), shimName), "#!/bin/sh\nexit 0\n")
	if got := Add(1, 2); got != 3 {
		t.Fatal("x")
	}
}
`,
		cfg: func(dir string) testaudit.Config {
			return testaudit.Config{Fakes: []testaudit.Fake{{Tool: "git", RealSide: "none", Reason: "no real side in the fixture"}}}
		},
		redCfg: noCfg,
	},
	{
		name: "unverified-fake/row-without-suite",
		rule: "unverified-fake",
		red: `package fx

import "testing"

func TestPlain(t *testing.T) {
	if got := Add(1, 2); got != 3 {
		t.Fatal("x")
	}
}
`,
		green: `package fx

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fakecontract"
)

var gitSuite = fakecontract.Suite{Tool: "git"}

func TestGitContract(t *testing.T) {
	_ = gitSuite
	if got := Add(1, 2); got != 3 {
		t.Fatal("x")
	}
}
`,
		cfg: func(dir string) testaudit.Config {
			return testaudit.Config{Fakes: []testaudit.Fake{{Tool: "git", SuiteDir: dir, RealSide: "none", Reason: "no real side in the fixture"}}}
		},
	},
	{
		name: "mock-call-order",
		rule: "mock-call-order",
		red: `package fx

import (
	"reflect"
	"testing"
)

type fakeRunner struct{ calls []string }

func (f *fakeRunner) Run(c string) { f.calls = append(f.calls, c) }

func TestOrder(t *testing.T) {
	f := &fakeRunner{}
	Drive(f.Run)
	want := []string{"a", "b"}
	if !reflect.DeepEqual(f.calls, want) {
		t.Fatalf("calls %v", f.calls)
	}
}
`,
		green: `package fx

import (
	"reflect"
	"testing"
)

func TestNamesValue(t *testing.T) {
	if !reflect.DeepEqual(Names(), []string{"a", "b"}) {
		t.Fatalf("names %v", Names())
	}
}
`,
	},
	{
		name: "golden-update-flag",
		rule: "golden-update-flag",
		red: `package fx

import (
	"os"
	"testing"
)

func TestGoldenWrites(t *testing.T) {
	got := Render()
	if err := os.WriteFile("testdata/golden.txt", []byte(got), 0o644); err != nil {
		t.Fatal(err)
	}
}
`,
		green: `package fx

import (
	"flag"
	"os"
	"testing"
)

var update = flag.Bool("update", false, "update golden")

func TestGoldenWithFlag(t *testing.T) {
	got := Render()
	if *update {
		if err := os.WriteFile("testdata/golden.txt", []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile("testdata/golden.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(want) != got {
		t.Fatalf("drift")
	}
}
`,
	},
	{
		name: "error-text-match",
		rule: "error-text-match",
		red: `package fx

import (
	"strings"
	"testing"
)

func TestErrorText(t *testing.T) {
	err := Fail()
	if err == nil || !strings.Contains(err.Error(), "bad") {
		t.Fatal("wrong error")
	}
}
`,
		green: `package fx

import (
	"errors"
	"testing"
)

func TestErrorIdentity(t *testing.T) {
	err := Fail()
	if !errors.Is(err, ErrBad) {
		t.Fatal("wrong error")
	}
}
`,
	},
	{
		name: "sleep-sync",
		rule: "sleep-sync",
		red: `package fx

import (
	"testing"
	"time"
)

func TestSleeps(t *testing.T) {
	time.Sleep(10 * time.Millisecond)
	if got := Add(1, 2); got != 3 {
		t.Fatal("x")
	}
}
`,
		green: `package fx

import "testing"

func TestWaitsOnChannel(t *testing.T) {
	done := make(chan int, 1)
	go func() { done <- Add(1, 2) }()
	if got := <-done; got != 3 {
		t.Fatal("x")
	}
}
`,
	},
}

func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		full := filepath.Join(dir, filepath.FromSlash(name))
		if mkErr := os.MkdirAll(filepath.Dir(full), 0o755); mkErr != nil {
			t.Fatal(mkErr)
		}
		if wrErr := os.WriteFile(full, []byte(files[name]), 0o644); wrErr != nil {
			t.Fatal(wrErr)
		}
	}
	return dir
}

func analyze(t *testing.T, dir string, cfg testaudit.Config) []testaudit.Finding {
	t.Helper()
	found, err := testaudit.Analyze(dir, cfg)
	if err != nil {
		t.Fatalf("Analyze(%s): %v", dir, err)
	}
	return found
}

func ofRule(found []testaudit.Finding, rule string) []testaudit.Finding {
	var out []testaudit.Finding
	for _, f := range found {
		if f.Rule == rule {
			out = append(out, f)
		}
	}
	return out
}

func describe(found []testaudit.Finding) string {
	parts := make([]string, 0, len(found))
	for _, f := range found {
		parts = append(parts, f.Rule+":"+f.Test)
	}
	return strings.Join(parts, ", ")
}

func configFor(c ruleCase, dir string) testaudit.Config {
	if c.cfg == nil {
		return noCfg(dir)
	}
	return c.cfg(dir)
}

func redConfigFor(c ruleCase, dir string) testaudit.Config {
	if c.redCfg != nil {
		return c.redCfg(dir)
	}
	return configFor(c, dir)
}

var wantRuleIDs = []string{
	"no-assertion", "log-only", "asserts-own-setup", "both-branches-pass", "skip-hides-error",
	"literal-field-count", "single-file-source-pin", "single-file-ast-pin", "unverified-fake",
	"mock-call-order", "golden-update-flag", "error-text-match", "sleep-sync",
}

func TestC2489_001_RuleSetIsExactlyPlanSection6(t *testing.T) {
	got := append([]string(nil), testaudit.RuleIDs()...)
	sort.Strings(got)
	want := append([]string(nil), wantRuleIDs...)
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("RuleIDs() = %v, want %v", got, want)
	}
}

func TestC2489_002_EveryRuleHasARedFixtureWithOneFindingAndAGreenFixtureWithNone(t *testing.T) {
	covered := map[string]bool{}
	for _, c := range ruleCases {
		covered[c.rule] = true
		t.Run(c.name, func(t *testing.T) {
			redDir := writeTree(t, map[string]string{"lib.go": libSource, "x_test.go": c.red})
			red := analyze(t, redDir, redConfigFor(c, redDir))
			if got := ofRule(red, c.rule); len(got) != 1 {
				t.Errorf("red fixture: %d findings of %s, want exactly 1 (all findings: %s)", len(got), c.rule, describe(red))
			}
			greenDir := writeTree(t, map[string]string{"lib.go": libSource, "x_test.go": c.green, "testdata/in.txt": "hello"})
			green := analyze(t, greenDir, configFor(c, greenDir))
			if len(green) != 0 {
				t.Errorf("green fixture: want no findings, got %s", describe(green))
			}
		})
	}
	for _, id := range wantRuleIDs {
		if !covered[id] {
			t.Errorf("rule %s has no red/green fixture pair", id)
		}
	}
}

func TestC2489_003_ATestNameInsideARawStringIsNotAFinding(t *testing.T) {
	tick := "`"
	embedded := "const embedded = " + tick + "func TestInRaw(t *testing.T) {\n\ttime.Sleep(1)\n\tt.Skip(\"x\")\n}" + tick + "\n"
	body := "package fx\n\nimport \"testing\"\n\n" + embedded +
		"\nfunc TestReal(t *testing.T) {\n\tif got := Add(1, 2); got != 3 {\n\t\tt.Fatalf(\"got %d\", got)\n\t}\n\t_ = embedded\n}\n"
	dir := writeTree(t, map[string]string{"lib.go": libSource, "x_test.go": body})
	if found := analyze(t, dir, testaudit.Config{}); len(found) != 0 {
		t.Errorf("raw-string text became a finding: %s", describe(found))
	}
	twin := strings.Replace(body, "\t_ = embedded\n", "\ttime.Sleep(1)\n", 1)
	twin = strings.Replace(twin, "import \"testing\"", "import (\n\t\"testing\"\n\t\"time\"\n)", 1)
	twinDir := writeTree(t, map[string]string{"lib.go": libSource, "x_test.go": twin})
	if got := ofRule(analyze(t, twinDir, testaudit.Config{}), "sleep-sync"); len(got) != 1 {
		t.Errorf("a real time.Sleep beside the raw string gave %d sleep-sync findings, want 1", len(got))
	}
}

func TestC2489_004_AGuardRowExemptsByRowNeverByShape(t *testing.T) {
	sleepCase := ruleCases[len(ruleCases)-1]
	dir := writeTree(t, map[string]string{"lib.go": libSource, "x_test.go": sleepCase.red})
	if got := ofRule(analyze(t, dir, testaudit.Config{}), "sleep-sync"); len(got) != 1 {
		t.Fatalf("unguarded sleep gave %d findings, want 1", len(got))
	}
	byTest := testaudit.Config{Guards: []testaudit.Guard{{Package: dir, Test: "TestSleeps", Reason: "waits for a real timer by design"}}}
	if got := ofRule(analyze(t, dir, byTest), "sleep-sync"); len(got) != 0 {
		t.Errorf("a guard row naming TestSleeps left %d findings", len(got))
	}
	byPackage := testaudit.Config{Guards: []testaudit.Guard{{Package: dir, Reason: "whole package waits on real timers"}}}
	if got := ofRule(analyze(t, dir, byPackage), "sleep-sync"); len(got) != 0 {
		t.Errorf("a package guard row left %d findings", len(got))
	}
	otherTest := testaudit.Config{Guards: []testaudit.Guard{{Package: dir, Test: "TestSomethingElse", Reason: "other"}}}
	if got := ofRule(analyze(t, dir, otherTest), "sleep-sync"); len(got) != 1 {
		t.Errorf("a guard row for another test exempted TestSleeps: %d findings, want 1", len(got))
	}
}

func TestC2489_005_DefaultConfigCarriesKnownFakeRowsAndABoundedGuardList(t *testing.T) {
	cfg, err := testaudit.DefaultConfig()
	if err != nil {
		t.Fatalf("DefaultConfig: %v", err)
	}
	tools := map[string]bool{}
	for _, f := range cfg.Fakes {
		tools[f.Tool] = true
		if f.RealSide == "none" && strings.TrimSpace(f.Reason) == "" {
			t.Errorf("fake row %q has real side none and no reason", f.Tool)
		}
	}
	for _, want := range []string{"git", "tmux"} {
		if !tools[want] {
			t.Errorf("fakes.json has no row for %s", want)
		}
	}
	if cfg.MaxGuardRows <= 0 {
		t.Fatalf("MaxGuardRows = %d, want a positive bound", cfg.MaxGuardRows)
	}
	if len(cfg.Guards) > cfg.MaxGuardRows {
		t.Errorf("%d guard rows exceed max_rows %d", len(cfg.Guards), cfg.MaxGuardRows)
	}
}

func redTree(t *testing.T) string {
	t.Helper()
	return writeTree(t, map[string]string{"lib.go": libSource, "x_test.go": ruleCases[0].red})
}

func TestC2489_006_RatchetPassesWhenShardMatchesFindings(t *testing.T) {
	dir := redTree(t)
	found := analyze(t, dir, testaudit.Config{})
	if len(found) == 0 {
		t.Fatal("red tree gave no findings to baseline")
	}
	if err := testaudit.WriteShard(dir, found); err != nil {
		t.Fatalf("WriteShard: %v", err)
	}
	if err := testaudit.CheckShard(dir, testaudit.Config{}); err != nil {
		t.Errorf("CheckShard on a matching shard: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "testdata", "testaudit-baseline.json"))
	if err != nil {
		t.Fatalf("shard not at testdata/testaudit-baseline.json: %v", err)
	}
	var doc struct {
		Rows []map[string]any `json:"rows"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil || len(doc.Rows) == 0 {
		t.Fatalf("shard is not {\"rows\":[...]}: %v", err)
	}
	for _, row := range doc.Rows {
		for _, key := range []string{"package", "test", "rule"} {
			if _, ok := row[key]; !ok {
				t.Errorf("shard row %v has no %q key", row, key)
			}
		}
		for key := range row {
			if strings.Contains(key, "line") {
				t.Errorf("shard row carries %q; rows must hold no line number", key)
			}
		}
	}
}

func TestC2489_007_RatchetFailsOnAFindingMissingFromTheBaseline(t *testing.T) {
	dir := redTree(t)
	if err := testaudit.WriteShard(dir, analyze(t, dir, testaudit.Config{})); err != nil {
		t.Fatal(err)
	}
	extra := "package fx\n\nimport \"testing\"\n\nfunc TestBrandNewEmpty(t *testing.T) {\n\tx := 2\n\t_ = x\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "y_test.go"), []byte(extra), 0o644); err != nil {
		t.Fatal(err)
	}
	err := testaudit.CheckShard(dir, testaudit.Config{})
	if err == nil {
		t.Fatal("CheckShard passed with a finding that has no baseline row")
	}
	if !strings.Contains(err.Error(), "TestBrandNewEmpty") {
		t.Errorf("error does not name the unbaselined test: %v", err)
	}
}

func TestC2489_008_RatchetFailsOnABaselineRowThatNoLongerExists(t *testing.T) {
	dir := redTree(t)
	if err := testaudit.WriteShard(dir, analyze(t, dir, testaudit.Config{})); err != nil {
		t.Fatal(err)
	}
	fixed := "package fx\n\nimport \"testing\"\n\nfunc TestNothing(t *testing.T) {\n\tif got := Add(1, 2); got != 3 {\n\t\tt.Fatalf(\"got %d\", got)\n\t}\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "x_test.go"), []byte(fixed), 0o644); err != nil {
		t.Fatal(err)
	}
	if len(analyze(t, dir, testaudit.Config{})) != 0 {
		t.Fatal("fixed tree still has findings; fixture is wrong")
	}
	err := testaudit.CheckShard(dir, testaudit.Config{})
	if err == nil {
		t.Fatal("CheckShard passed with a stale baseline row")
	}
	if !strings.Contains(err.Error(), "TestNothing") {
		t.Errorf("error does not name the stale row's test: %v", err)
	}
}

func TestC2489_009_RatchetAcceptsACleanTreeWithNoShard(t *testing.T) {
	dir := writeTree(t, map[string]string{"lib.go": libSource, "x_test.go": ruleCases[0].green})
	if err := testaudit.CheckShard(dir, testaudit.Config{}); err != nil {
		t.Errorf("clean tree with no shard: %v", err)
	}
	unbaselined := redTree(t)
	if err := testaudit.CheckShard(unbaselined, testaudit.Config{}); err == nil {
		t.Error("tree with findings and no shard passed the ratchet")
	}
}

func TestC2489_010_FakecontractSuiteNamesItsTool(t *testing.T) {
	s := fakecontract.Suite{Tool: "git"}
	if s.Tool != "git" {
		t.Errorf("Suite.Tool = %q, want git", s.Tool)
	}
}

func TestC2489_011_NewPackagesGraduateIntoTheApicoverGate(t *testing.T) {
	root := acsassert.RepoRoot(t)
	enforce, err := os.ReadFile(filepath.Join(root, "go", ".apicover-enforce"))
	if err != nil {
		t.Fatal(err)
	}
	lines := map[string]bool{}
	for _, l := range strings.Split(string(enforce), "\n") {
		lines[strings.TrimSpace(l)] = true
	}
	for _, pkg := range []string{"testaudit", "fakecontract"} {
		if !lines["./internal/"+pkg] {
			t.Errorf("go/.apicover-enforce has no ./internal/%s line", pkg)
		}
		if !acsassert.FileExists(t, filepath.Join(root, "go", "internal", pkg, "apicover_named_test.go")) {
			t.Errorf("internal/%s/apicover_named_test.go missing", pkg)
		}
	}
}
