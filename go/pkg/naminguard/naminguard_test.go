package naminguard

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Forbidden tokens are assembled at runtime so this committed test file never
// contains a literal forbidden token — otherwise the legacynames gate (and Scan
// itself) would flag the test that exercises them.
func deadSlug() string   { return "evolve" + "loop" }                   // the dead repo slug
func deadHandle() string { return "evolve-loop" + "@" + "evolve-loop" } // the dead plugin handle

func testManifest() *Manifest {
	return &Manifest{
		Forbidden: []Forbidden{
			{Token: deadSlug(), Replacement: "evolve-loop", Reason: "old slug"},
			{Token: deadHandle(), Replacement: "evo@evo", Reason: "dead handle"},
		},
		Exclude: []string{"*/testdata/*", "CHANGELOG.md"},
	}
}

func TestValidate(t *testing.T) {
	if err := testManifest().Validate(); err != nil {
		t.Fatalf("valid manifest rejected: %v", err)
	}
	if err := (&Manifest{}).Validate(); err == nil {
		t.Error("empty forbidden[] should fail validation")
	}
	nonConverging := &Manifest{Forbidden: []Forbidden{
		{Token: deadSlug(), Replacement: "x" + deadSlug() + "y"},
	}}
	if err := nonConverging.Validate(); err == nil {
		t.Error("replacement that still contains the token should fail validation")
	}
	emptyRepl := &Manifest{Forbidden: []Forbidden{{Token: deadSlug(), Replacement: ""}}}
	if err := emptyRepl.Validate(); err == nil {
		t.Error("empty replacement should fail validation")
	}
}

func TestLoad(t *testing.T) {
	b, err := json.Marshal(testManifest())
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "naming.json")
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got.Forbidden) != 2 {
		t.Fatalf("got %d forbidden, want 2", len(got.Forbidden))
	}
	if _, err := Load(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Error("Load of a missing manifest should error")
	}
}

func TestParseGrepOutput(t *testing.T) {
	tok := deadSlug()
	out := "README.md:12:see github.com/mickeyyaya/" + tok + "\n" +
		"docs/x.md:3:install " + tok + "\n"
	vs, err := parseGrepOutput(out, tok)
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 2 {
		t.Fatalf("want 2 violations, got %d", len(vs))
	}
	if vs[0].File != "README.md" || vs[0].Line != 12 || vs[0].Token != tok {
		t.Errorf("bad first violation: %+v", vs[0])
	}
	if _, err := parseGrepOutput("garbage-no-colons\n", tok); err == nil {
		t.Error("unparseable grep line should error")
	}
}

// --- integration: real git work tree ---------------------------------------

func initRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "t@example.com"},
		{"config", "user.name", "t"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

func writeTracked(t *testing.T, dir, rel, content string) {
	t.Helper()
	abs := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", rel)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add %s: %v\n%s", rel, err, out)
	}
}

func TestScan_FindsTrackedHonorsExclude(t *testing.T) {
	dir := initRepo(t)
	tok := deadSlug()
	writeTracked(t, dir, "README.md", "slug "+tok+"\nline 2 "+tok+"\n")
	writeTracked(t, dir, "other.md", "slug "+tok+"\n")
	writeTracked(t, dir, "sub/testdata/fixture.txt", "ignored "+tok+"\n") // excluded
	writeTracked(t, dir, "clean.md", "all good evolve-loop\n")            // canonical, not forbidden

	vs, err := Scan(dir, testManifest())
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 3 {
		t.Fatalf("want exactly 3 violations, got %d: %v", len(vs), vs)
	}
	if vs[0].File != "README.md" || vs[0].Line != 1 || vs[0].Token != tok {
		t.Errorf("unexpected first violation: %+v", vs[0])
	}
	if vs[1].File != "README.md" || vs[1].Line != 2 || vs[1].Token != tok {
		t.Errorf("unexpected second violation: %+v", vs[1])
	}
	if vs[2].File != "other.md" || vs[2].Line != 1 || vs[2].Token != tok {
		t.Errorf("unexpected third violation: %+v", vs[2])
	}
}

func TestFix_RewritesAndReportsChanged(t *testing.T) {
	dir := initRepo(t)
	tok := deadSlug()
	writeTracked(t, dir, "README.md", "slug "+tok+" here\n")
	writeTracked(t, dir, "sub/testdata/f.txt", tok+"\n") // excluded -> untouched

	m := testManifest()
	changed, err := Fix(dir, m)
	if err != nil {
		t.Fatal(err)
	}
	if len(changed) != 1 || changed[0] != "README.md" {
		t.Fatalf("changed = %v, want [README.md]", changed)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "README.md"))
	if strings.Contains(string(got), tok) {
		t.Errorf("token not replaced: %q", got)
	}
	if !strings.Contains(string(got), "evolve-loop") {
		t.Errorf("replacement missing: %q", got)
	}
	ex, _ := os.ReadFile(filepath.Join(dir, "sub/testdata/f.txt"))
	if !strings.Contains(string(ex), tok) {
		t.Errorf("excluded file was modified: %q", ex)
	}
	if vs, _ := Scan(dir, m); len(vs) != 0 {
		t.Errorf("post-fix scan should be clean, got %v", vs)
	}
}

