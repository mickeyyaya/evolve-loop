package naminguard

import (
	"encoding/json"
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
	writeTracked(t, dir, "README.md", "slug "+tok+"\n")
	writeTracked(t, dir, "sub/testdata/fixture.txt", "ignored "+tok+"\n") // excluded
	writeTracked(t, dir, "clean.md", "all good evolve-loop\n")            // canonical, not forbidden

	vs, err := Scan(dir, testManifest())
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 1 {
		t.Fatalf("want exactly 1 violation (testdata excluded, canonical ignored), got %d: %v", len(vs), vs)
	}
	if vs[0].File != "README.md" || vs[0].Token != tok {
		t.Errorf("unexpected violation: %+v", vs[0])
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
