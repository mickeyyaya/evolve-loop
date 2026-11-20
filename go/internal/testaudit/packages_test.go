package testaudit

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

const cleanTest = "package fx\nimport \"testing\"\nfunc TestOK(t *testing.T) { t.Fatal(\"x\") }\n"

func rowSet(err error) map[RowError]bool {
	set := map[RowError]bool{}
	for _, p := range Problems(err) {
		var re *RowError
		if errors.As(p, &re) {
			set[*re] = true
		}
	}
	return set
}

func hasRow(err error, kind RowKind, f Finding) bool {
	return rowSet(err)[RowError{Kind: kind, Finding: f}]
}

func TestRowError_CarriesItsKindAndWrapsTheSentinel(t *testing.T) {
	t.Parallel()
	f := Finding{Package: "p", Test: "TestA", Rule: "r"}
	missing := &RowError{Kind: RowMissing, Finding: f}
	stale := &RowError{Kind: RowStale, Finding: f}
	if !errors.Is(missing, ErrNoBaselineRow) || errors.Is(missing, ErrStaleBaselineRow) {
		t.Errorf("a missing row must wrap only ErrNoBaselineRow")
	}
	if !errors.Is(stale, ErrStaleBaselineRow) || errors.Is(stale, ErrNoBaselineRow) {
		t.Errorf("a stale row must wrap only ErrStaleBaselineRow")
	}
	if got, want := missing.Error(), "testaudit: p TestA r has no baseline row"; got != want {
		t.Errorf("missing message = %q, want %q", got, want)
	}
	if got, want := stale.Error(), "testaudit: baseline row p TestA r matches no finding"; got != want {
		t.Errorf("stale message = %q, want %q", got, want)
	}
}

func TestProblems_FlattensNestedJoinsAndKeepsPlainErrors(t *testing.T) {
	t.Parallel()
	if got := Problems(nil); got != nil {
		t.Errorf("Problems(nil) = %v, want nil", got)
	}
	a, b, c := errors.New("a"), errors.New("b"), errors.New("c")
	got := Problems(errors.Join(a, errors.Join(b, c)))
	if !reflect.DeepEqual(got, []error{a, b, c}) {
		t.Errorf("Problems = %v, want [a b c]", got)
	}
	if got := Problems(a); !reflect.DeepEqual(got, []error{a}) {
		t.Errorf("Problems(single) = %v", got)
	}
}

func TestDiffFindings_ReturnsTypedRowErrors(t *testing.T) {
	t.Parallel()
	added := Finding{Package: "p", Test: "TestNew", Rule: "r"}
	gone := Finding{Package: "p", Test: "TestOld", Rule: "r"}
	err := diffFindings([]Finding{added}, []Finding{gone})
	if !hasRow(err, RowMissing, added) || !hasRow(err, RowStale, gone) || len(rowSet(err)) != 2 {
		t.Errorf("diffFindings rows = %v", rowSet(err))
	}
	if !errors.Is(err, ErrNoBaselineRow) || !errors.Is(err, ErrStaleBaselineRow) {
		t.Errorf("diffFindings must wrap both sentinels: %v", err)
	}
}

func TestCheckPackages_RefusesAFindingWithNoBaselineRow(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "pkg", "a", "a_test.go"), redTest)
	err := CheckPackages(root, []string{"pkg/a"}, Config{})
	if !hasRow(err, RowMissing, Finding{Package: "pkg/a", Test: "TestNothing", Rule: ruleNoAssertion}) {
		t.Errorf("CheckPackages = %v, want a missing row for pkg/a", err)
	}
}

func TestCheckPackages_PassesACleanPackageAndABaselinedFinding(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "pkg", "clean", "c_test.go"), cleanTest)
	mustWrite(t, filepath.Join(root, "pkg", "red", "r_test.go"), redTest)
	if err := Rebaseline(root, "pkg/red", Config{}); err != nil {
		t.Fatal(err)
	}
	if err := CheckPackages(root, []string{"pkg/clean", "pkg/red", "./pkg/red/", "pkg//clean"}, Config{}); err != nil {
		t.Errorf("CheckPackages = %v, want nil", err)
	}
}

func TestCheckPackages_RefusesAStaleLocalRow(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "pkg", "a", "a_test.go"), redTest)
	mustWrite(t, filepath.Join(root, "pkg", "a", shardRelPath), `{"rows":[{"package":"pkg/a","test":"TestNothing","rule":"no-assertion"},{"package":"pkg/a","test":"TestGone","rule":"no-assertion"}]}`)
	err := CheckPackages(root, []string{"pkg/a"}, Config{})
	if !hasRow(err, RowStale, Finding{Package: "pkg/a", Test: "TestGone", Rule: ruleNoAssertion}) || len(rowSet(err)) != 1 {
		t.Errorf("CheckPackages rows = %v", rowSet(err))
	}
}

