package versionbump

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/semvercheck"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestIsSemver(t *testing.T) {
	for _, ok := range []string{"1.0.0", "11.7.1", "0.0.1"} {
		if !semvercheck.IsSemver(ok) {
			t.Errorf("expected %q valid", ok)
		}
	}
	for _, bad := range []string{"v1.0.0", "1.0", "1.0.0-beta", "abc"} {
		if semvercheck.IsSemver(bad) {
			t.Errorf("expected %q invalid", bad)
		}
	}
}

func TestMajorMinor(t *testing.T) {
	tests := map[string]string{
		"1.0.0":  "1.0",
		"11.7.1": "11.7",
		"0.0.0":  "0.0",
		"weird":  "weird",
	}
	for in, want := range tests {
		if got := MajorMinor(in); got != want {
			t.Errorf("MajorMinor(%q)=%q, want %q", in, got, want)
		}
	}
}

func TestDefaultPaths(t *testing.T) {
	p := DefaultPaths("/repo")
	if p.PluginJSON != "/repo/.claude-plugin/plugin.json" {
		t.Errorf("PluginJSON=%q", p.PluginJSON)
	}
	if p.MarketplaceJSON != "/repo/.claude-plugin/marketplace.json" {
		t.Errorf("MarketplaceJSON=%q", p.MarketplaceJSON)
	}
	if p.CodexPluginJSON != "/repo/.codex-plugin/plugin.json" {
		t.Errorf("CodexPluginJSON=%q", p.CodexPluginJSON)
	}
	if p.SkillMD != "/repo/skills/loop/SKILL.md" {
		t.Errorf("SkillMD=%q", p.SkillMD)
	}
	if p.ReadmeMD != "/repo/README.md" {
		t.Errorf("ReadmeMD=%q", p.ReadmeMD)
	}
}

func TestCurrentJSONVersion(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "plugin.json")
	writeFile(t, p, `{"name":"evo","version":"11.6.6","extra":"x"}`)
	got, err := CurrentJSONVersion(p)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if got != "11.6.6" {
		t.Errorf("got %q", got)
	}
}

func TestCurrentJSONVersion_NoField(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "x.json")
	writeFile(t, p, `{"name":"x"}`)
	got, err := CurrentJSONVersion(p)
	if err != nil || got != "" {
		t.Errorf("got %q err %v", got, err)
	}
}

func TestCurrentJSONVersion_MissingFile(t *testing.T) {
	_, err := CurrentJSONVersion(filepath.Join(t.TempDir(), "missing"))
	if err == nil {
		t.Errorf("expected error")
	}
}

func TestBumpJSONVersion_TopLevel(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "plugin.json")
	writeFile(t, p, `{"name":"evo","version":"11.6.6"}`)
	changed, err := BumpJSONVersion(p, "11.7.0", false)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if !changed {
		t.Errorf("expected change")
	}
	body, _ := os.ReadFile(p)
	if !strings.Contains(string(body), `"version": "11.7.0"`) {
		t.Errorf("missing new version: %s", body)
	}
}

func TestBumpJSONVersion_IdempotentSkip(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "plugin.json")
	writeFile(t, p, `{"version":"11.7.0"}`)
	before, _ := os.ReadFile(p)
	changed, err := BumpJSONVersion(p, "11.7.0", false)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if changed {
		t.Errorf("expected no change")
	}
	after, _ := os.ReadFile(p)
	if string(before) != string(after) {
		t.Errorf("file mutated despite idempotent skip")
	}
}

func TestBumpJSONVersion_DryRunReportsButDoesNotWrite(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "plugin.json")
	writeFile(t, p, `{"version":"11.6.6"}`)
	changed, err := BumpJSONVersion(p, "11.7.0", true)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if !changed {
		t.Errorf("dry-run should report would-change")
	}
	body, _ := os.ReadFile(p)
	if !strings.Contains(string(body), "11.6.6") {
		t.Errorf("dry-run mutated file: %s", body)
	}
}

func TestBumpJSONVersion_MissingFile(t *testing.T) {
	_, err := BumpJSONVersion(filepath.Join(t.TempDir(), "missing.json"), "1.0.0", false)
	if err == nil {
		t.Errorf("expected error")
	}
}

