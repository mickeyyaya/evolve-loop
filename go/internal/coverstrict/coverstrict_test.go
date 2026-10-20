package coverstrict

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/covchanged"
)

const strictModule = "example.com/m"

const strictProfile = "mode: atomic\n" +
	"example.com/m/p/a.go:3.10,4.5 2 1\n" +
	"example.com/m/p/a.go:5.1,6.5 1 0\n" +
	"example.com/m/p/b.go:3.10,4.5 1 3\n" +
	"example.com/m/q/q.go:3.10,4.5 4 1\n" +
	"example.com/m/pp/z.go:3.10,4.5 5 0\n" +
	"example.com/m/p/sub/s.go:3.10,4.5 6 0\n"

type strictTree struct {
	root    string
	profile string
	floors  string
}

func newStrictTree(t *testing.T, floors string) strictTree {
	t.Helper()
	root := t.TempDir()
	tree := strictTree{root: root, profile: filepath.Join(root, "coverage.txt"), floors: filepath.Join(root, ".cover-strict")}
	for rel, text := range map[string]string{
		"go.mod":        "module " + strictModule + "\n\ngo 1.23\n",
		"p/a.go":        "package p\n",
		"p/b.go":        "package p\n",
		"p/a_test.go":   "package p\n",
		"q/q.go":        "package q\n",
		"coverage.txt":  strictProfile,
		".cover-strict": floors,
	} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tree.touchProfile(t, time.Now().Add(time.Hour))
	return tree
}

func (s strictTree) touchProfile(t *testing.T, at time.Time) {
	t.Helper()
	if err := os.Chtimes(s.profile, at, at); err != nil {
		t.Fatal(err)
	}
}

func TestParseFloors_ReadsEveryLineIncludingTheLastWithoutANewline(t *testing.T) {
	got, err := ParseFloors([]byte("# why\n\n./internal/a 100\n  ./internal/b/c   87.5  \n./d 0"))

	want := []Floor{{"internal/a", 100}, {"internal/b/c", 87.5}, {"d", 0}}
	if err != nil || len(got) != len(want) {
		t.Fatalf("floors = %v, %v; want %v", got, err, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("floor %d = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestParseFloors_RefusesEveryInvalidLine(t *testing.T) {
	for name, line := range map[string]string{
		"a word for the floor":  "./p invalid",
		"a floor over 100":      "./p 100.1",
		"a signed floor":        "./p -1",
		"a floor with no digit": "./p .5",
		"a missing floor":       "./p",
		"an extra field":        "./p 100 extra",
		"a pattern with dots":   "./internal/... 100",
		"a parent path":         "../p 100",
		"an absolute path":      "/p 100",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseFloors([]byte("./ok 100\n" + line + "\n"))
			if err == nil || !strings.Contains(err.Error(), "floors line 2") {
				t.Errorf("err = %v; want a refusal that names line 2", err)
			}
		})
	}
}

func TestMeasure_CountsTheStatementsOfEveryFileOfExactlyThePackage(t *testing.T) {
	profile, err := covchanged.ParseProfile([]byte(strictProfile), strictModule)
	if err != nil {
		t.Fatal(err)
	}

	got := Measure([]Floor{{"p", 100}, {"q", 50}, {"p", 75}}, profile)

	if len(got) != 3 {
		t.Fatalf("results = %v", got)
	}
	if got[0].Statements != 4 || got[0].Covered != 3 || got[0].Percent != 75 || got[0].OK() {
		t.Errorf("p at floor 100 = %+v; want 3 of 4 statements, 75%%, refused", got[0])
	}
	if !got[1].OK() || got[1].Percent != 100 || got[1].Statements != 4 {
		t.Errorf("q = %+v; want 100%% of its own 4 statements and ok", got[1])
	}
	if !got[2].OK() {
		t.Errorf("p at floor 75 = %+v; want ok when the percent equals the floor", got[2])
	}
}

func TestMeasure_RefusesAPackageWithNoStatementsInTheProfile(t *testing.T) {
	profile, err := covchanged.ParseProfile([]byte(strictProfile), strictModule)
	if err != nil {
		t.Fatal(err)
	}

	got := Measure([]Floor{{"missing", 0}, {"p/none", 0}}, profile)

	for _, r := range got {
		if r.OK() || !strings.Contains(r.Problem, "no statements") {
			t.Errorf("%s = %+v; want a no-statements refusal even at floor 0", r.Pkg, r)
		}
	}
}

func TestClean_IsTrueOnlyWhenEveryPackageIsOk(t *testing.T) {
	ok, bad := Result{Pkg: "a"}, Result{Pkg: "b", Problem: "x"}

	if !Clean(nil) || !Clean([]Result{ok, ok}) || Clean([]Result{ok, bad}) || Clean([]Result{bad, ok}) {
		t.Error("Clean must hold for none or all ok, and fail when any package is refused")
	}
}

func TestResultLine_NamesThePackageTheFiguresAndTheVerdict(t *testing.T) {
	ok := Result{Pkg: "p", Min: 70, Percent: 75}
	bad := Result{Pkg: "p", Min: 100, Percent: 75, Problem: "75.0% < 100.0%"}

	if got := ok.Line(); got != "cover strict: p: 75.0% >= 70.0% ok" {
		t.Errorf("ok line = %q", got)
	}
	if got := bad.Line(); got != "cover strict: p: 75.0% < 100.0% FAIL" {
		t.Errorf("bad line = %q", got)
	}
}

func TestCheck_MeasuresTheProfileOfAFreshRun(t *testing.T) {
	tree := newStrictTree(t, "./p 70\n./q 100\n")

	got, err := Check(tree.root, tree.profile, tree.floors)

	if err != nil || len(got) != 2 || !got[0].OK() || !got[1].OK() {
		t.Errorf("results = %+v, %v; want both packages ok", got, err)
	}
}

func TestCheck_IgnoresFilesThatAreNotGoSources(t *testing.T) {
	tree := newStrictTree(t, "./p 70\n")
	notes := filepath.Join(tree.root, "p", "notes.txt")
	write(t, notes, "newer than the profile\n")
	later := time.Now().Add(2 * time.Hour)
	if err := os.Chtimes(notes, later, later); err != nil {
		t.Fatal(err)
	}

	got, err := Check(tree.root, tree.profile, tree.floors)

	if err != nil || len(got) != 1 || !got[0].OK() {
		t.Errorf("results = %+v, %v; want ok, since a text file is not a source", got, err)
	}
}

func ageTree(t *testing.T, s strictTree) {
	t.Helper()
	old := time.Now().Add(-3 * time.Hour)
	if err := filepath.WalkDir(s.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path == s.profile {
			return err
		}
		return os.Chtimes(path, old, old)
	}); err != nil {
		t.Fatal(err)
	}
	s.touchProfile(t, time.Now().Add(-time.Hour))
}

