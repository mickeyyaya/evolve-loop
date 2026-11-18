package testaudit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const redTest = "package fx\n\nimport \"testing\"\n\nfunc TestNothing(t *testing.T) {\n\t_ = 1\n}\n"

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	err := os.MkdirAll(filepath.Dir(path), 0o755)
	if err == nil {
		err = os.WriteFile(path, []byte(body), 0o644)
	}
	if err != nil {
		t.Fatal(err)
	}
}

func TestAnalyze_WalkSkipsFixtureAndHiddenDirs(t *testing.T) {
	t.Parallel()
	dir := tree(t, map[string]string{
		"a/x_test.go":          redTest,
		"testdata/x_test.go":   redTest,
		"vendor/x_test.go":     redTest,
		".hidden/x_test.go":    redTest,
		"_under/x_test.go":     redTest,
		"plain/lib.go":         "package fx\n",
		"a/notes.txt":          "x",
		"a/sub/deeper_test.go": redTest,
	})
	found, err := Analyze(dir, Config{})
	if err != nil {
		t.Fatal(err)
	}
	var pkgs []string
	for _, f := range found {
		pkgs = append(pkgs, strings.TrimPrefix(f.Package, dir))
	}
	if got := strings.Join(pkgs, ","); got != "/a,/a/sub" {
		t.Errorf("packages = %q, want /a,/a/sub", got)
	}
}

func TestAnalyze_ReportsAMissingRootAndABrokenFile(t *testing.T) {
	t.Parallel()
	if _, err := Analyze(filepath.Join(t.TempDir(), "absent"), Config{}); err == nil {
		t.Error("Analyze accepted a root that does not exist")
	}
	dir := tree(t, map[string]string{"x_test.go": "package fx\nfunc {"})
	if _, err := Analyze(dir, Config{}); err == nil {
		t.Error("Analyze accepted a file that does not parse")
	}
	if err := CheckShard(dir, Config{}); err == nil {
		t.Error("CheckShard accepted a file that does not parse")
	}
}

func TestAnalyze_GuardNeedsTheSamePackage(t *testing.T) {
	t.Parallel()
	dir := tree(t, map[string]string{"x_test.go": redTest})
	cfg := Config{Guards: []Guard{{Package: dir, Test: "TestOther", Reason: "r"}, {Package: filepath.Join(dir, "elsewhere"), Reason: "r"}}}
	found, err := Analyze(dir, cfg)
	if err != nil || len(found) != 1 {
		t.Fatalf("Analyze = %v, %v; want the one unguarded finding", found, err)
	}
}

func suiteTree(t *testing.T, body string) string {
	t.Helper()
	return tree(t, map[string]string{"s_test.go": "package fx\n\nimport \"testing\"\n\n" + body + "\nfunc TestS(t *testing.T) { t.Fatal(\"x\") }\n"})
}

func TestFakeRows_SuiteLiteralShapes(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		body string
		want int
	}{
		"matching":        {"var s = fakecontract.Suite{Tool: \"git\"}", 0},
		"const tool":      {"const tool = \"git\"\nvar s = fakecontract.Suite{Tool: tool}", 0},
		"other tool":      {"var s = fakecontract.Suite{Tool: \"tmux\"}", 1},
		"unresolved tool": {"var s = fakecontract.Suite{Tool: name}", 1},
		"positional":      {"var s = fakecontract.Suite{\"git\"}", 1},
		"other key":       {"var s = fakecontract.Suite{Other: \"git\"}", 1},
		"computed key":    {"var s = map[string]string{k(): \"git\"}", 1},
		"bare type":       {"var s = Suite{Tool: \"git\"}", 1},
		"other package":   {"var s = other.Suite{Tool: \"git\"}", 1},
		"call type":       {"var s = []string{\"git\"}", 1},
		"not a suite":     {"var s = fakecontract.Other{Tool: \"git\"}", 1},
	}
	for name, c := range cases {
		dir := suiteTree(t, c.body)
		cfg := Config{Fakes: []Fake{{Tool: "git", SuiteDir: dir, RealSide: "x"}}}
		found, err := Analyze(dir, cfg)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := len(ofRuleCount(found, ruleUnverifiedFake)); got != c.want {
			t.Errorf("%s: %d unverified-fake findings, want %d", name, got, c.want)
		}
	}
}