func TestBumpJSONVersion_MarketplaceShape(t *testing.T) {
	// marketplace.json has plugins[].version + a top-level version stub.
	tmp := t.TempDir()
	p := filepath.Join(tmp, "marketplace.json")
	writeFile(t, p, `{"plugins":[{"name":"evo","version":"11.6.6"},{"name":"other","version":"11.6.6"}]}`)
	changed, err := BumpJSONVersion(p, "11.7.0", false)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if !changed {
		t.Errorf("expected change")
	}
	body, _ := os.ReadFile(p)
	// Both plugin versions bumped.
	if strings.Count(string(body), `"version": "11.7.0"`) != 2 {
		t.Errorf("expected 2 occurrences of new version, got: %s", body)
	}
}

func TestCurrentSkillHeading(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "SKILL.md")
	writeFile(t, p, "# Evolve Loop v11.6.6\n\nbody")
	got, _ := CurrentSkillHeading(p)
	if got != "11.6" {
		t.Errorf("got %q", got)
	}
}

func TestBumpSkillHeading_Changes(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "SKILL.md")
	writeFile(t, p, "# Evolve Loop v11.6.6\n\nintro\n# Evolve Loop v11.6.6 should not match\n")
	changed, err := BumpSkillHeading(p, "11.7", false)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if !changed {
		t.Errorf("expected change")
	}
	body, _ := os.ReadFile(p)
	if !strings.Contains(string(body), "# Evolve Loop v11.7") {
		t.Errorf("missing new heading: %s", body)
	}
}

func TestBumpSkillHeading_Idempotent(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "SKILL.md")
	writeFile(t, p, "# Evolve Loop v11.7\nbody")
	changed, _ := BumpSkillHeading(p, "11.7", false)
	if changed {
		t.Errorf("idempotent skip expected")
	}
}

func TestBumpSkillHeading_MissingFileSilent(t *testing.T) {
	changed, err := BumpSkillHeading(filepath.Join(t.TempDir(), "x.md"), "11.7", false)
	if err != nil || changed {
		t.Errorf("missing file should silently no-op, got %v %v", changed, err)
	}
}

func TestBumpSkillHeading_NoMatchingHeading(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "SKILL.md")
	writeFile(t, p, "no heading here\n")
	changed, _ := BumpSkillHeading(p, "11.7", false)
	if changed {
		t.Errorf("no heading should silently no-op")
	}
}

func TestBumpReadmeCurrent_Changes(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "README.md")
	writeFile(t, p, "Current (v11.6) — latest stable\n")
	changed, err := BumpReadmeCurrent(p, "11.7", false)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if !changed {
		t.Errorf("expected change")
	}
	body, _ := os.ReadFile(p)
	if !strings.Contains(string(body), "Current (v11.7)") {
		t.Errorf("missing new version: %s", body)
	}
}

func TestBumpReadmeCurrent_Idempotent(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "README.md")
	writeFile(t, p, "Current (v11.7)\n")
	changed, _ := BumpReadmeCurrent(p, "11.7", false)
	if changed {
		t.Errorf("idempotent skip expected")
	}
}

func TestBumpReadmeCurrent_MissingFileSilent(t *testing.T) {
	changed, _ := BumpReadmeCurrent(filepath.Join(t.TempDir(), "x.md"), "11.7", false)
	if changed {
		t.Errorf("missing file should silently no-op")
	}
}

func TestHasHistoryRow(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "README.md")
	writeFile(t, p, "| v11.6 | May 23 | shipped |\n| v11.7 | TBD | TBD |\n")
	yes, _ := HasHistoryRow(p, "11.6")
	if !yes {
		t.Errorf("expected 11.6 row")
	}
	yes, _ = HasHistoryRow(p, "11.7")
	if !yes {
		t.Errorf("expected 11.7 row")
	}
	yes, _ = HasHistoryRow(p, "12.0")
	if yes {
		t.Errorf("12.0 should be absent")
	}
}