func TestCheck_RefusesAnyModuleInputNewerThanTheProfile(t *testing.T) {
	for name, rel := range map[string]string{
		"go.mod":                    "go.mod",
		"go.sum":                    "go.sum",
		"a package off the floor":   "z/deep/z.go",
		"a test file off the floor": "z/z_test.go",
	} {
		t.Run(name, func(t *testing.T) {
			tree := newStrictTree(t, "./p 70\n./q 100\n")
			write(t, filepath.Join(tree.root, "go.sum"), "sum\n")
			if err := os.MkdirAll(filepath.Join(tree.root, "z", "deep"), 0o755); err != nil {
				t.Fatal(err)
			}
			write(t, filepath.Join(tree.root, "z", "z_test.go"), "package z\n")
			write(t, filepath.Join(tree.root, "z", "deep", "z.go"), "package deep\n")
			ageTree(t, tree)
			if _, err := Check(tree.root, tree.profile, tree.floors); err != nil {
				t.Fatalf("an aged tree must pass: %v", err)
			}
			now := time.Now()
			if err := os.Chtimes(filepath.Join(tree.root, filepath.FromSlash(rel)), now, now); err != nil {
				t.Fatal(err)
			}

			_, err := Check(tree.root, tree.profile, tree.floors)

			if err == nil || !strings.Contains(err.Error(), "older than") || !strings.Contains(err.Error(), filepath.Base(rel)) {
				t.Errorf("err = %v; want a stale refusal that names %s", err, rel)
			}
		})
	}
}

func TestCheck_SkipsTestdataVendorAndDotDirectories(t *testing.T) {
	for _, dir := range []string{"testdata", "vendor", ".hidden", "p/testdata"} {
		t.Run(dir, func(t *testing.T) {
			tree := newStrictTree(t, "./p 70\n")
			if err := os.MkdirAll(filepath.Join(tree.root, filepath.FromSlash(dir)), 0o755); err != nil {
				t.Fatal(err)
			}
			skipped := filepath.Join(tree.root, filepath.FromSlash(dir), "x.go")
			write(t, skipped, "package x\n")
			ageTree(t, tree)
			now := time.Now()
			if err := os.Chtimes(skipped, now, now); err != nil {
				t.Fatal(err)
			}

			if _, err := Check(tree.root, tree.profile, tree.floors); err != nil {
				t.Errorf("a newer file under %s must not make the profile stale: %v", dir, err)
			}
		})
	}
}

func TestCheck_RefusesASourceWhoseInfoCannotBeRead(t *testing.T) {
	tree := newStrictTree(t, "./p 70\n")
	orig := entryInfo
	entryInfo = func(fs.DirEntry) (fs.FileInfo, error) { return nil, os.ErrPermission }
	t.Cleanup(func() { entryInfo = orig })

	_, err := Check(tree.root, tree.profile, tree.floors)

	if err == nil || !strings.Contains(err.Error(), "stat ") {
		t.Errorf("err = %v; want a stat refusal", err)
	}
}

