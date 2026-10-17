package acspathscan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func protectsPersona(p string) bool { return p == "agents/evolve-auditor.md" }

func scan(t *testing.T, body string) ([]Need, error) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "p_test.go"), []byte("package p\n"+body), 0o644); err != nil {
		t.Fatal(err)
	}
	return Needs(dir, protectsPersona)
}

func TestNeeds_ReadsOnlyPathsThatATestFunctionReadsFromTheRepoRoot(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, body string
		want       int
	}{
		{"cycle 1933 shape: a joined path bound to a variable and given to acsassert", `
func TestA(t *testing.T) {
	persona := filepath.Join(acsassert.RepoRoot(t), "agents", "evolve-auditor.md")
	ok, _ := acsassert.LineContainsAllChecked(persona, "x")
	_ = ok
}`, 1},
		{"a root variable", `
func TestA(t *testing.T) {
	root := acsassert.RepoRoot(t)
	os.ReadFile(filepath.Join(root, "agents", "evolve-auditor.md"))
}`, 1},
		{"a bare RepoRoot call", `
func TestA(t *testing.T) { os.Stat(path.Join(RepoRoot(t), "agents", "evolve-auditor.md")) }`, 1},
		{"an all-literal join and a whole literal", `
func TestA(t *testing.T) {
	os.Open(filepath.Join("agents", "evolve-auditor.md"))
	os.Lstat("agents/evolve-auditor.md")
}`, 1},
		{"a var declaration", `
func TestA(t *testing.T) {
	var p = "agents/evolve-auditor.md"
	os.OpenFile(p, 0, 0)
}`, 1},
		{"the same path read twice counts once", `
func TestA(t *testing.T) {
	os.ReadFile("agents/evolve-auditor.md")
	os.ReadFile("agents/evolve-auditor.md")
}`, 1},
		{"a temp directory base", `
func TestA(t *testing.T) {
	tmp := t.TempDir()
	os.ReadFile(filepath.Join(tmp, "agents", "evolve-auditor.md"))
}`, 0},
		{"a call that is not the repo root", `
func TestA(t *testing.T) { os.ReadFile(filepath.Join(t.TempDir(), "agents", "evolve-auditor.md")) }`, 0},
		{"a variable that is never read", `
func TestA(t *testing.T) { p := "agents/evolve-auditor.md"; _ = p }`, 0},
		{"a blank assignment", `
func TestA(t *testing.T) { _ = filepath.Join("agents", "evolve-auditor.md") }`, 0},
		{"a dead helper that is not a test function", `
func helper() { os.ReadFile("agents/evolve-auditor.md") }`, 0},
		{"a struct tag and a package variable", "\nvar dead = \"agents/evolve-auditor.md\"\ntype S struct {\n\tF string `json:\"agents/evolve-auditor.md\"`\n}\nfunc TestA(t *testing.T) {}", 0},
		{"a message call", `
func TestA(t *testing.T) { t.Errorf("agents/evolve-auditor.md") }`, 0},
		{"prose", `
func TestA(t *testing.T) { os.ReadFile("see agents/evolve-auditor.md") }`, 0},
		{"a call of another package", `
func TestA(t *testing.T) { strings.Contains("agents/evolve-auditor.md", "x"); os.Getenv("agents/evolve-auditor.md"); acsassert.RepoRoot("agents/evolve-auditor.md") }`, 0},
		{"calls with no selector or a nested selector", `
func TestA(t *testing.T) { read("agents/evolve-auditor.md"); a.b.ReadFile("agents/evolve-auditor.md"); func() {}() }`, 0},
		{"mismatched assignment and a non-identifier target", `
func TestA(t *testing.T) {
	a, b := pair()
	t.x = "agents/evolve-auditor.md"
	os.ReadFile(a)
	os.ReadFile(b)
}`, 0},
		{"join shapes that are not a path", `
func TestA(t *testing.T) {
	os.ReadFile(filepath.Join("agents"))
	os.ReadFile(filepath.Join("agents", name))
	os.ReadFile(filepath.Join("agents", "evolve auditor.md"))
	os.ReadFile(filepath.Join("agents", 7))
	os.ReadFile(strings.ToLower("agents/evolve-auditor.md"))
	os.ReadFile(7)
	os.ReadFile(base + "agents/evolve-auditor.md")
	os.ReadFile(fn()("agents/evolve-auditor.md"))
	os.ReadFile(filepath.Join(fn()(), "agents", "evolve-auditor.md"))
}`, 0},
		{"a test function with no body", `
func TestA(t *testing.T)`, 0},
	}
	for _, tc := range cases {
		got, err := scan(t, tc.body)
		if err != nil {
			t.Errorf("%s: %v", tc.name, err)
		}
		if len(got) != tc.want {
			t.Errorf("%s: got %+v want %d", tc.name, got, tc.want)
		}
	}
}