func ofRuleCount(found []Finding, rule string) []Finding {
	var out []Finding
	for _, f := range found {
		if f.Rule == rule {
			out = append(out, f)
		}
	}
	return out
}

func TestFakeRows_ScopeAndFailures(t *testing.T) {
	t.Parallel()
	dir := tree(t, map[string]string{"a/x_test.go": "package fx\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) { t.Fatal(\"x\") }\n"})
	inside := filepath.Join(dir, "a")
	outside := t.TempDir()
	row := func(d string) Config {
		return Config{Fakes: []Fake{{Tool: "git", SuiteDir: d, RealSide: "x"}, {Tool: "tmux", RealSide: "none", Reason: "r"}}}
	}
	if found, err := Analyze(dir, row(inside)); err != nil || len(found) != 1 {
		t.Errorf("a suite dir under the root: %v, %v; want one finding", found, err)
	}
	if found, err := Analyze(dir, row(outside)); err != nil || len(found) != 0 {
		t.Errorf("a suite dir outside the root: %v, %v; want none", found, err)
	}
	if found, err := Analyze(dir, row(filepath.Join(dir, "absent"))); err != nil || len(found) != 1 {
		t.Errorf("a suite dir that does not exist: %v, %v; want one finding", found, err)
	}
	if found, err := analyze(dir, row(inside), false); err != nil || len(found) != 0 {
		t.Errorf("flat analysis of the parent: %v, %v; want none", found, err)
	}
	if found, err := analyze(inside, row(inside), false); err != nil || len(found) != 1 {
		t.Errorf("flat analysis of the suite dir: %v, %v; want one finding", found, err)
	}
	if found, err := analyze(dir, Config{Fakes: []Fake{{Tool: "git", SuiteDir: "relative/dir", RealSide: "x"}}}, true); err != nil || len(found) != 0 {
		t.Errorf("a relative suite dir against an absolute root: %v, %v; want none", found, err)
	}
	broken := filepath.Join(dir, "brk")
	mustWrite(t, filepath.Join(broken, "b.go"), "package fx\nfunc {")
	if _, err := Analyze(dir, Config{Fakes: []Fake{{Tool: "git", SuiteDir: broken, RealSide: "x"}}}); err == nil {
		t.Error("a suite dir that does not parse did not fail")
	}
}

func TestDefaultConfig_RefusesBadEmbeddedData(t *testing.T) {
	fakes, guards := fakesJSON, guardsJSON
	t.Cleanup(func() { fakesJSON, guardsJSON = fakes, guards })
	cases := map[string][2]string{
		"bad fakes":  {"{", string(guards)},
		"bad guards": {string(fakes), "{"},
		"no bound":   {string(fakes), `{"max_rows":0,"rows":[]}`},
	}
	for name, c := range cases {
		fakesJSON, guardsJSON = []byte(c[0]), []byte(c[1])
		if _, err := DefaultConfig(); err == nil {
			t.Errorf("%s: DefaultConfig accepted the data", name)
		}
	}
	fakesJSON, guardsJSON = fakes, guards
	cfg, err := DefaultConfig()
	if err != nil || !cfg.fakeListed("git") || cfg.fakeListed("nothing") {
		t.Errorf("DefaultConfig = %+v, %v; want git listed and nothing unlisted", cfg, err)
	}
}

func TestConfigValidate_NamesTheToolOfAnEmptyRow(t *testing.T) {
	t.Parallel()
	cfg := Config{MaxGuardRows: 1, Fakes: []Fake{{RealSide: "none", Reason: "r"}}}
	if err := cfg.validate(); err == nil {
		t.Error("validate accepted a fake row with no tool")
	}
	ok := Config{MaxGuardRows: 1, Fakes: []Fake{{Tool: "git", RealSide: "x", SuiteDir: "d"}}, Guards: []Guard{{Package: "p", Reason: "r"}}}
	if err := ok.validate(); err != nil {
		t.Errorf("validate rejected a good config: %v", err)
	}
}

