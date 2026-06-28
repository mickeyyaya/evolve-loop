package skillcheck

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeClaudeManifest drops a minimal canonical Claude plugin.json — the single
// source the Codex projection reads.
func writeClaudeManifest(t *testing.T, root string) {
	t.Helper()
	pj := `{
  "name": "evo",
  "version": "9.9.9",
  "description": "desc here",
  "author": { "name": "Dan Lee" },
  "homepage": "https://example.com/repo",
  "repository": "https://example.com/repo",
  "license": "Apache-2.0",
  "keywords": ["a", "b"]
}`
	dir := filepath.Join(root, ".claude-plugin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(pj), 0o644); err != nil {
		t.Fatal(err)
	}
}

// genCodex writes the rendered manifests to disk (simulating `skills generate`).
func genCodex(t *testing.T, root string) {
	t.Helper()
	diffs, err := codexManifestDiffs(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range diffs {
		if err := os.MkdirAll(filepath.Dir(d.path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(d.path, []byte(d.next), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestRenderCodexPluginManifest_SourcedFromClaude — every shared field traces to
// the Claude manifest (single source); Codex-only fields are the projection
// constants. Version especially must come from source (the D3 drift risk).
func TestRenderCodexPluginManifest_SourcedFromClaude(t *testing.T) {
	meta := claudePluginMeta{
		Name: "evo", Version: "9.9.9", Description: "d",
		Author: codexAuthor{Name: "Dan Lee"}, Homepage: "h", Repository: "r",
		License: "Apache-2.0", Keywords: []string{"a"},
	}
	b, err := renderCodexPluginManifest(meta)
	if err != nil {
		t.Fatal(err)
	}
	var got codexPluginManifest
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("rendered manifest is not valid JSON: %v", err)
	}
	if got.Version != "9.9.9" {
		t.Errorf("version = %q, want 9.9.9 (from Claude source)", got.Version)
	}
	if got.Name != "evo" {
		t.Errorf("name = %q, want evo", got.Name)
	}
	if got.Skills != "./skills/" {
		t.Errorf("skills = %q, want ./skills/ (Codex projection const)", got.Skills)
	}
	if got.Interface.DisplayName != codexDisplayName {
		t.Errorf("interface.displayName = %q, want %q", got.Interface.DisplayName, codexDisplayName)
	}
	if !strings.HasSuffix(string(b), "}\n") {
		t.Error("manifest must end with a trailing newline (canonical form)")
	}
}

// TestRenderCodexMarketplace_StrictSchemaConformance — Codex's .strict()
// marketplace schema rejects authentication "NONE"; the projection must emit a
// valid enum and the repo-root source.path.
func TestRenderCodexMarketplace_StrictSchemaConformance(t *testing.T) {
	b, err := renderCodexMarketplace(claudePluginMeta{Name: "evo"})
	if err != nil {
		t.Fatal(err)
	}
	var got codexMarketplaceManifest
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}
	if got.Name != "evo" {
		t.Errorf("marketplace name = %q, want evo", got.Name)
	}
	if len(got.Plugins) != 1 {
		t.Fatalf("want exactly 1 plugin entry, got %d", len(got.Plugins))
	}
	p := got.Plugins[0]
	if p.Name != "evo" {
		t.Errorf("plugin name = %q, want evo (so `codex plugin add evo@evo` resolves)", p.Name)
	}
	if p.Source.Path != "." {
		t.Errorf("source.path = %q, want . (repo root is the plugin)", p.Source.Path)
	}
	if p.Policy.Authentication != "ON_USE" && p.Policy.Authentication != "ON_INSTALL" {
		t.Errorf("authentication = %q, want ON_USE/ON_INSTALL (Codex rejects NONE)", p.Policy.Authentication)
	}
}

// TestCodexManifestDiffs_DriftLifecycle — both manifests read as drifted when
// missing and in-sync once generated, so `skills generate`/`check` behave.
func TestCodexManifestDiffs_DriftLifecycle(t *testing.T) {
	root := t.TempDir()
	writeClaudeManifest(t, root)

	diffs, err := codexManifestDiffs(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(diffs) != 2 {
		t.Fatalf("want 2 Codex manifests (plugin + marketplace), got %d", len(diffs))
	}
	for _, d := range diffs {
		if !d.drifted {
			t.Errorf("%s: want drifted (file missing), got in-sync", d.rel)
		}
	}

	genCodex(t, root)
	diffs, err = codexManifestDiffs(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range diffs {
		if d.drifted {
			t.Errorf("%s: want in-sync after generate, got drift", d.rel)
		}
	}
}

// TestCodexManifestDiffs_ToleratedAbsentSource — a checkout without the
// canonical Claude manifest is not an evo plugin repo; the projection must
// no-op (nil, nil), never fault the whole `skills check` run.
func TestCodexManifestDiffs_ToleratedAbsentSource(t *testing.T) {
	diffs, err := codexManifestDiffs(t.TempDir()) // no .claude-plugin/plugin.json
	if err != nil {
		t.Fatalf("absent Claude manifest must be tolerated, got: %v", err)
	}
	if diffs != nil {
		t.Errorf("want nil diffs when source is absent, got %v", diffs)
	}
}

// TestCodexManifestDiffs_DetectsVersionDrift is the D3 guard: a Codex manifest
// whose version no longer matches the Claude source is flagged as drift (the
// exact stale-version condition `skills check` must catch in CI).
func TestCodexManifestDiffs_DetectsVersionDrift(t *testing.T) {
	root := t.TempDir()
	writeClaudeManifest(t, root)
	genCodex(t, root)

	path := filepath.Join(root, filepath.FromSlash(codexPluginManifestRel))
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	staled := strings.Replace(string(raw), `"9.9.9"`, `"0.0.1"`, 1)
	if staled == string(raw) {
		t.Fatal("setup: version string not found to stale")
	}
	if err := os.WriteFile(path, []byte(staled), 0o644); err != nil {
		t.Fatal(err)
	}

	diffs, err := codexManifestDiffs(root)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, d := range diffs {
		if d.rel == codexPluginManifestRel && d.drifted {
			found = true
		}
	}
	if !found {
		t.Error("stale .codex-plugin/plugin.json version not detected as drift")
	}
}