func TestNeeds_NamesThePathTheTestFunctionAndTheFile(t *testing.T) {
	t.Parallel()
	got, err := scan(t, `
func TestC1933_004(t *testing.T) { os.ReadFile("agents/evolve-auditor.md") }`)
	if err != nil || len(got) != 1 || got[0] != (Need{Path: "agents/evolve-auditor.md", Test: "TestC1933_004", File: "p_test.go"}) {
		t.Errorf("got %+v err=%v", got, err)
	}
}

func TestNeeds_NoMembershipTestMeansNoNeeds(t *testing.T) {
	t.Parallel()
	if got, err := Needs(t.TempDir(), nil); got != nil || err != nil {
		t.Errorf("got %v %v", got, err)
	}
}

func TestNeeds_ReportsAListingFault(t *testing.T) {
	t.Parallel()
	if _, err := Needs(filepath.Join(t.TempDir(), "[bad"), protectsPersona); err == nil || !strings.Contains(err.Error(), "list") {
		t.Errorf("a bad pattern is a loud fault: %v", err)
	}
}

func TestNeeds_ReportsAParseFaultAndKeepsTheOtherFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a_bad.go", "package p\nfunc (")
	write("b_good.go", "package p\nfunc TestA(t *testing.T) { os.ReadFile(\"agents/evolve-auditor.md\") }\n")
	got, err := Needs(dir, protectsPersona)
	if err == nil || !strings.Contains(err.Error(), "a_bad.go") {
		t.Errorf("the unparseable file is named: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("the good file still counts: %+v", got)
	}
}

func TestSnapshot_CopiesTheGoFilesAndNothingElse(t *testing.T) {
	t.Parallel()
	src, dst := t.TempDir(), filepath.Join(t.TempDir(), "copy")
	for name, body := range map[string]string{"a_test.go": "package p\n", "notes.txt": "x"} {
		if err := os.WriteFile(filepath.Join(src, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := Snapshot(src, dst); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dst, "a_test.go")); err != nil {
		t.Error(err)
	}
	if _, err := os.Stat(filepath.Join(dst, "notes.txt")); err == nil {
		t.Error("only .go files are copied")
	}
}

func TestSnapshot_ReportsEachFault(t *testing.T) {
	t.Parallel()
	if err := Snapshot(filepath.Join(t.TempDir(), "[bad"), t.TempDir()); err == nil {
		t.Error("bad pattern")
	}
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Snapshot(t.TempDir(), filepath.Join(blocker, "sub")); err == nil {
		t.Error("a destination below a file")
	}
	src := t.TempDir()
	dir := filepath.Join(src, "dir.go")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Snapshot(src, t.TempDir()); err == nil {
		t.Error("an unreadable entry")
	}
	src2, dst2 := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(src2, "a.go"), []byte("package p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dst2, "a.go"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Snapshot(src2, dst2); err == nil {
		t.Error("a destination file that cannot be written")
	}
}