func TestBumpReadmeHistory_AppendsRow(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "README.md")
	writeFile(t, p, "## Version history\n\n| Version | Date | Theme |\n|---|---|---|\n| v11.5 | May 23 | first |\n| v11.6 | May 23 | second |\n\n## Next section\n")
	now := time.Date(2026, 5, 24, 0, 0, 0, 0, time.UTC)
	changed, err := BumpReadmeHistory(p, "11.7", now, false)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if !changed {
		t.Errorf("expected insert")
	}
	body, _ := os.ReadFile(p)
	if !strings.Contains(string(body), "| v11.7 | May 24 | TBD") {
		t.Errorf("new row not inserted: %s", body)
	}
	// Order: 11.5 then 11.6 then 11.7 then "Next section".
	idx5 := strings.Index(string(body), "v11.5")
	idx6 := strings.Index(string(body), "v11.6")
	idx7 := strings.Index(string(body), "v11.7")
	idxNext := strings.Index(string(body), "Next section")
	if !(idx5 < idx6 && idx6 < idx7 && idx7 < idxNext) {
		t.Errorf("row order wrong: 5=%d 6=%d 7=%d next=%d", idx5, idx6, idx7, idxNext)
	}
}

func TestBumpReadmeHistory_IdempotentWhenRowExists(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "README.md")
	writeFile(t, p, "| v11.7 | already there | done |\n")
	now := time.Date(2026, 5, 24, 0, 0, 0, 0, time.UTC)
	changed, _ := BumpReadmeHistory(p, "11.7", now, false)
	if changed {
		t.Errorf("expected idempotent skip")
	}
}

func TestBumpReadmeHistory_NoTableNoOp(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "README.md")
	writeFile(t, p, "README without history table\n")
	now := time.Date(2026, 5, 24, 0, 0, 0, 0, time.UTC)
	changed, _ := BumpReadmeHistory(p, "11.7", now, false)
	if changed {
		t.Errorf("no table → no insert")
	}
}

func TestBumpReadmeHistory_DryRunDoesNotWrite(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "README.md")
	original := "| v11.6 | May 23 | s |\n"
	writeFile(t, p, original)
	changed, _ := BumpReadmeHistory(p, "11.7", time.Now(), true)
	if !changed {
		t.Errorf("dry-run should report would-change")
	}
	body, _ := os.ReadFile(p)
	if string(body) != original {
		t.Errorf("dry-run mutated file: %s", body)
	}
}