func TestCheckPackages_ReadsTheCentralRowsOfThePackage(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	central := `{"packages":["pkg/a","pkg/b"],"rows":[{"package":"pkg/a","test":"TestNothing","rule":"no-assertion"},{"package":"pkg/b","test":"TestStale","rule":"no-assertion"}]}`
	mustWrite(t, filepath.Join(root, filepath.FromSlash(centralBaselinesRelPath)), central)
	mustWrite(t, filepath.Join(root, "pkg", "a", "a_test.go"), redTest)
	mustWrite(t, filepath.Join(root, "pkg", "b", "b_test.go"), cleanTest)
	if err := CheckPackages(root, []string{"pkg/a"}, Config{}); err != nil {
		t.Errorf("pkg/a central row must match: %v", err)
	}
	err := CheckPackages(root, []string{"pkg/b"}, Config{})
	if !hasRow(err, RowStale, Finding{Package: "pkg/b", Test: "TestStale", Rule: ruleNoAssertion}) {
		t.Errorf("pkg/b stale central row missing: %v", err)
	}
}

func TestCheckPackages_ReportsTheStaleRowsOfADeletedPackage(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, filepath.FromSlash(centralBaselinesRelPath)), `{"packages":["pkg/gone"],"rows":[{"package":"pkg/gone","test":"TestNothing","rule":"no-assertion"}]}`)
	err := CheckPackages(root, []string{"pkg/gone"}, Config{})
	if !hasRow(err, RowStale, Finding{Package: "pkg/gone", Test: "TestNothing", Rule: ruleNoAssertion}) {
		t.Errorf("deleted package must report its central row stale: %v", err)
	}
}

func TestCheckPackages_ChecksTheFakeRowOfAPackageWithoutADirectory(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cfg := Config{Fakes: []Fake{{Tool: "git", SuiteDir: "pkg/gone", RealSide: "x"}}}
	err := CheckPackages(root, []string{"pkg/gone"}, cfg)
	if !hasRow(err, RowMissing, Finding{Package: "pkg/gone", Test: "fakes:git", Rule: ruleUnverifiedFake}) {
		t.Errorf("a fake row of a missing suite dir must be a finding: %v", err)
	}
}

func TestCheckPackages_DropsPackagesInSkippedDirectories(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, d := range []string{"pkg/testdata/x", "vendor/v", ".hidden/h", "_skip/s"} {
		mustWrite(t, filepath.Join(root, filepath.FromSlash(d), "x_test.go"), redTest)
	}
	pkgs := []string{"pkg/testdata/x", "vendor/v", ".hidden/h", "_skip/s", "../out"}
	if err := CheckPackages(root, pkgs, Config{}); err != nil {
		t.Errorf("CheckPackages = %v, want skipped packages ignored", err)
	}
}

func TestCheckPackages_NormalizesTheRootPackage(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "r_test.go"), redTest)
	want := Finding{Package: ".", Test: "TestNothing", Rule: ruleNoAssertion}
	for _, in := range [][]string{{"."}, {""}, {"./"}} {
		if err := CheckPackages(root, in, Config{}); !hasRow(err, RowMissing, want) {
			t.Errorf("CheckPackages(%q) = %v, want the root finding", in, err)
		}
	}
}

func TestCheckPackages_IgnoresARedPackageOutsideTheScope(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "pkg", "in", "i_test.go"), cleanTest)
	mustWrite(t, filepath.Join(root, "pkg", "out", "o_test.go"), redTest)
	if err := CheckPackages(root, []string{"pkg/in"}, Config{}); err != nil {
		t.Errorf("CheckPackages = %v, want a red package outside the scope ignored", err)
	}
	if err := CheckTree(root, Config{}); err == nil {
		t.Error("CheckTree must still refuse the red package")
	}
}

func TestCheckPackages_ReportsABrokenInputInsteadOfPassing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "pkg", "bad", "b_test.go"), "package fx\nfunc {")
	mustWrite(t, filepath.Join(root, "pkg", "shard", "s_test.go"), cleanTest)
	mustWrite(t, filepath.Join(root, "pkg", "shard", shardRelPath), "{")
	mustWrite(t, filepath.Join(root, "pkg", "ok", "o_test.go"), cleanTest)
	mustWrite(t, filepath.Join(root, "pkg", "ok", shardRelPath), `{"rows":[]}`)
	err := CheckPackages(root, []string{"pkg/bad", "pkg/shard"}, Config{})
	if got := len(Problems(err)); got != 2 {
		t.Errorf("CheckPackages problems = %d (%v), want 2", got, err)
	}
	if err := CheckPackages(root, []string{"pkg/ok"}, Config{}); err != nil {
		t.Errorf("an empty shard must pass: %v", err)
	}
}

