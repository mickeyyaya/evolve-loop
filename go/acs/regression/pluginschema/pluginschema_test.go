//go:build acs

// Package pluginschema is the durable regression guard for the 2026-06-29
// Claude Code 2.1.195 plugin-manifest schema break.
//
// CC 2.1.195 tightened plugin validation and *claimed* the `binaries` key as a
// native field — `binaries: record(<basename> -> {sha256, platforms})`. evo had
// repurposed `binaries` as a documentation ARRAY and added a custom
// `compatibility` object. The result was a hard install failure:
//
//	.claude-plugin/marketplace.json plugin entry — CC's marketplace-entry schema
//	  is .strict(); the unknown `binaries`/`compatibility` keys surfaced as the
//	  MISLEADING error "This plugin uses a source type your Claude Code version
//	  does not support."
//	.claude-plugin/plugin.json — `binaries: Invalid input: expected record,
//	  received array`.
//
// Both fields were documentation-only (release matrix SSOT is .goreleaser.yml;
// compatibility tiers live in docs/platform-compatibility.md) and were removed.
// This gate pins that removal and the shape rules so the install-blocking class
// can never silently return. It encodes the schema RULES and checks both the
// live repo manifests AND adversarial fixtures, so a failure means a real break,
// not a tautology.
//
// acs-tagged like every go/acs/regression predicate; CI runs it via
//
//	go test -count=1 -tags acs ./acs/regression/...
//
// Test-only package outside ./internal/...; no .apicover-enforce enrollment
// (same as acs/regression/pluginnamespace, noorphan, flagreaders).
package pluginschema

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// allowedMarketplaceEntryKeys is the key set CC 2.1.195's .strict()
// marketplace plugin-entry schema accepts, calibrated against the marketplaces
// that install clean on this CC version (ecc, worktrunk). Any key outside this
// set is rejected by CC — `binaries` and `compatibility` are exactly what broke.
// Editing this set is a deliberate decision to track a CC schema change; an
// accidental reintroduction of a custom field is not, and is what this catches.
var allowedMarketplaceEntryKeys = map[string]bool{
	"name": true, "source": true, "description": true, "version": true,
	"author": true, "homepage": true, "repository": true, "license": true,
	"keywords": true, "category": true, "tags": true, "strict": true,
}

// decodeObject parses raw JSON into an ordered-irrelevant key map, failing the
// test loudly on malformed input (a manifest that does not parse is itself a
// regression).
func decodeObject(t *testing.T, label string, raw []byte) map[string]json.RawMessage {
	t.Helper()
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("parse %s: %v", label, err)
	}
	return obj
}

// loadRepoJSON reads a repo-relative JSON manifest as raw bytes.
func loadRepoJSON(t *testing.T, rel string) []byte {
	t.Helper()
	path := filepath.Join(acsassert.RepoRoot(t), filepath.FromSlash(rel))
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return raw
}

// binariesPresentButNotRecord is the exact CC 2.1.195 rule that hard-failed:
// `binaries` may be ABSENT or a JSON object (record); an array or string (evo's
// old documentation shape) is rejected with "expected record, received array".
// Returns (present, offending) — offending is true only when present and not a
// JSON object.
func binariesPresentButNotRecord(obj map[string]json.RawMessage) (present, offending bool) {
	raw, ok := obj["binaries"]
	if !ok {
		return false, false
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return false, false
	}
	return true, trimmed[0] != '{'
}

// unsupportedEntryKeys returns the keys of a marketplace plugin entry that fall
// outside CC's .strict() entry schema, sorted for stable output.
func unsupportedEntryKeys(entry map[string]json.RawMessage) []string {
	var bad []string
	for k := range entry {
		if !allowedMarketplaceEntryKeys[k] {
			bad = append(bad, k)
		}
	}
	sort.Strings(bad)
	return bad
}

// --- Live-manifest guards: lock the fixed state of the real repo files. ---

// TestClaudePlugin_BinariesIsRecordOrAbsent pins the exact field/type that
// hard-failed install on CC 2.1.195. An array `binaries` must never return.
func TestClaudePlugin_BinariesIsRecordOrAbsent(t *testing.T) {
	obj := decodeObject(t, ".claude-plugin/plugin.json", loadRepoJSON(t, ".claude-plugin/plugin.json"))
	if present, offending := binariesPresentButNotRecord(obj); present && offending {
		t.Errorf(".claude-plugin/plugin.json `binaries` is present but not a JSON object — " +
			"CC 2.1.195 types it as a record and rejects an array/string with " +
			"\"expected record, received array\". Remove it (release matrix SSOT is .goreleaser.yml) " +
			"or express it as a record of <basename> -> {sha256, platforms}.")
	}
}

