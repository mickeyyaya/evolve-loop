package gitstarts

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func writeSample(t *testing.T, dir, name, body string) {
	t.Helper()
	src := "package sample\n\nfunc f() {\n" + body + "\n}\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCount_WeighsCallsAndIgnoresNonTestFilesAndSubdirs(t *testing.T) {
	dir := t.TempDir()
	writeSample(t, dir, "a_test.go", `gittest.Fixture(t)
gittest.Bare(t)
gittest.At(t, dir)
gittest.Clone(t, src)
gittest.FromTemplate(t, "tpl", build)
gittest.BareFromTemplate(t)
r.Git("status")
exec.Command("git", "init")
exec.Command("tar", "-xf", "x.tar")
fmt.Println("gitignore")`)
	writeSample(t, dir, "prod.go", `gittest.Fixture(t)`)
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	writeSample(t, sub, "sub_test.go", `gittest.Fixture(t)`)

	got, err := Count(dir)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if got != 18 {
		t.Errorf("Count = %d, want 18", got)
	}
}

func TestCount_SkipsArgumentsThatAreNotStringLiterals(t *testing.T) {
	dir := t.TempDir()
	writeSample(t, dir, "a_test.go", "use(name, 3, `git`)\nuse(name, 3)\ngittest.Other(t)")
	got, err := Count(dir)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if got != 1 {
		t.Errorf("Count = %d, want 1", got)
	}
}

func TestVerify_PropagatesCountErrorForMissingDir(t *testing.T) {
	okFile := filepath.Join(t.TempDir(), "ok.json")
	if err := os.WriteFile(okFile, []byte(`{"recorded": 5}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Verify(filepath.Join(t.TempDir(), "missing"), okFile); err == nil {
		t.Error("Verify on missing dir succeeded, want error")
	}
}

func TestCount_ErrorsOnMissingDirOrInvalidSyntax(t *testing.T) {
	if _, err := Count(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Error("Count on missing dir succeeded, want error")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bad_test.go"), []byte("package p\nfunc ("), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Count(dir); err == nil {
		t.Error("Count on unparsable test file succeeded, want error")
	}
}

func TestVerify_ChecksBoundsAndRejectsInvalidRecordedFiles(t *testing.T) {
	dir := t.TempDir()
	writeSample(t, dir, "a_test.go", "r.Git(\"status\")\nr.Git(\"log\")")

	okFile := filepath.Join(t.TempDir(), "ok.json")
	if err := os.WriteFile(okFile, []byte(`{"recorded": 2}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Verify(dir, okFile); err != nil {
		t.Errorf("Verify at exact recorded count failed: %v", err)
	}

	lowFile := filepath.Join(t.TempDir(), "low.json")
	if err := os.WriteFile(lowFile, []byte(`{"recorded": 1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Verify(dir, lowFile); err == nil {
		t.Error("Verify above recorded count succeeded, want error")
	}

	for name, body := range map[string]string{
		"invalid json": "{",
		"missing key":  `{"count": 5}`,
		"negative":     `{"recorded": -1}`,
	} {
		bad := filepath.Join(t.TempDir(), "bad.json")
		if err := os.WriteFile(bad, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := Verify(dir, bad); err == nil {
			t.Errorf("%s: Verify succeeded, want error", name)
		}
	}
	if err := Verify(dir, filepath.Join(t.TempDir(), "absent.json")); err == nil {
		t.Error("absent file: Verify succeeded, want error")
	}
}

func TestHelperCost_ChargesEveryGittestTableEntryAndIgnoresUnknowns(t *testing.T) {
	costs := gittest.StartCosts()
	var lines []string
	want := 0
	for key, cost := range costs {
		want += cost
		if method, ok := strings.CutPrefix(key, "Repo."); ok {
			lines = append(lines, "r."+method+"(t)")
		} else {
			lines = append(lines, "gittest."+key+"(t)")
		}
	}
	dir := t.TempDir()
	writeSample(t, dir, "all_test.go", strings.Join(lines, "\n"))
	got, err := Count(dir)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if got != want {
		t.Errorf("Count = %d, want table sum %d", got, want)
	}

	unknownDir := t.TempDir()
	writeSample(t, unknownDir, "unknown_test.go", "gittest.Unknown(t)\nr.Unknown(t)\nslices.Clone(x)")
	if got, err := Count(unknownDir); err != nil || got != 0 {
		t.Errorf("unknown calls Count = (%d, %v), want (0, nil)", got, err)
	}

	emptyDir := t.TempDir()
	if got, err := Count(emptyDir); err != nil || got != 0 {
		t.Errorf("empty dir Count = (%d, %v), want (0, nil)", got, err)
	}
}

func TestMissingCosts_DetectsUncostedGitStarters(t *testing.T) {
	dir := t.TempDir()
	prod := `package gittest

import (
	"os"
	"os/exec"
)

type Box[T any] struct{}

func (b Box[T]) Ignored() {}
func (r *Repo) Git(args ...string) {}
func (r *Repo) CommitAll() { r.Git("commit") }
func RawProbe() { exec.Command("git", "status") }
func Transitive() { unexportedGit() }
func unexportedGit() { exec.Command("git", "log") }
func Harmless() {
	exec.LookPath("git")
	os.RemoveAll("x")
	func() {}()
	FromTemplate()
	cyclicA()
}
func CallsCosted() {
	Fixture()
	Harmless()
}
func cyclicA() { cyclicB() }
func cyclicB() { cyclicA() }
func ForwardOnly()
`
	if err := os.WriteFile(filepath.Join(dir, "helpers.go"), []byte(prod), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "helpers_test.go"), []byte("package gittest\nimport \"os/exec\"\nfunc TestOnly() { exec.Command(\"git\", \"status\") }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := MissingCosts(dir)
	if err != nil {
		t.Fatalf("MissingCosts: %v", err)
	}
	want := []string{"CallsCosted", "RawProbe", "Repo.CommitAll", "Transitive"}
	if !slices.Equal(got, want) {
		t.Errorf("MissingCosts = %v, want %v", got, want)
	}
	if _, err := MissingCosts(filepath.Join(dir, "missing")); err == nil {
		t.Error("MissingCosts on missing dir succeeded, want error")
	}
	badDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(badDir, "bad.go"), []byte("package gittest\nfunc ("), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := MissingCosts(badDir); err == nil {
		t.Error("MissingCosts on invalid syntax succeeded, want error")
	}
}