func TestPackageFindings_ReportsAnUnreadableDirectory(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "pkg", "file"), "not a dir")
	if _, err := packageFindings(root, "pkg/file", Config{}); err == nil || errors.Is(err, fs.ErrNotExist) {
		t.Errorf("packageFindings(file) = %v, want a non-ENOENT error", err)
	}
	cfg := Config{Fakes: []Fake{{Tool: "git", SuiteDir: "pkg/suite", RealSide: "x"}}}
	mustWrite(t, filepath.Join(root, "pkg", "suite", "bad.go"), "package suite\nfunc {")
	if _, err := packageFindings(root, "pkg/other", cfg); err != nil {
		t.Errorf("a fake row of another package must be ignored: %v", err)
	}
	if _, err := packageFindings(root, "pkg/suite", cfg); err == nil {
		t.Error("packageFindings accepted a broken suite dir")
	}
}

func TestPackageBaseline_UnionsTheLocalShardAndTheCentralRows(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	local := Finding{Package: "pkg/a", Test: "TestL", Rule: ruleNoAssertion}
	central := Finding{Package: "pkg/a", Test: "TestC", Rule: ruleNoAssertion}
	other := Finding{Package: "pkg/b", Test: "TestB", Rule: ruleNoAssertion}
	if err := WriteShard(filepath.Join(root, "pkg", "a"), []Finding{local}); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(root, filepath.FromSlash(centralBaselinesRelPath)), `{"packages":["pkg/a","pkg/b"],"rows":[{"package":"pkg/a","test":"TestC","rule":"no-assertion"},{"package":"pkg/b","test":"TestB","rule":"no-assertion"}]}`)
	got, err := packageBaseline(root, "pkg/a")
	if err != nil || !reflect.DeepEqual(got, []Finding{local, central}) {
		t.Errorf("packageBaseline = %v, %v, want local+central of pkg/a without %v", got, err, other)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(centralBaselinesRelPath)), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := packageBaseline(root, "pkg/a"); err == nil {
		t.Error("packageBaseline accepted a malformed central file")
	}
	mustWrite(t, filepath.Join(root, "pkg", "m", shardRelPath), "{")
	if _, err := packageBaseline(root, "pkg/m"); err == nil {
		t.Error("packageBaseline accepted a malformed shard")
	}
}

func buildParityModule(t *testing.T) (string, Config) {
	t.Helper()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "pkg", "new", "n_test.go"), redTest)
	mustWrite(t, filepath.Join(root, "pkg", "clean", "c_test.go"), cleanTest)
	mustWrite(t, filepath.Join(root, "pkg", "local", "l_test.go"), redTest)
	mustWrite(t, filepath.Join(root, "pkg", "local", shardRelPath), `{"rows":[{"package":"pkg/local","test":"TestNothing","rule":"no-assertion"},{"package":"pkg/local","test":"TestOld","rule":"no-assertion"}]}`)
	mustWrite(t, filepath.Join(root, "pkg", "central", "c_test.go"), redTest)
	mustWrite(t, filepath.Join(root, "pkg", "fake", "f_test.go"), cleanTest)
	mustWrite(t, filepath.Join(root, "pkg", "testdata", "td", "t_test.go"), redTest)
	mustWrite(t, filepath.Join(root, filepath.FromSlash(centralBaselinesRelPath)), `{"packages":["pkg/central","pkg/gone"],"rows":[{"package":"pkg/central","test":"TestNothing","rule":"no-assertion"},{"package":"pkg/central","test":"TestStale","rule":"no-assertion"},{"package":"pkg/gone","test":"TestNothing","rule":"no-assertion"}]}`)
	return root, Config{Fakes: []Fake{{Tool: "git", SuiteDir: "pkg/fake", RealSide: "x"}}}
}

func allPackages(t *testing.T, root string) []string {
	t.Helper()
	pkgs := []string{"pkg/gone"}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			pkgs = append(pkgs, relSlash(root, path))
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return pkgs
}

func TestCheckPackages_EqualsCheckTreeOnAFixtureModule(t *testing.T) {
	t.Parallel()
	root, cfg := buildParityModule(t)
	tree := rowSet(CheckTree(root, cfg))
	if len(tree) == 0 {
		t.Fatal("the fixture must carry rows")
	}
	scoped := rowSet(CheckPackages(root, allPackages(t, root), cfg))
	if !reflect.DeepEqual(scoped, tree) {
		t.Errorf("CheckPackages rows differ from CheckTree:\nscoped %v\ntree   %v", scoped, tree)
	}
}

func TestCheckPackages_EqualsCheckTreeOnTheModule(t *testing.T) {
	t.Parallel()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root, err := findModuleRoot(wd)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := DefaultConfig()
	if err != nil {
		t.Fatal(err)
	}
	tree := rowSet(CheckTree(root, cfg))
	scoped := rowSet(CheckPackages(root, allPackages(t, root), cfg))
	if !reflect.DeepEqual(scoped, tree) {
		t.Errorf("CheckPackages rows differ from CheckTree on the module:\nscoped %v\ntree   %v", scoped, tree)
	}
}
