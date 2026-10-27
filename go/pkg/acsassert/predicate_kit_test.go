package acsassert

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func writeTree(t *testing.T, root string, files ...string) {
	t.Helper()
	for _, rel := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestModuleDir_FindsTheNearestGoModAboveTheStart(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, "go.mod", "pkg/deep/file.go", "inner/go.mod", "inner/sub/file.go")
	cases := []struct {
		name, start, want string
	}{
		{"module root itself", root, root},
		{"nested directory", filepath.Join(root, "pkg", "deep"), root},
		{"file in nested directory", filepath.Join(root, "pkg", "deep", "file.go"), root},
		{"inner module wins over the outer one", filepath.Join(root, "inner", "sub"), filepath.Join(root, "inner")},
		{"go.mod file itself", filepath.Join(root, "go.mod"), root},
	}
	for _, tc := range cases {
		got, err := ModuleDir(tc.start)
		if err != nil {
			t.Errorf("%s: ModuleDir(%q) error: %v", tc.name, tc.start, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%s: ModuleDir(%q) = %q, want %q", tc.name, tc.start, got, tc.want)
		}
	}
}

func TestModuleDir_RefusesAStartWithNoGoModAboveIt(t *testing.T) {
	t.Parallel()
	bare := t.TempDir()
	writeTree(t, bare, "a/b/file.go")
	cases := []struct {
		name, start, wantInError string
	}{
		{"no go.mod in any ancestor", filepath.Join(bare, "a", "b"), "go.mod"},
		{"start does not exist", filepath.Join(bare, "missing"), "missing"},
		{"empty start", "", "start"},
	}
	for _, tc := range cases {
		got, err := ModuleDir(tc.start)
		if err == nil {
			t.Errorf("%s: ModuleDir(%q) = %q, want an error", tc.name, tc.start, got)
			continue
		}
		if got != "" {
			t.Errorf("%s: ModuleDir(%q) returned %q beside the error", tc.name, tc.start, got)
		}
		if !strings.Contains(err.Error(), tc.wantInError) {
			t.Errorf("%s: error %q does not name %q", tc.name, err, tc.wantInError)
		}
	}
}

func TestModuleDir_ResolvesTheRealModuleOfThisPackage(t *testing.T) {
	t.Parallel()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	got, err := ModuleDir(wd)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(got, "go.mod")); err != nil {
		t.Errorf("ModuleDir(%q) = %q holds no go.mod: %v", wd, got, err)
	}
	if filepath.Base(got) != "go" {
		t.Errorf("ModuleDir(%q) = %q, want the go/ module directory", wd, got)
	}
}

func TestOutputTail_KeepsTheLastLines(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		output string
		n      int
		want   string
	}{
		{"last two of three, trailing newline", "a\nb\nc\n", 2, "b\nc"},
		{"last two of three, no trailing newline", "a\nb\nc", 2, "b\nc"},
		{"one line", "a\nb\nc\n", 1, "c"},
		{"all lines when n equals the count", "a\nb\nc\n", 3, "a\nb\nc"},
		{"all lines when n exceeds the count", "a\nb", 5, "a\nb"},
		{"blank lines count as lines", "a\n\n\nb\n", 2, "\nb"},
		{"only the final newline is dropped", "a\nb\n\n", 2, "b\n"},
		{"empty output", "", 3, ""},
		{"single line without newline", "only", 1, "only"},
	}
	for _, tc := range cases {
		if got := OutputTail(tc.output, tc.n); got != tc.want {
			t.Errorf("%s: OutputTail(%q, %d) = %q, want %q", tc.name, tc.output, tc.n, got, tc.want)
		}
	}
}

func TestOutputTail_ReturnsNothingForANonPositiveCount(t *testing.T) {
	t.Parallel()
	for _, n := range []int{0, -1, -100} {
		if got := OutputTail("a\nb\nc\n", n); got != "" {
			t.Errorf("OutputTail(_, %d) = %q, want empty", n, got)
		}
	}
}

const coverFuncSample = "github.com/x/y/pkg/a.go:10:\tAlpha\t100.0%\n" +
	"github.com/x/y/pkg/a.go:20:\tBeta\t85.7%\n" +
	"github.com/x/y/pkg/b.go:5:\tGamma\t0.0%\n" +
	"github.com/x/y/pkg/b.go:30:\tDup\t100.0%\n" +
	"github.com/x/y/pkg/c.go:8:\tDup\t50.0%\n" +
	"total:\t\t\t\t(statements)\t91.2%\n"

func TestFuncCoverage_ReadsTheCoverageOfOneFunction(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, funcName string
		want           float64
	}{
		{"fully covered", "Alpha", 100},
		{"partly covered", "Beta", 85.7},
		{"uncovered", "Gamma", 0},
		{"a name in two files reports the lower figure", "Dup", 50},
	}
	for _, tc := range cases {
		got, err := FuncCoverage(coverFuncSample, tc.funcName)
		if err != nil {
			t.Errorf("%s: FuncCoverage(%q) error: %v", tc.name, tc.funcName, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%s: FuncCoverage(%q) = %v, want %v", tc.name, tc.funcName, got, tc.want)
		}
	}
}