func TestFormatHistoryDate(t *testing.T) {
	tests := map[string]time.Time{
		"May 24": time.Date(2026, 5, 24, 0, 0, 0, 0, time.UTC),
		"May 4":  time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC),
		"Jan 1":  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	for want, when := range tests {
		if got := formatHistoryDate(when); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
}

func TestRun_FullBumpPipeline(t *testing.T) {
	tmp := t.TempDir()
	paths := Paths{
		PluginJSON:      filepath.Join(tmp, "plugin.json"),
		MarketplaceJSON: filepath.Join(tmp, "marketplace.json"),
		SkillMD:         filepath.Join(tmp, "SKILL.md"),
		ReadmeMD:        filepath.Join(tmp, "README.md"),
	}
	writeFile(t, paths.PluginJSON, `{"name":"x","version":"11.6.6"}`)
	writeFile(t, paths.MarketplaceJSON, `{"plugins":[{"name":"x","version":"11.6.6"}]}`)
	writeFile(t, paths.SkillMD, "# Evolve Loop v11.6\n")
	writeFile(t, paths.ReadmeMD, "Current (v11.6)\n\n| v11.6 | May 23 | shipped |\n")
	now := time.Date(2026, 5, 24, 0, 0, 0, 0, time.UTC)
	res, err := Run(paths, "11.7.0", false, now)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if res.Target != "11.7.0" {
		t.Errorf("target=%q", res.Target)
	}
	if len(res.Modified) != 5 {
		t.Errorf("expected 5 modified, got %d: %v", len(res.Modified), res.Modified)
	}
	// All files reflect 11.7.0 now.
	body, _ := os.ReadFile(paths.PluginJSON)
	if !strings.Contains(string(body), `"version": "11.7.0"`) {
		t.Errorf("plugin.json not bumped")
	}
	body, _ = os.ReadFile(paths.MarketplaceJSON)
	if !strings.Contains(string(body), `"version": "11.7.0"`) {
		t.Errorf("marketplace.json not bumped")
	}
	body, _ = os.ReadFile(paths.SkillMD)
	if !strings.Contains(string(body), "# Evolve Loop v11.7") {
		t.Errorf("SKILL.md not bumped")
	}
	body, _ = os.ReadFile(paths.ReadmeMD)
	if !strings.Contains(string(body), "Current (v11.7)") {
		t.Errorf("README Current not bumped")
	}
	if !strings.Contains(string(body), "| v11.7 |") {
		t.Errorf("README history not appended")
	}
}

// TestRun_BumpsCodexPluginJSON pins the D3 fix: the generated Codex mirror is
// version-bumped in lockstep with the Claude manifest, so a release never leaves
// the Codex install surface stale.
func TestRun_BumpsCodexPluginJSON(t *testing.T) {
	tmp := t.TempDir()
	paths := Paths{
		PluginJSON:      filepath.Join(tmp, "plugin.json"),
		MarketplaceJSON: filepath.Join(tmp, "marketplace.json"),
		CodexPluginJSON: filepath.Join(tmp, ".codex-plugin", "plugin.json"),
		SkillMD:         filepath.Join(tmp, "SKILL.md"),
		ReadmeMD:        filepath.Join(tmp, "README.md"),
	}
	writeFile(t, paths.PluginJSON, `{"name":"evo","version":"21.4.1"}`)
	writeFile(t, paths.MarketplaceJSON, `{"plugins":[{"name":"evo","version":"21.4.1"}]}`)
	writeFile(t, paths.CodexPluginJSON, `{"name":"evo","version":"21.4.1","skills":"./skills/"}`)
	writeFile(t, paths.SkillMD, "# Evolve Loop v21.4\n")
	writeFile(t, paths.ReadmeMD, "Current (v21.4)\n\n| v21.4 | Jun 29 | shipped |\n")
	res, err := Run(paths, "21.4.2", false, time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("%v", err)
	}
	body, _ := os.ReadFile(paths.CodexPluginJSON)
	if !strings.Contains(string(body), `"version": "21.4.2"`) {
		t.Errorf(".codex-plugin/plugin.json not bumped: %s", body)
	}
	// surgical replace preserves the rest (drift gate stays green post-release)
	if !strings.Contains(string(body), `"skills":"./skills/"`) {
		t.Errorf("surgical bump altered non-version content: %s", body)
	}
	var sawCodex bool
	for _, m := range res.Modified {
		if m == ".codex-plugin/plugin.json" {
			sawCodex = true
		}
	}
	if !sawCodex {
		t.Errorf("Modified should list .codex-plugin/plugin.json, got %v", res.Modified)
	}
}

// TestRun_CodexPluginJSON_ToleratedAbsent — a checkout without the generated
// Codex mirror must not fail the bump (the file is generated, not hand-tracked
// in every working tree).
func TestRun_CodexPluginJSON_ToleratedAbsent(t *testing.T) {
	tmp := t.TempDir()
	paths := Paths{
		PluginJSON:      filepath.Join(tmp, "plugin.json"),
		MarketplaceJSON: filepath.Join(tmp, "marketplace.json"),
		CodexPluginJSON: filepath.Join(tmp, ".codex-plugin", "plugin.json"), // never created
		SkillMD:         filepath.Join(tmp, "SKILL.md"),
		ReadmeMD:        filepath.Join(tmp, "README.md"),
	}
	writeFile(t, paths.PluginJSON, `{"name":"evo","version":"21.4.1"}`)
	writeFile(t, paths.MarketplaceJSON, `{"plugins":[{"name":"evo","version":"21.4.1"}]}`)
	writeFile(t, paths.SkillMD, "# Evolve Loop v21.4\n")
	writeFile(t, paths.ReadmeMD, "Current (v21.4)\n\n| v21.4 | Jun 29 | shipped |\n")
	res, err := Run(paths, "21.4.2", false, time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("absent Codex mirror must be tolerated, got error: %v", err)
	}
	for _, m := range res.Modified {
		if m == ".codex-plugin/plugin.json" {
			t.Errorf("absent Codex mirror should not appear in Modified: %v", res.Modified)
		}
	}
}

func TestRun_IdempotentNoMods(t *testing.T) {
	tmp := t.TempDir()
	paths := DefaultPaths(tmp)
	writeFile(t, paths.PluginJSON, `{"version":"11.7.0"}`)
	writeFile(t, paths.MarketplaceJSON, `{"version":"11.7.0"}`)
	writeFile(t, paths.SkillMD, "# Evolve Loop v11.7\n")
	writeFile(t, paths.ReadmeMD, "Current (v11.7)\n| v11.7 | x | y |\n")
	res, err := Run(paths, "11.7.0", false, time.Now())
	if err != nil {
		t.Fatalf("%v", err)
	}
	if len(res.Modified) != 0 {
		t.Errorf("expected 0 mods, got %v", res.Modified)
	}
}

func TestRun_BadSemverErrors(t *testing.T) {
	_, err := Run(DefaultPaths(t.TempDir()), "v1.0.0", false, time.Now())
	if err == nil || !strings.Contains(err.Error(), "not semver") {
		t.Errorf("got %v", err)
	}
}

func TestRun_DryRunReportsButDoesNotWrite(t *testing.T) {
	tmp := t.TempDir()
	paths := Paths{
		PluginJSON:      filepath.Join(tmp, "plugin.json"),
		MarketplaceJSON: filepath.Join(tmp, "marketplace.json"),
	}
	writeFile(t, paths.PluginJSON, `{"version":"11.6.6"}`)
	writeFile(t, paths.MarketplaceJSON, `{"version":"11.6.6"}`)
	res, err := Run(paths, "11.7.0", true, time.Now())
	if err != nil {
		t.Fatalf("%v", err)
	}
	if len(res.Modified) != 2 {
		t.Errorf("expected 2 mod reports (plugin+marketplace), got %v", res.Modified)
	}
	body, _ := os.ReadFile(paths.PluginJSON)
	if !strings.Contains(string(body), "11.6.6") {
		t.Errorf("dry-run mutated file: %s", body)
	}
}

func TestResultJSON_RoundTrip(t *testing.T) {
	r := Result{Target: "11.7.0", Modified: []string{".claude-plugin/plugin.json", "README.md (Current)"}}
	got := r.ResultJSON()
	if !strings.HasSuffix(got, "\n") {
		t.Errorf("missing trailing newline")
	}
	var parsed Result
	if err := json.Unmarshal([]byte(strings.TrimSpace(got)), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, got)
	}
	if parsed.Target != r.Target || len(parsed.Modified) != len(r.Modified) {
		t.Errorf("round-trip mismatch: %+v", parsed)
	}
}

