package coversplit

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParse_ReadsEachSplitAndRefusesEveryMalformedLine(t *testing.T) {
	t.Parallel()
	got, err := Parse([]byte("# the splits\n\n./a/at ./a\n  ./b/bt   ./b  \n"))
	if err != nil || !reflect.DeepEqual(got, []Split{{"./a/at", "./a"}, {"./b/bt", "./b"}}) {
		t.Fatalf("Parse = %v, %v", got, err)
	}
	for name, line := range map[string]string{
		"one field":        "./a/at",
		"three fields":     "./a/at ./a ./b",
		"not relative":     "a/at ./a",
		"a pattern":        "./a/... ./a",
		"unclean":          "./a/../at ./a",
		"the module root":  "./. ./a",
		"itself":           "./a ./a",
		"listed twice":     "./a/at ./a\n./a/at ./b",
		"an absolute path": "./a/at /a",
	} {
		if _, err := Parse([]byte(line)); err == nil || !strings.Contains(err.Error(), File) {
			t.Errorf("%s: Parse(%q) err = %v, want a refusal naming %s", name, line, err, File)
		}
	}
}

func TestLoad_AMissingListIsNoSplitAndAnUnreadableOneIsAnError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if got, err := Load(dir); err != nil || got != nil {
		t.Fatalf("no list: %v, %v", got, err)
	}
	if err := os.Mkdir(filepath.Join(dir, File), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil {
		t.Fatal("a list that cannot be read must be an error")
	}
	other := t.TempDir()
	if err := os.WriteFile(filepath.Join(other, File), []byte("./x/xt ./x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := Load(other); err != nil || len(got) != 1 {
		t.Fatalf("a list: %v, %v", got, err)
	}
}

func TestPlan_ASelectedTargetRunsItsSplitPackageApartWithItsCoverage(t *testing.T) {
	t.Parallel()
	splits := []Split{{"./s/st", "./s"}, {"./u/ut", "./u"}}
	got := Plan([]string{"example.com/m/a", "example.com/m/s", "example.com/m/s/st", "example.com/m/u/ut"}, splits)
	want := []Run{
		{Packages: []string{"example.com/m/a", "example.com/m/s", "example.com/m/u/ut"}},
		{Packages: []string{"./s/st"}, CoverPkg: "./s,./s/st"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Plan = %+v, want %+v", got, want)
	}
	if got := Plan([]string{"./s"}, splits); len(got) != 2 || got[0].Packages[0] != "./s" || got[1].Packages[0] != "./s/st" {
		t.Fatalf("a target alone still runs its split package: %+v", got)
	}
	if got := Plan([]string{"example.com/m/xs", "example.com/m/s/stx"}, splits); len(got) != 1 || len(got[0].Packages) != 2 {
		t.Fatalf("a package whose path only ends in the same letters is not the tested package: %+v", got)
	}
	if got := Plan(nil, splits); got != nil {
		t.Fatalf("no package plans no run: %+v", got)
	}
}

func TestRunArgs_PutTheCoverageFlagBeforeThePackages(t *testing.T) {
	t.Parallel()
	if got := (Run{Packages: []string{"./a"}}).Args(); !reflect.DeepEqual(got, []string{"./a"}) {
		t.Fatalf("plain run args %v", got)
	}
	if got := (Run{Packages: []string{"./s/st"}, CoverPkg: "./s,./s/st"}).Args(); !reflect.DeepEqual(got, []string{"-coverpkg=./s,./s/st", "./s/st"}) {
		t.Fatalf("split run args %v", got)
	}
}

func TestMerge_JoinsTheBlocksUnderOneModeLine(t *testing.T) {
	t.Parallel()
	got, err := Merge([][]byte{[]byte("mode: atomic\na.go:1.1,2.2 1 1\n"), []byte("mode: atomic\na.go:1.1,2.2 1 0"), []byte("mode: atomic\n")})
	if want := "mode: atomic\na.go:1.1,2.2 1 1\na.go:1.1,2.2 1 0\n"; err != nil || string(got) != want {
		t.Fatalf("Merge = %q, %v, want %q", got, err, want)
	}
	for name, profiles := range map[string][][]byte{
		"no profile":   nil,
		"no mode line": {[]byte("a.go:1.1,2.2 1 1\n")},
		"another mode": {[]byte("mode: atomic\n"), []byte("mode: set\n")},
	} {
		if _, err := Merge(profiles); err == nil {
			t.Errorf("%s: Merge must refuse", name)
		}
	}
}

func TestMain_PrintsOneArgumentLinePerRun(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, File), []byte("./s/st ./s\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := Main([]string{"-dir", dir, "m/a", "m/s"}, &out, &errOut); code != 0 || out.String() != "m/a m/s\n-coverpkg=./s,./s/st ./s/st\n" {
		t.Fatalf("Main = %d, %q (stderr %q)", code, out.String(), errOut.String())
	}
	if code := Main([]string{"-bogus"}, &out, &errOut); code != 2 {
		t.Fatalf("a bad flag exits %d, want 2", code)
	}
	if err := os.WriteFile(filepath.Join(dir, File), []byte("bad\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	errOut.Reset()
	if code := Main([]string{"-dir", dir, "m/s"}, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), File) {
		t.Fatalf("a bad list exits %d with %q, want 1 naming the list", code, errOut.String())
	}
}

func TestMain_MergesTheNamedProfilesIntoOneFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	write := func(name, body string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	main := write("main.txt", "mode: atomic\na.go:1.1,2.2 1 0\n")
	split := write("split.txt", "mode: atomic\na.go:1.1,2.2 1 1\n")
	out := filepath.Join(dir, "merged.txt")
	var stdout, stderr bytes.Buffer
	if code := Main([]string{"-merge", out, main, split}, &stdout, &stderr); code != 0 {
		t.Fatalf("merge exited %d: %s", code, stderr.String())
	}
	if got, err := os.ReadFile(out); err != nil || string(got) != "mode: atomic\na.go:1.1,2.2 1 0\na.go:1.1,2.2 1 1\n" {
		t.Fatalf("merged profile %q, %v", got, err)
	}
	for name, args := range map[string][]string{
		"an absent input":      {"-merge", out, filepath.Join(dir, "absent.txt")},
		"another mode":         {"-merge", out, main, write("set.txt", "mode: set\n")},
		"an unwritable output": {"-merge", filepath.Join(dir, "no", "such", "dir", "out.txt"), main},
	} {
		stderr.Reset()
		if code := Main(args, &stdout, &stderr); code != 1 || stderr.Len() == 0 {
			t.Errorf("%s: exit %d stderr %q, want 1 with a reason", name, code, stderr.String())
		}
	}
}