func TestFix_PreservesExecutableMode(t *testing.T) {
	dir := initRepo(t)
	tok := deadSlug()
	rel := "scripts/run.sh"
	writeTracked(t, dir, rel, "#!/bin/sh\n# slug "+tok+"\n")
	abs := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.Chmod(abs, 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := Fix(dir, testManifest()); err != nil {
		t.Fatal(err)
	}

	got, _ := os.ReadFile(abs)
	if strings.Contains(string(got), tok) {
		t.Errorf("token not replaced: %q", got)
	}
	info, err := os.Stat(abs)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Errorf("Fix changed mode to %v, want 0755 — the executable bit must survive the atomic rewrite", info.Mode().Perm())
	}
}

// deadCmdNS assembles the boundary-anchored dead-command-namespace regex so the
// literal dead-command token never appears in this committed source.
func deadCmdNS() string { return "(^|[^A-Za-z0-9._/-])" + "/evolve-loop" + ":" }

func TestApplyRegex_BoundaryAnchored(t *testing.T) {
	f := Forbidden{Token: deadCmdNS(), Replacement: "$1/evo:", Match: "regex"}
	if err := (&Manifest{Forbidden: []Forbidden{f}}).Validate(); err != nil {
		t.Fatalf("regex manifest should validate: %v", err)
	}
	cmd := "run " + "/evolve-loop" + ":loop here"
	got, err := f.apply(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if want := "run /evo:loop here"; got != want {
		t.Errorf("apply(command) = %q, want %q", got, want)
	}
	// A repo-ref URL (owner/repo:branch) must be left alone (boundary char is alnum).
	url := "github.com/mickeyyaya/" + "evolve-loop" + ":main"
	if out, _ := f.apply(url); out != url {
		t.Errorf("apply rewrote a non-command ref: %q -> %q", url, out)
	}
}

func TestScan_RegexMatchesCommandNotURL(t *testing.T) {
	dir := initRepo(t)
	writeTracked(t, dir, "doc.md",
		"use "+"/evolve-loop"+":loop\nrepo github.com/mickeyyaya/"+"evolve-loop"+":main\n")
	m := &Manifest{Forbidden: []Forbidden{{Token: deadCmdNS(), Replacement: "$1/evo:", Match: "regex"}}}
	vs, err := Scan(dir, m)
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 1 {
		t.Fatalf("want 1 violation (the command line only), got %d: %v", len(vs), vs)
	}
	if vs[0].Line != 1 {
		t.Errorf("expected the command on line 1, got line %d", vs[0].Line)
	}
}

func TestViolation_String(t *testing.T) {
	v := Violation{File: "main.go", Line: 10, Token: "foo", Text: "contains foo"}
	got := v.String()
	want := "main.go:10: contains \"foo\" — contains foo"
	if got != want {
		t.Errorf("Violation.String() = %q, want %q", got, want)
	}
}

func TestValidate_InvalidForbidden(t *testing.T) {
	// empty token
	m1 := &Manifest{Forbidden: []Forbidden{{Token: "", Replacement: "bar"}}}
	if err := m1.Validate(); err == nil || !strings.Contains(err.Error(), "has empty token") {
		t.Errorf("expected empty token error, got %v", err)
	}
	// invalid match type
	m2 := &Manifest{Forbidden: []Forbidden{{Token: "foo", Replacement: "bar", Match: "invalid"}}}
	if err := m2.Validate(); err == nil || !strings.Contains(err.Error(), "want \"fixed\" or \"regex\"") {
		t.Errorf("expected invalid match type error, got %v", err)
	}
	// invalid regex
	m3 := &Manifest{Forbidden: []Forbidden{{Token: "[invalid", Replacement: "bar", Match: "regex"}}}
	if err := m3.Validate(); err == nil || !strings.Contains(err.Error(), "regex") {
		t.Errorf("expected invalid regex error, got %v", err)
	}
}

func TestParseGrepOutput_Errors(t *testing.T) {
	// line number missing second colon
	_, err := parseGrepOutput("file:123", "tok")
	if err == nil || !strings.Contains(err.Error(), "unparseable grep line") {
		t.Errorf("expected unparseable error, got %v", err)
	}
	// line number not an integer
	_, err = parseGrepOutput("file:notanint:text", "tok")
	if err == nil || !strings.Contains(err.Error(), "bad line number") {
		t.Errorf("expected bad line number error, got %v", err)
	}
}

func TestWriteFilePreservingMode_Errors(t *testing.T) {
	// Non-existent file (Stat error)
	err := writeFilePreservingMode("/nonexistent/file/path", []byte("data"))
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}

	// Sibling directory that exists but is read-only (CreateTemp error)
	dir := t.TempDir()
	subDir := filepath.Join(dir, "readonly")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(subDir, "test.txt")
	if err := os.WriteFile(file, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	// Make directory read-only (read/execute but no write)
	if err := os.Chmod(subDir, 0555); err != nil {
		t.Fatal(err)
	}
	defer func() {
		// Restore permissions to clean up
		_ = os.Chmod(subDir, 0755)
	}()

	err = writeFilePreservingMode(file, []byte("new"))
	if err == nil {
		t.Fatal("expected error when directory is read-only")
	}
}

func TestScanAndFix_MockGrepErrors(t *testing.T) {
	orig := gitGrep
	defer func() { gitGrep = orig }()

	m := testManifest()

	// Test gitGrep returning error
	gitGrep = func(root string, args ...string) (string, int, error) {
		return "", -1, fmt.Errorf("simulated grep error")
	}
	if _, err := Scan("root", m); err == nil {
		t.Fatal("expected Scan error")
	}
	if _, err := Fix("root", m); err == nil {
		t.Fatal("expected Fix error")
	}

	// Test gitGrep returning bad exit code (e.g. 128)
	gitGrep = func(root string, args ...string) (string, int, error) {
		return "some stdout", 128, nil
	}
	if _, err := Scan("root", m); err == nil {
		t.Fatal("expected Scan error for exit 128")
	}
	if _, err := Fix("root", m); err == nil {
		t.Fatal("expected Fix error for exit 128")
	}

	// Test gitGrep returning a file that doesn't exist to trigger os.ReadFile error in Fix
	gitGrep = func(root string, args ...string) (string, int, error) {
		if strings.Contains(strings.Join(args, " "), "-l") {
			return "does-not-exist.txt\n", 0, nil
		}
		return "", 1, nil
	}
	if _, err := Fix("root", m); err == nil {
		t.Fatal("expected Fix error for missing file")
	}

	// Test parseGrepOutput returning error inside Scan
	gitGrep = func(root string, args ...string) (string, int, error) {
		return "garbage-no-colons\n", 0, nil
	}
	if _, err := Scan("root", m); err == nil {
		t.Fatal("expected Scan error when parseGrepOutput fails")
	}

	// Test f.apply returning error inside Fix (invalid regex pattern)
	invalidRegexpManifest := &Manifest{
		Forbidden: []Forbidden{
			{Token: "[invalid", Replacement: "bar", Match: "regex"},
		},
	}
	tmpFile := filepath.Join(t.TempDir(), "test.txt")
	if err := os.WriteFile(tmpFile, []byte("some content"), 0644); err != nil {
		t.Fatal(err)
	}
	gitGrep = func(root string, args ...string) (string, int, error) {
		if strings.Contains(strings.Join(args, " "), "-l") {
			return filepath.Base(tmpFile) + "\n", 0, nil
		}
		return "", 1, nil
	}
	if _, err := Fix(filepath.Dir(tmpFile), invalidRegexpManifest); err == nil {
		t.Fatal("expected Fix error when regex compile fails")
	}

	// Test nb == string(b) continue path
	noopManifest := &Manifest{
		Forbidden: []Forbidden{
			{Token: "non-existent-token", Replacement: "bar"},
		},
	}
	changedList, err := Fix(filepath.Dir(tmpFile), noopManifest)
	if err != nil {
		t.Fatalf("unexpected Fix error: %v", err)
	}
	if len(changedList) != 0 {
		t.Errorf("expected no files to be changed, got %v", changedList)
	}

	// Test Fix failing during writeFilePreservingMode (write error) and empty relation check
	mWriteFail := &Manifest{
		Forbidden: []Forbidden{
			{Token: "content", Replacement: "bar"},
		},
	}
	writeFailDir := filepath.Join(t.TempDir(), "faildir")
	if err := os.Mkdir(writeFailDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeFailFile := filepath.Join(writeFailDir, "test.txt")
	if err := os.WriteFile(writeFailFile, []byte("some content"), 0644); err != nil {
		t.Fatal(err)
	}
	gitGrep = func(root string, args ...string) (string, int, error) {
		if strings.Contains(strings.Join(args, " "), "-l") {
			// trailing empty line after split covers rel == "" continue path!
			return filepath.Base(writeFailFile) + "\n\n", 0, nil
		}
		return "", 1, nil
	}
	// Make directory read-only so CreateTemp inside writeFilePreservingMode fails
	if err := os.Chmod(writeFailDir, 0555); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chmod(writeFailDir, 0755)
	}()
	if _, err := Fix(writeFailDir, mWriteFail); err == nil {
		t.Fatal("expected Fix error when writeFilePreservingMode fails")
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	p := filepath.Join(t.TempDir(), "invalid.json")
	if err := os.WriteFile(p, []byte("invalid json"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(p); err == nil {
		t.Fatal("expected error loading invalid JSON")
	}
}

func TestLoad_InvalidManifest(t *testing.T) {
	p := filepath.Join(t.TempDir(), "invalid_manifest.json")
	// JSON is valid, but forbidden list is empty
	if err := os.WriteFile(p, []byte(`{"forbidden": []}`), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(p); err == nil {
		t.Fatal("expected error loading invalid manifest")
	}
}

func TestGitGrep_NotFound(t *testing.T) {
	t.Setenv("PATH", "")
	_, _, err := gitGrep(t.TempDir(), "args...")
	if err == nil {
		t.Fatal("expected error when git is not on PATH")
	}
}