func TestResultJSON_EmptyModified(t *testing.T) {
	r := Result{Target: "11.7.0"}
	got := r.ResultJSON()
	if got != `{"target":"11.7.0","modified":[]}`+"\n" {
		t.Errorf("got %q", got)
	}
}

// roDir creates a directory containing `file` with given content, then makes
// the directory read-only so that atomicwrite.Bytes's tmp-write fails. The cleanup
// restores write permission so t.TempDir teardown can remove it.
func roDir(t *testing.T, content string) string {
	t.Helper()
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "ro")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	p := filepath.Join(dir, "file")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	return p
}

// TestBumpJSONVersion_InvalidJSON pins that a file whose version regex matches
// but whose body is not valid JSON surfaces a "parse" error rather than writing.
func TestBumpJSONVersion_InvalidJSON(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "plugin.json")
	writeFile(t, p, `{"version":"11.6.6" this is broken`)
	_, err := BumpJSONVersion(p, "11.7.0", false)
	if err == nil || !strings.Contains(err.Error(), "parse") {
		t.Errorf("got %v, want a parse error", err)
	}
}

// TestBumpJSONVersion_AtomicWriteError pins that a write failure (read-only
// parent dir) propagates an error and does not silently report success.
func TestBumpJSONVersion_AtomicWriteError(t *testing.T) {
	p := roDir(t, `{"version":"11.6.6"}`)
	changed, err := BumpJSONVersion(p, "11.7.0", false)
	if err == nil {
		t.Fatalf("expected write error")
	}
	if changed {
		t.Errorf("changed must be false on write error, got true")
	}
}

