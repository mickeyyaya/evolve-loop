package testmainexit

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/rawgitratchet"
)

func writeTestFile(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "x_test.go")
	if err := os.WriteFile(p, []byte("package x\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\n"+body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSkippedDefers_FindsACleanupThatOsExitSkips(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		want       []int
	}{
		{"a deferred cleanup before os.Exit(m.Run()) is skipped", "func TestMain(m *testing.M) {\n\tdir, _ := os.MkdirTemp(\"\", \"x\")\n\tdefer os.RemoveAll(dir)\n\tos.Exit(m.Run())\n}\n", []int{10}},
		{"a TestMain that returns runs its defer", "func TestMain(m *testing.M) {\n\tdir, _ := os.MkdirTemp(\"\", \"x\")\n\tdefer os.RemoveAll(dir)\n\tm.Run()\n}\n", nil},
		{"os.Exit without a defer skips nothing", "func TestMain(m *testing.M) {\n\tos.Exit(m.Run())\n}\n", nil},
		{"cleanup run before os.Exit is fine", "func TestMain(m *testing.M) {\n\tdir, _ := os.MkdirTemp(\"\", \"x\")\n\tcode := m.Run()\n\t_ = os.RemoveAll(dir)\n\tos.Exit(code)\n}\n", nil},
		{"a defer in an ordinary test is not TestMain's", "func TestX(t *testing.T) {\n\tdefer os.Exit(0)\n}\n", nil},
		{"a defer inside a closure is the closure's", "func TestMain(m *testing.M) {\n\tfunc() { defer os.RemoveAll(\"x\") }()\n\tos.Exit(m.Run())\n}\n", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SkippedDefers(writeTestFile(t, tc.body))
			if err != nil || !slices.Equal(got, tc.want) {
				t.Fatalf("SkippedDefers = %v (%v), want %v", got, err, tc.want)
			}
		})
	}
	if _, err := SkippedDefers(filepath.Join(t.TempDir(), "absent_test.go")); err == nil {
		t.Fatal("an unreadable file is an error, never a clean result")
	}
}

func TestModuleTestMainsNeverDeferCleanupPastOsExit(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	files, _, err := rawgitratchet.BoundTestFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range files {
		lines, err := SkippedDefers(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range lines {
			t.Errorf("%s:%d: TestMain defers a cleanup and then calls os.Exit, which skips it (a leaked temp dir per run); return from TestMain instead", rel, line)
		}
	}
}