func TestShard_WriteFailures(t *testing.T) {
	t.Parallel()
	found := []Finding{{Package: "p", Test: "TestA", Rule: "sleep-sync"}}

	blocked := t.TempDir()
	mustWrite(t, filepath.Join(blocked, "testdata"), "a file where the directory belongs")
	if err := WriteShard(blocked, found); err == nil {
		t.Error("WriteShard passed with a file in place of testdata")
	}

	readOnly := t.TempDir()
	if err := os.MkdirAll(filepath.Join(readOnly, "testdata"), 0o500); err != nil {
		t.Fatal(err)
	}
	if err := WriteShard(readOnly, found); err == nil {
		t.Error("WriteShard passed in a read-only testdata")
	}

	occupied := t.TempDir()
	mustWrite(t, filepath.Join(occupied, shardRelPath, "child"), "x")
	if err := WriteShard(occupied, found); err == nil {
		t.Error("WriteShard passed with a directory at the shard path")
	}
	if err := WriteShard(occupied, nil); err == nil {
		t.Error("clearing a shard passed with a non-empty directory at its path")
	}
	if err := CheckShard(occupied, Config{}); err == nil {
		t.Error("CheckShard read a directory as a shard")
	}
}

func TestShard_BadContentAndMissingDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, shardRelPath), "{")
	if err := CheckShard(dir, Config{}); err == nil {
		t.Error("CheckShard accepted a shard that is not JSON")
	}
	if err := CheckShard(filepath.Join(dir, "absent"), Config{}); err == nil {
		t.Error("CheckShard accepted a directory that does not exist")
	}
}

func TestFinish_DropsDuplicatesAndOrders(t *testing.T) {
	t.Parallel()
	a := Finding{Package: "b", Test: "TestA", Rule: "r2"}
	b := Finding{Package: "a", Test: "TestB", Rule: "r1"}
	c := Finding{Package: "a", Test: "TestA", Rule: "r2"}
	d := Finding{Package: "a", Test: "TestA", Rule: "r1"}
	got := finish([]Finding{a, b, c, d, a}, Config{})
	want := []Finding{d, c, b, a}
	if len(got) != len(want) {
		t.Fatalf("finish = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("finish[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestGuard_MatchesAPackageOrATestByRow(t *testing.T) {
	t.Parallel()
	f := Finding{Package: filepath.Clean("a/b/"), Test: "TestA", Rule: "sleep-sync"}
	cases := []struct {
		guard Guard
		want  bool
	}{
		{Guard{Package: "a/b", Test: "TestA"}, true},
		{Guard{Package: "a/b"}, true},
		{Guard{Package: "a/b", Test: "TestB"}, false},
		{Guard{Package: "a/c"}, false},
	}
	for _, c := range cases {
		if got := (Config{Guards: []Guard{c.guard}}).guarded(f); got != c.want {
			t.Errorf("guard %+v: guarded = %v, want %v", c.guard, got, c.want)
		}
	}
}

func TestCheckShard_NamesBothKindsOfDrift(t *testing.T) {
	t.Parallel()
	dir := tree(t, map[string]string{"x_test.go": redTest})
	if err := CheckShard(dir, Config{}); err == nil || !strings.Contains(err.Error(), "has no baseline row") {
		t.Errorf("no shard: %v, want a missing-row error", err)
	}
	stale := []Finding{{Package: dir, Test: "TestGone", Rule: "sleep-sync"}}
	if err := WriteShard(dir, stale); err != nil {
		t.Fatal(err)
	}
	err := CheckShard(dir, Config{})
	if err == nil || !strings.Contains(err.Error(), "has no baseline row") || !strings.Contains(err.Error(), "matches no finding") {
		t.Errorf("stale shard: %v, want both errors", err)
	}
}