// TestBumpSkillHeading_DryRunReportsButDoesNotWrite pins dry-run semantics:
// reports would-change without mutating the file.
func TestBumpSkillHeading_DryRunReportsButDoesNotWrite(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "SKILL.md")
	writeFile(t, p, "# Evolve Loop v11.6.6\nbody")
	changed, err := BumpSkillHeading(p, "11.7", true)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if !changed {
		t.Errorf("dry-run should report would-change")
	}
	body, _ := os.ReadFile(p)
	if !strings.Contains(string(body), "# Evolve Loop v11.6.6") {
		t.Errorf("dry-run mutated file: %s", body)
	}
}

// TestBumpSkillHeading_ReadErrorNonNotExist pins that a non-IsNotExist read
// error (a directory at the path) surfaces a wrapped read error.
func TestBumpSkillHeading_ReadErrorNonNotExist(t *testing.T) {
	dir := t.TempDir()
	_, err := BumpSkillHeading(dir, "11.7", false)
	if err == nil || !strings.Contains(err.Error(), "read") {
		t.Errorf("got %v, want a read error for directory path", err)
	}
}

// TestBumpSkillHeading_AtomicWriteError pins write-failure propagation.
func TestBumpSkillHeading_AtomicWriteError(t *testing.T) {
	p := roDir(t, "# Evolve Loop v11.6.6\nbody")
	changed, err := BumpSkillHeading(p, "11.7", false)
	if err == nil {
		t.Fatalf("expected write error")
	}
	if changed {
		t.Errorf("changed must be false on write error")
	}
}

// TestCurrentReadmeCurrent_MissingFile pins the read-error return.
func TestCurrentReadmeCurrent_MissingFile(t *testing.T) {
	_, err := CurrentReadmeCurrent(filepath.Join(t.TempDir(), "missing"))
	if err == nil {
		t.Errorf("expected read error for missing file")
	}
}

// TestCurrentReadmeCurrent_NoMatch pins the empty-result (no version) path.
func TestCurrentReadmeCurrent_NoMatch(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "README.md")
	writeFile(t, p, "no current marker here\n")
	got, err := CurrentReadmeCurrent(p)
	if err != nil || got != "" {
		t.Errorf("got %q err %v, want empty", got, err)
	}
}

// TestBumpReadmeCurrent_DryRunReportsButDoesNotWrite pins dry-run semantics.
func TestBumpReadmeCurrent_DryRunReportsButDoesNotWrite(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "README.md")
	writeFile(t, p, "Current (v11.6)\n")
	changed, err := BumpReadmeCurrent(p, "11.7", true)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if !changed {
		t.Errorf("dry-run should report would-change")
	}
	body, _ := os.ReadFile(p)
	if !strings.Contains(string(body), "Current (v11.6)") {
		t.Errorf("dry-run mutated file: %s", body)
	}
}

// TestBumpReadmeCurrent_ReadErrorNonNotExist pins a directory-path read error.
func TestBumpReadmeCurrent_ReadErrorNonNotExist(t *testing.T) {
	dir := t.TempDir()
	_, err := BumpReadmeCurrent(dir, "11.7", false)
	if err == nil || !strings.Contains(err.Error(), "read") {
		t.Errorf("got %v, want a read error for directory path", err)
	}
}

// TestBumpReadmeHistory_ReadErrorNonNotExist pins a directory-path read error.
func TestBumpReadmeHistory_ReadErrorNonNotExist(t *testing.T) {
	dir := t.TempDir()
	_, err := BumpReadmeHistory(dir, "11.7", time.Unix(0, 0).UTC(), false)
	if err == nil || !strings.Contains(err.Error(), "read") {
		t.Errorf("got %v, want a read error for directory path", err)
	}
}