func TestFuncCoverage_RefusesAnOutputThatDoesNotAnswer(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, output, funcName, wantInError string
	}{
		{"function absent", coverFuncSample, "Missing", "Missing"},
		{"a prefix is not the function", coverFuncSample, "Alph", "Alph"},
		{"the total line is not a function", coverFuncSample, "total:", "total:"},
		{"empty output", "", "Alpha", "Alpha"},
		{"percent is not a number", "p/a.go:1:\tAlpha\tabc%\n", "Alpha", "Alpha"},
		{"percent has no percent sign", "p/a.go:1:\tAlpha\t100.0\n", "Alpha", "Alpha"},
	}
	for _, tc := range cases {
		got, err := FuncCoverage(tc.output, tc.funcName)
		if err == nil {
			t.Errorf("%s: FuncCoverage(%q) = %v, want an error", tc.name, tc.funcName, got)
			continue
		}
		if !strings.Contains(err.Error(), tc.wantInError) {
			t.Errorf("%s: error %q does not name %q", tc.name, err, tc.wantInError)
		}
	}
}

func TestFuncCoverage_SkipsAMalformedLineAndStillFindsTheFunction(t *testing.T) {
	t.Parallel()
	got, err := FuncCoverage("short line\np/a.go:1:\tAlpha\t90.0%\n", "Alpha")
	if err != nil || got != 90 {
		t.Errorf("FuncCoverage = (%v, %v), want (90, nil)", got, err)
	}
}

func TestWalkModule_ListsTheFilesTheFilterKeeps(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, "go.mod", "main.go", "pkg/a.go", "pkg/a_test.go", "pkg/deep/b.go", "docs/readme.md", ".git/config")
	isGo := func(rel string) bool { return strings.HasSuffix(rel, ".go") }
	notTest := func(rel string) bool { return isGo(rel) && !strings.HasSuffix(rel, "_test.go") }
	underPkg := func(rel string) bool { return strings.HasPrefix(rel, "pkg/") }
	cases := []struct {
		name string
		keep func(string) bool
		want []string
	}{
		{"nil filter keeps every file but the git directory", nil, []string{"docs/readme.md", "go.mod", "main.go", "pkg/a.go", "pkg/a_test.go", "pkg/deep/b.go"}},
		{"suffix filter", isGo, []string{"main.go", "pkg/a.go", "pkg/a_test.go", "pkg/deep/b.go"}},
		{"suffix filter without tests", notTest, []string{"main.go", "pkg/a.go", "pkg/deep/b.go"}},
		{"prefix filter", underPkg, []string{"pkg/a.go", "pkg/a_test.go", "pkg/deep/b.go"}},
		{"filter that keeps nothing", func(string) bool { return false }, nil},
	}
	for _, tc := range cases {
		got, err := WalkModule(root, tc.keep)
		if err != nil {
			t.Errorf("%s: WalkModule error: %v", tc.name, err)
			continue
		}
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s: WalkModule = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestWalkModule_ReturnsNothingForAnEmptyDirectory(t *testing.T) {
	t.Parallel()
	got, err := WalkModule(t.TempDir(), nil)
	if err != nil || len(got) != 0 {
		t.Errorf("WalkModule(empty) = (%v, %v), want no files and no error", got, err)
	}
}

func TestWalkModule_RefusesARootItCannotWalk(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTree(t, dir, "plain.txt")
	cases := []struct {
		name, root, wantInError string
	}{
		{"root does not exist", filepath.Join(dir, "missing"), "missing"},
		{"root is a file", filepath.Join(dir, "plain.txt"), "plain.txt"},
		{"root is empty", "", "root"},
	}
	for _, tc := range cases {
		got, err := WalkModule(tc.root, nil)
		if err == nil {
			t.Errorf("%s: WalkModule(%q) = %v, want an error", tc.name, tc.root, got)
			continue
		}
		if got != nil {
			t.Errorf("%s: WalkModule(%q) returned %v beside the error", tc.name, tc.root, got)
		}
		if !strings.Contains(err.Error(), tc.wantInError) {
			t.Errorf("%s: error %q does not name %q", tc.name, err, tc.wantInError)
		}
	}
}

func TestWalkModule_ReportsADirectoryItCannotRead(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("directory permissions do not block this user")
	}
	root := t.TempDir()
	writeTree(t, root, "ok.go", "locked/inner.go")
	locked := filepath.Join(root, "locked")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	got, err := WalkModule(root, nil)
	if err == nil {
		t.Fatalf("WalkModule = %v, want an error for the unreadable directory", got)
	}
	if got != nil {
		t.Errorf("WalkModule returned %v beside the error", got)
	}
	if !strings.Contains(err.Error(), "locked") {
		t.Errorf("error %q does not name the unreadable directory", err)
	}
}