func TestCheck_RefusesEveryInputItCannotTrust(t *testing.T) {
	for name, tc := range map[string]struct {
		prepare func(t *testing.T, s strictTree)
		want    string
	}{
		"no profile yet":  {func(t *testing.T, s strictTree) { remove(t, s.profile) }, "make -C go test-integration"},
		"a stale profile": {func(t *testing.T, s strictTree) { s.touchProfile(t, time.Now().Add(-time.Hour)) }, "older than"},
		"a stale test file": {func(t *testing.T, s strictTree) {
			s.touchProfile(t, time.Now().Add(-time.Hour))
			old := time.Now().Add(-2 * time.Hour)
			for _, f := range []string{"go.mod", "p/b.go", "q/q.go"} {
				if err := os.Chtimes(filepath.Join(s.root, f), old, old); err != nil {
					t.Fatal(err)
				}
			}
			future := time.Now().Add(-30 * time.Minute)
			if err := os.Chtimes(filepath.Join(s.root, "p/a.go"), old, old); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(filepath.Join(s.root, "p/a_test.go"), future, future); err != nil {
				t.Fatal(err)
			}
		}, "a_test.go"},
		"a profile path below a file": {func(t *testing.T, s strictTree) {}, "read the profile"},
		"no floors file":              {func(t *testing.T, s strictTree) { remove(t, s.floors) }, "floors file"},
		"a floors directory":          {func(t *testing.T, s strictTree) { remove(t, s.floors); mkdir(t, s.floors) }, "floors file"},
		"an invalid floor":            {func(t *testing.T, s strictTree) { write(t, s.floors, "./p invalid\n") }, "floors line 1"},
		"a missing package":           {func(t *testing.T, s strictTree) { write(t, s.floors, "./gone 100\n") }, "list the sources of gone"},
		"an unreadable directory": {func(t *testing.T, s strictTree) {
			locked := filepath.Join(s.root, "q", "locked")
			mkdir(t, locked)
			if err := os.Chmod(locked, 0); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
		}, "walk"},
		"no go.mod": {func(t *testing.T, s strictTree) { remove(t, filepath.Join(s.root, "go.mod")) }, "go.mod"},
		"a garbled profile": {func(t *testing.T, s strictTree) {
			write(t, s.profile, "garbage\n")
			s.touchProfile(t, time.Now().Add(time.Hour))
		}, "mode line"},
		"an unreadable profile": {func(t *testing.T, s strictTree) {
			remove(t, s.profile)
			mkdir(t, s.profile)
			s.touchProfile(t, time.Now().Add(time.Hour))
		}, "read the profile"},
	} {
		t.Run(name, func(t *testing.T) {
			tree := newStrictTree(t, "./p 70\n./q 100\n")
			tc.prepare(t, tree)

			profile := tree.profile
			if name == "a profile path below a file" {
				profile = filepath.Join(tree.root, "go.mod", "coverage.txt")
			}

			_, err := Check(tree.root, profile, tree.floors)

			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v; want a refusal that holds %q", err, tc.want)
			}
		})
	}
}

func remove(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
}

func mkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func link(t *testing.T, path string) {
	t.Helper()
	if err := os.Symlink(filepath.Join(filepath.Dir(path), "nowhere"), path); err != nil {
		t.Fatal(err)
	}
}

func TestMeasure_APercentJustUnderTheFloorNeverPrintsAsTheFloor(t *testing.T) {
	var lines strings.Builder
	lines.WriteString("mode: atomic\n")
	lines.WriteString("example.com/m/p/a.go:3.10,4.5 3412 1\n")
	lines.WriteString("example.com/m/p/a.go:5.1,6.5 1 0\n")
	profile, err := covchanged.ParseProfile([]byte(lines.String()), strictModule)
	if err != nil {
		t.Fatal(err)
	}

	got := Measure([]Floor{{"p", 100}}, profile)

	want := "99.97% (3412 of 3413 statements) < 100.0%"
	if len(got) != 1 || got[0].Problem != want {
		t.Fatalf("problem = %q; want %q: 99.97 must not round up to the floor it misses", got[0].Problem, want)
	}
}

func TestMeasure_APercentThatRoundsBelowTheFloorKeepsTheShortForm(t *testing.T) {
	profile, err := covchanged.ParseProfile([]byte(strictProfile), strictModule)
	if err != nil {
		t.Fatal(err)
	}

	got := Measure([]Floor{{"p", 100}}, profile)

	if got[0].Problem != "75.0% < 100.0%" {
		t.Errorf("problem = %q; a percent that reads below the floor keeps one decimal", got[0].Problem)
	}
}

func TestWithin_KeepsOnlyThePackagesOfTheSetAndEveryPackageForANilSet(t *testing.T) {
	results := []Result{{Pkg: "p"}, {Pkg: "q", Problem: "below"}, {Pkg: "p/sub"}}

	got := Within(results, covchanged.PackageSet{"p": true})

	if len(got) != 1 || got[0].Pkg != "p" {
		t.Errorf("Within = %+v, want only p", got)
	}
	if all := Within(results, nil); len(all) != 3 || Clean(all) {
		t.Errorf("Within(nil) = %+v, want every result kept, the refusal included", all)
	}
	if none := Within(results, covchanged.PackageSet{"r": true}); none == nil || len(none) != 0 {
		t.Errorf("Within of a disjoint set = %#v, want an empty, non-nil list", none)
	}
}