// TestRun_PropagatesBumpErrors pins that a failure in any single bump phase
// aborts Run and returns that error, rather than silently continuing. One
// subtest per phase positions the failure so the earlier phases are clean
// no-ops (already at target) and the targeted phase errors.
func TestRun_PropagatesBumpErrors(t *testing.T) {
	const target = "11.7.0"
	const mm = "11.7"
	now := time.Date(2026, 5, 24, 0, 0, 0, 0, time.UTC)

	// cleanJSON / cleanSkill / cleanReadme write files already at target so
	// their bump phases are no-op-clean (changed=false, no error).
	cleanJSON := func(t *testing.T, dir, name string) string {
		t.Helper()
		p := filepath.Join(dir, name)
		writeFile(t, p, `{"version":"`+target+`"}`)
		return p
	}

	t.Run("plugin json parse error", func(t *testing.T) {
		tmp := t.TempDir()
		paths := Paths{
			PluginJSON:      filepath.Join(tmp, "plugin.json"),
			MarketplaceJSON: cleanJSON(t, tmp, "marketplace.json"),
			SkillMD:         filepath.Join(tmp, "SKILL.md"),
			ReadmeMD:        filepath.Join(tmp, "README.md"),
		}
		writeFile(t, paths.PluginJSON, `{"version":"11.6.6" broken`)
		_, err := Run(paths, target, false, now)
		if err == nil || !strings.Contains(err.Error(), "parse") {
			t.Errorf("got %v, want parse error from plugin.json phase", err)
		}
	})

	t.Run("marketplace json parse error", func(t *testing.T) {
		tmp := t.TempDir()
		paths := Paths{
			PluginJSON:      cleanJSON(t, tmp, "plugin.json"),
			MarketplaceJSON: filepath.Join(tmp, "marketplace.json"),
			SkillMD:         filepath.Join(tmp, "SKILL.md"),
			ReadmeMD:        filepath.Join(tmp, "README.md"),
		}
		writeFile(t, paths.MarketplaceJSON, `{"version":"11.6.6" broken`)
		_, err := Run(paths, target, false, now)
		if err == nil || !strings.Contains(err.Error(), "parse") {
			t.Errorf("got %v, want parse error from marketplace phase", err)
		}
	})

	t.Run("skill heading read error", func(t *testing.T) {
		tmp := t.TempDir()
		skillDir := filepath.Join(tmp, "skilldir")
		if err := os.Mkdir(skillDir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		paths := Paths{
			PluginJSON:      cleanJSON(t, tmp, "plugin.json"),
			MarketplaceJSON: cleanJSON(t, tmp, "marketplace.json"),
			SkillMD:         skillDir, // directory → non-IsNotExist read error
			ReadmeMD:        filepath.Join(tmp, "README.md"),
		}
		_, err := Run(paths, target, false, now)
		if err == nil || !strings.Contains(err.Error(), "read") {
			t.Errorf("got %v, want read error from skill phase", err)
		}
	})

	t.Run("readme current read error", func(t *testing.T) {
		tmp := t.TempDir()
		readmeDir := filepath.Join(tmp, "readmedir")
		if err := os.Mkdir(readmeDir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		paths := Paths{
			PluginJSON:      cleanJSON(t, tmp, "plugin.json"),
			MarketplaceJSON: cleanJSON(t, tmp, "marketplace.json"),
			SkillMD:         filepath.Join(tmp, "missing-skill.md"), // IsNotExist → no-op
			ReadmeMD:        readmeDir,                              // directory → read error
		}
		_, err := Run(paths, target, false, now)
		if err == nil || !strings.Contains(err.Error(), "read") {
			t.Errorf("got %v, want read error from README current phase", err)
		}
	})

	t.Run("readme history write error", func(t *testing.T) {
		// Position failure at the history phase: plugin/marketplace clean,
		// skill missing, README's "Current" already at target so the Current
		// phase is a no-op, but the README has an unbumped history row AND
		// lives in a read-only dir so atomicwrite.Bytes fails in the history phase.
		tmp := t.TempDir()
		dir := filepath.Join(tmp, "ro")
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		paths := Paths{
			PluginJSON:      filepath.Join(dir, "plugin.json"),
			MarketplaceJSON: filepath.Join(dir, "marketplace.json"),
			SkillMD:         filepath.Join(dir, "missing-skill.md"),
			ReadmeMD:        filepath.Join(dir, "README.md"),
		}
		writeFile(t, paths.PluginJSON, `{"version":"`+target+`"}`)
		writeFile(t, paths.MarketplaceJSON, `{"version":"`+target+`"}`)
		// "Current" already at target (no-op), history row unbumped (needs write).
		writeFile(t, paths.ReadmeMD, "Current (v"+mm+")\n| v11.6 | May 23 | shipped |\n")
		if err := os.Chmod(dir, 0o555); err != nil {
			t.Fatalf("chmod: %v", err)
		}
		t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
		_, err := Run(paths, target, false, now)
		if err == nil {
			t.Fatalf("expected write error from history phase")
		}
	})
}
