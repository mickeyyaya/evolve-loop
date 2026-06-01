package releaseconsistency

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// makeRepo creates a fixture repo with all 4 markers at the given version.
func makeRepo(t *testing.T, version string) string {
	t.Helper()
	d := t.TempDir()
	mm := majorMinor(version)
	must := func(rel, body string) {
		path := filepath.Join(d, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	must(".claude-plugin/plugin.json", fmt.Sprintf(`{"name":"evolve-loop","version":"%s"}`, version))
	must(".claude-plugin/marketplace.json", fmt.Sprintf(`{"plugins":[{"name":"evolve-loop","version":"%s"}]}`, version))
	must("skills/evolve-loop/SKILL.md", fmt.Sprintf("---\nname: x\n---\n\n# Evolve Loop v%s\n\nbody\n", mm))
	must("README.md", fmt.Sprintf("# Evolve Loop\n\n**Current (v%s)** description\n\n| v%s | 2026 |\n", mm, mm))
	must("CHANGELOG.md", fmt.Sprintf("# Changelog\n\n## [%s] - 2026-05-24\n\nEntries.\n", version))
	return d
}

// === Happy path: all markers consistent ====================================
func TestCheck_HappyPath(t *testing.T) {
	d := makeRepo(t, "11.8.2")
	var buf bytes.Buffer
	res, err := Run(Options{ProjectRoot: d, Target: "11.8.2", Stderr: &buf})
	if err != nil {
		t.Fatalf("err = %v\nlog=%s", err, buf.String())
	}
	if res.Errors != 0 {
		t.Errorf("Errors = %d, want 0", res.Errors)
	}
	for _, c := range res.Checks {
		if c.Status != "OK" {
			t.Errorf("check %s: status=%s, want OK", c.File, c.Status)
		}
	}
}

// === plugin.json missing → MISSING + ErrInconsistent =======================
func TestCheck_MissingPluginJSON(t *testing.T) {
	d := t.TempDir()
	_, err := Run(Options{ProjectRoot: d, Target: "1.0.0"})
	if !errors.Is(err, ErrInconsistent) {
		t.Fatalf("err = %v, want ErrInconsistent", err)
	}
}

// === Marker mismatch (plugin.json says X, target says Y) → MISMATCH =======
func TestCheck_PluginJSONMismatch(t *testing.T) {
	d := makeRepo(t, "11.8.1")
	var buf bytes.Buffer
	_, err := Run(Options{ProjectRoot: d, Target: "11.8.2", Stderr: &buf})
	if !errors.Is(err, ErrInconsistent) {
		t.Fatalf("err = %v, want ErrInconsistent", err)
	}
	if !strings.Contains(buf.String(), "MISMATCH") {
		t.Errorf("log missing MISMATCH: %s", buf.String())
	}
}

// === No CHANGELOG entry → MISSING ==========================================
func TestCheck_NoChangelogEntry(t *testing.T) {
	d := makeRepo(t, "11.8.2")
	// Strip the entry.
	if err := os.WriteFile(filepath.Join(d, "CHANGELOG.md"), []byte("# Changelog\n\nno entries\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := Run(Options{ProjectRoot: d, Target: "11.8.2"})
	if !errors.Is(err, ErrInconsistent) {
		t.Fatalf("err = %v, want ErrInconsistent", err)
	}
}

// === SKILL.md heading at wrong version → MISMATCH ==========================
func TestCheck_SkillHeadingMismatch(t *testing.T) {
	d := makeRepo(t, "11.8.2")
	if err := os.WriteFile(filepath.Join(d, "skills/evolve-loop/SKILL.md"),
		[]byte("# Evolve Loop v10.0\n\nstale\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := Run(Options{ProjectRoot: d, Target: "11.8.2"})
	if !errors.Is(err, ErrInconsistent) {
		t.Fatalf("err = %v, want ErrInconsistent", err)
	}
}

// === No target arg → derives from plugin.json ==============================
func TestCheck_DerivesTargetFromPluginJSON(t *testing.T) {
	d := makeRepo(t, "11.8.2")
	var buf bytes.Buffer
	res, err := Run(Options{ProjectRoot: d, Stderr: &buf})
	if err != nil {
		t.Fatalf("err = %v\nlog=%s", err, buf.String())
	}
	if res.Target != "11.8.2" {
		t.Errorf("Target = %q, want '11.8.2'", res.Target)
	}
}

// === README current cell mismatch → MISMATCH ===============================
func TestCheck_ReadmeCurrentMismatch(t *testing.T) {
	d := makeRepo(t, "11.8.2")
	if err := os.WriteFile(filepath.Join(d, "README.md"),
		[]byte("# Evolve Loop\n\n**Current (v9.0)** stale\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := Run(Options{ProjectRoot: d, Target: "11.8.2"})
	if !errors.Is(err, ErrInconsistent) {
		t.Fatalf("err = %v, want ErrInconsistent", err)
	}
}

// === Empty ProjectRoot → error =============================================
func TestCheck_EmptyProjectRoot(t *testing.T) {
	_, err := Run(Options{Target: "1.0.0"})
	if err == nil {
		t.Error("want err for empty ProjectRoot")
	}
}

// === extractJSONVersion handles malformed gracefully =======================
func TestExtractJSONVersion(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "v.json")
	if err := os.WriteFile(p, []byte(`{"version":"1.2.3"}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	v, err := extractJSONVersion(p)
	if err != nil || v != "1.2.3" {
		t.Errorf("extractJSONVersion = (%q, %v), want (1.2.3, nil)", v, err)
	}
	bad := filepath.Join(d, "no-version.json")
	if err := os.WriteFile(bad, []byte(`{"name":"foo"}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := extractJSONVersion(bad); err == nil {
		t.Error("want err for no version field")
	}
}

// === A single missing marker file is reported MISSING + logged ============
// Removing marketplace.json (but keeping plugin.json so Run reaches the checks)
// drives checkJSONVersion's os.Stat-err → MISSING branch and the MISSING log line.
func TestCheck_MarkerFileMissing(t *testing.T) {
	d := makeRepo(t, "11.8.2")
	if err := os.Remove(filepath.Join(d, ".claude-plugin/marketplace.json")); err != nil {
		t.Fatalf("rm: %v", err)
	}
	var buf bytes.Buffer
	res, err := Run(Options{ProjectRoot: d, Target: "11.8.2", Stderr: &buf})
	if !errors.Is(err, ErrInconsistent) {
		t.Fatalf("err = %v, want ErrInconsistent", err)
	}
	if res.Errors != 1 {
		t.Errorf("Errors = %d, want 1", res.Errors)
	}
	var mk Check
	for _, c := range res.Checks {
		if c.File == ".claude-plugin/marketplace.json" {
			mk = c
		}
	}
	if mk.Status != "MISSING" {
		t.Errorf("marketplace.json status = %q, want MISSING", mk.Status)
	}
	if !strings.Contains(buf.String(), "MISSING  .claude-plugin/marketplace.json") {
		t.Errorf("log missing MISSING line: %s", buf.String())
	}
}

// === A JSON marker present but with no "version" field → NO_MATCH ==========
// Drives checkJSONVersion's extractJSONVersion-err → NO_MATCH branch and the
// NO MATCH log line in Run's switch (covers Run's case "NO_MATCH").
func TestCheck_MarkerNoVersionField(t *testing.T) {
	d := makeRepo(t, "11.8.2")
	// marketplace.json present but lacks a version field.
	if err := os.WriteFile(filepath.Join(d, ".claude-plugin/marketplace.json"),
		[]byte(`{"plugins":[{"name":"evolve-loop"}]}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	var buf bytes.Buffer
	res, err := Run(Options{ProjectRoot: d, Target: "11.8.2", Stderr: &buf})
	if !errors.Is(err, ErrInconsistent) {
		t.Fatalf("err = %v, want ErrInconsistent", err)
	}
	var mk Check
	for _, c := range res.Checks {
		if c.File == ".claude-plugin/marketplace.json" {
			mk = c
		}
	}
	if mk.Status != "NO_MATCH" {
		t.Errorf("marketplace.json status = %q, want NO_MATCH", mk.Status)
	}
	if !strings.Contains(buf.String(), "NO MATCH .claude-plugin/marketplace.json") {
		t.Errorf("log missing NO MATCH line: %s", buf.String())
	}
}

// === SKILL.md absent → MISSING ============================================
// Drives checkSkillHeading's os.ReadFile-err → MISSING branch.
func TestCheck_SkillFileMissing(t *testing.T) {
	d := makeRepo(t, "11.8.2")
	if err := os.Remove(filepath.Join(d, "skills/evolve-loop/SKILL.md")); err != nil {
		t.Fatalf("rm: %v", err)
	}
	res, err := Run(Options{ProjectRoot: d, Target: "11.8.2"})
	if !errors.Is(err, ErrInconsistent) {
		t.Fatalf("err = %v, want ErrInconsistent", err)
	}
	if statusOf(res, "skills/evolve-loop/SKILL.md") != "MISSING" {
		t.Errorf("SKILL.md status = %q, want MISSING", statusOf(res, "skills/evolve-loop/SKILL.md"))
	}
}

// === SKILL.md present but no matching heading → NO_MATCH ===================
// Drives checkSkillHeading's loop-exhausted → NO_MATCH branch.
func TestCheck_SkillHeadingAbsent(t *testing.T) {
	d := makeRepo(t, "11.8.2")
	if err := os.WriteFile(filepath.Join(d, "skills/evolve-loop/SKILL.md"),
		[]byte("---\nname: x\n---\n\n# Some Other Title\n\nbody\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	res, err := Run(Options{ProjectRoot: d, Target: "11.8.2"})
	if !errors.Is(err, ErrInconsistent) {
		t.Fatalf("err = %v, want ErrInconsistent", err)
	}
	if statusOf(res, "skills/evolve-loop/SKILL.md") != "NO_MATCH" {
		t.Errorf("SKILL.md status = %q, want NO_MATCH", statusOf(res, "skills/evolve-loop/SKILL.md"))
	}
}

// === README.md absent → MISSING for the current-version check =============
// Drives checkReadmeCurrent's os.ReadFile-err → MISSING branch.
func TestCheck_ReadmeFileMissing(t *testing.T) {
	d := makeRepo(t, "11.8.2")
	if err := os.Remove(filepath.Join(d, "README.md")); err != nil {
		t.Fatalf("rm: %v", err)
	}
	res, err := Run(Options{ProjectRoot: d, Target: "11.8.2"})
	if !errors.Is(err, ErrInconsistent) {
		t.Fatalf("err = %v, want ErrInconsistent", err)
	}
	// Both README checks (current-version + history row) are MISSING.
	if statusOf(res, "README.md") != "MISSING" {
		t.Errorf("README.md current-version status = %q, want MISSING", statusOf(res, "README.md"))
	}
}

// === README.md present but no "Current (vX.Y)" cell → NO_MATCH ============
// Drives checkReadmeCurrent's regex-no-match → NO_MATCH branch. The history
// "v11.8" row is kept so only the current-version check trips.
func TestCheck_ReadmeCurrentCellAbsent(t *testing.T) {
	d := makeRepo(t, "11.8.2")
	if err := os.WriteFile(filepath.Join(d, "README.md"),
		[]byte("# Evolve Loop\n\nno current cell here\n\n| v11.8 | 2026 |\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	res, err := Run(Options{ProjectRoot: d, Target: "11.8.2"})
	if !errors.Is(err, ErrInconsistent) {
		t.Fatalf("err = %v, want ErrInconsistent", err)
	}
	// First README check (current version) is NO_MATCH; history row still OK.
	var current Check
	for _, c := range res.Checks {
		if c.File == "README.md" {
			current = c
			break // first README entry is the current-version check
		}
	}
	if current.Status != "NO_MATCH" {
		t.Errorf("README current-version status = %q, want NO_MATCH", current.Status)
	}
}

// === CHANGELOG.md absent → MISSING ========================================
// Drives checkContains's os.ReadFile-err → MISSING branch (distinct from the
// existing TestCheck_NoChangelogEntry which exercises file-present-no-match).
func TestCheck_ChangelogFileMissing(t *testing.T) {
	d := makeRepo(t, "11.8.2")
	if err := os.Remove(filepath.Join(d, "CHANGELOG.md")); err != nil {
		t.Fatalf("rm: %v", err)
	}
	res, err := Run(Options{ProjectRoot: d, Target: "11.8.2"})
	if !errors.Is(err, ErrInconsistent) {
		t.Fatalf("err = %v, want ErrInconsistent", err)
	}
	if statusOf(res, "CHANGELOG.md") != "MISSING" {
		t.Errorf("CHANGELOG.md status = %q, want MISSING", statusOf(res, "CHANGELOG.md"))
	}
}

// statusOf returns the status of the first check matching file.
func statusOf(res Result, file string) string {
	for _, c := range res.Checks {
		if c.File == file {
			return c.Status
		}
	}
	return "<absent>"
}

// === majorMinor table ======================================================
func TestMajorMinor(t *testing.T) {
	cases := []struct{ in, want string }{
		{"1.2.3", "1.2"},
		{"11.8.2", "11.8"},
		{"1.2", "1.2"},
		{"1", "1"},
	}
	for _, tc := range cases {
		if got := majorMinor(tc.in); got != tc.want {
			t.Errorf("majorMinor(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
