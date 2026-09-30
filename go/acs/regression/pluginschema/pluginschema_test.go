//go:build acs

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

var allowedMarketplaceEntryKeys = map[string]bool{
	"name": true, "source": true, "description": true, "version": true,
	"author": true, "homepage": true, "repository": true, "license": true,
	"keywords": true, "category": true, "tags": true, "strict": true,
}

func decodeObject(t *testing.T, label string, raw []byte) map[string]json.RawMessage {
	t.Helper()
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("parse %s: %v", label, err)
	}
	return obj
}

func loadRepoJSON(t *testing.T, rel string) []byte {
	t.Helper()
	path := filepath.Join(acsassert.RepoRoot(t), filepath.FromSlash(rel))
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return raw
}

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

func TestClaudePlugin_BinariesIsRecordOrAbsent(t *testing.T) {
	obj := decodeObject(t, ".claude-plugin/plugin.json", loadRepoJSON(t, ".claude-plugin/plugin.json"))
	if present, offending := binariesPresentButNotRecord(obj); present && offending {
		t.Errorf(".claude-plugin/plugin.json `binaries` is present but not a JSON object — " +
			"CC 2.1.195 types it as a record and rejects an array/string with " +
			"\"expected record, received array\". Remove it (release matrix SSOT is .goreleaser.yml) " +
			"or express it as a record of <basename> -> {sha256, platforms}.")
	}
}

func TestClaudePlugin_NoCompatibilityField(t *testing.T) {
	obj := decodeObject(t, ".claude-plugin/plugin.json", loadRepoJSON(t, ".claude-plugin/plugin.json"))
	if _, ok := obj["compatibility"]; ok {
		t.Errorf(".claude-plugin/plugin.json carries a custom `compatibility` key — " +
			"keep platform/tier docs in docs/platform-compatibility.md, not in the CC manifest.")
	}
}

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