// TestClaudePlugin_NoCompatibilityField locks the compatibility removal. The
// plugin.json manifest schema is passthrough today (CC strips unknown keys), but
// `compatibility` is dead weight and becomes a hard failure the moment CC flips
// the manifest to .strict() (its frontmatter schemas already are). Compatibility
// tiers are documented in docs/platform-compatibility.md.
func TestClaudePlugin_NoCompatibilityField(t *testing.T) {
	obj := decodeObject(t, ".claude-plugin/plugin.json", loadRepoJSON(t, ".claude-plugin/plugin.json"))
	if _, ok := obj["compatibility"]; ok {
		t.Errorf(".claude-plugin/plugin.json carries a custom `compatibility` key — " +
			"keep platform/tier docs in docs/platform-compatibility.md, not in the CC manifest.")
	}
}

// TestClaudeMarketplace_EntriesUseStandardKeysOnly enforces CC's .strict()
// marketplace plugin-entry schema. The unknown `binaries`/`compatibility` keys
// here produced the misleading "source type ... not supported" install error.
func TestClaudeMarketplace_EntriesUseStandardKeysOnly(t *testing.T) {
	raw := loadRepoJSON(t, ".claude-plugin/marketplace.json")
	var mp struct {
		Plugins []map[string]json.RawMessage `json:"plugins"`
	}
	if err := json.Unmarshal(raw, &mp); err != nil {
		t.Fatalf("parse marketplace.json plugins[]: %v", err)
	}
	if len(mp.Plugins) == 0 {
		t.Fatal(".claude-plugin/marketplace.json has no plugins[] entry")
	}
	for i, entry := range mp.Plugins {
		if bad := unsupportedEntryKeys(entry); len(bad) > 0 {
			t.Errorf("marketplace.json plugins[%d] has keys CC's strict schema rejects: %v "+
				"(this class of key — binaries/compatibility — caused the misleading "+
				"\"source type not supported\" install failure)", i, bad)
		}
	}
}

// TestCodexManifest_InSyncWithClaude is the D3 drift guard: the generated Codex
// mirror (.codex-plugin/plugin.json) must carry the same name + version as the
// canonical .claude-plugin/plugin.json, so neither `evolve release` (versionbump)
// nor the skillcheck Codex projection can leave the Codex install surface on a
// stale version. A mismatch means someone hand-edited a manifest or skipped
// `evolve skills generate`.
func TestCodexManifest_InSyncWithClaude(t *testing.T) {
	claude := decodeObject(t, ".claude-plugin/plugin.json", loadRepoJSON(t, ".claude-plugin/plugin.json"))
	codex := decodeObject(t, ".codex-plugin/plugin.json", loadRepoJSON(t, ".codex-plugin/plugin.json"))
	for _, field := range []string{"name", "version"} {
		if !bytes.Equal(claude[field], codex[field]) {
			t.Errorf(".codex-plugin/plugin.json %s = %s, want %s (== .claude-plugin/plugin.json) — run `evolve skills generate`",
				field, codex[field], claude[field])
		}
	}
}

// --- Detection-logic guards: prove the rules CATCH the known breaks. ---

// TestBinariesRule_CatchesArrayAndString verifies the shape rule rejects exactly
// the inputs that broke install and accepts the valid ones — so the live guard
// above is meaningful, not a tautology.
func TestBinariesRule_CatchesArrayAndString(t *testing.T) {
	cases := []struct {
		name          string
		json          string
		wantPresent   bool
		wantOffending bool
	}{
		{"absent", `{"name":"evo"}`, false, false},
		{"null", `{"binaries":null}`, false, false},
		{"array (the break)", `{"binaries":[{"name":"evolve"}]}`, true, true},
		{"string", `{"binaries":"./go"}`, true, true},
		{"record (valid)", `{"binaries":{"evolve":{"sha256":"ab"}}}`, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			obj := decodeObject(t, tc.name, []byte(tc.json))
			present, offending := binariesPresentButNotRecord(obj)
			if present != tc.wantPresent || offending != tc.wantOffending {
				t.Errorf("binariesPresentButNotRecord(%s) = (present=%v, offending=%v), want (%v, %v)",
					tc.json, present, offending, tc.wantPresent, tc.wantOffending)
			}
		})
	}
}

// TestEntryKeyRule_CatchesCustomFields verifies the strict-entry rule flags the
// exact custom keys that broke and passes a standard entry.
func TestEntryKeyRule_CatchesCustomFields(t *testing.T) {
	cases := []struct {
		name string
		json string
		want []string
	}{
		{"standard entry", `{"name":"evo","source":"./","description":"d","version":"1.0.0","strict":false}`, nil},
		{"binaries key (the break)", `{"name":"evo","source":"./","binaries":[]}`, []string{"binaries"}},
		{"compatibility key (the break)", `{"name":"evo","source":"./","compatibility":{}}`, []string{"compatibility"}},
		{"both", `{"name":"evo","binaries":[],"compatibility":{}}`, []string{"binaries", "compatibility"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry := decodeObject(t, tc.name, []byte(tc.json))
			got := unsupportedEntryKeys(entry)
			if len(got) != len(tc.want) {
				t.Fatalf("unsupportedEntryKeys(%s) = %v, want %v", tc.json, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("unsupportedEntryKeys(%s)[%d] = %q, want %q", tc.json, i, got[i], tc.want[i])
				}
			}
		})
	}
}
