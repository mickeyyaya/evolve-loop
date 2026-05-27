package phasespec

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// writeRegistry writes a registry JSON fixture into a temp dir and returns its path.
func writeRegistry(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "phase-registry.json")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return p
}

const fullRegistry = `{
  "schema_version": 4,
  "config": { "mandatory_phases": ["scout","build","audit","ship"] },
  "phases": [
    {
      "name": "scout",
      "kind": "llm",
      "optional": false,
      "agent": "evolve-scout",
      "model": "auto",
      "writes_source": false,
      "inputs":  { "files": ["intent.md"], "signals": [] },
      "outputs": { "files": ["scout-report.md"], "signals": ["scout.cycle_size","scout.item_count"] },
      "prompt_context": ["goal"],
      "classify": { "require_sections": ["## Proposed Tasks"], "fail_if_empty": true, "verdict_on_pass": "PASS" },
      "gates": { "in": "gate_intent_to_discover", "out": "gate_discover_to_triage" }
    },
    {
      "name": "security-scan",
      "optional": true,
      "writes_source": false,
      "outputs": { "signals": ["security.severity_max"] },
      "classify": { "require_sections": ["## Findings"], "fail_if_signal": { "security.severity_max": ">=HIGH" } },
      "routing": { "insert_when": [ { "field": "build.files_touched", "op": "gt", "value": 0 } ] }
    }
  ]
}`

func TestLoad_FullSpecFields(t *testing.T) {
	cat, err := Load(writeRegistry(t, fullRegistry))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	scout, ok := cat.Get("scout")
	if !ok {
		t.Fatal("scout not found in catalog")
	}
	if scout.Kind != "llm" || scout.Optional {
		t.Errorf("scout kind/optional = %q/%v", scout.Kind, scout.Optional)
	}
	if got := scout.Outputs.Signals; len(got) != 2 || got[0] != "scout.cycle_size" {
		t.Errorf("scout output signals = %v", got)
	}
	if scout.Classify == nil || !scout.Classify.FailIfEmpty || scout.Classify.VerdictOnPass != "PASS" {
		t.Errorf("scout classify = %+v", scout.Classify)
	}
	if scout.Gates.In != "gate_intent_to_discover" {
		t.Errorf("scout gate_in = %q", scout.Gates.In)
	}

	sec, ok := cat.Get("security-scan")
	if !ok {
		t.Fatal("security-scan not found")
	}
	if sec.Routing == nil || len(sec.Routing.InsertWhen) != 1 || sec.Routing.InsertWhen[0].Field != "build.files_touched" {
		t.Errorf("security routing = %+v", sec.Routing)
	}
	if sec.Classify == nil || sec.Classify.FailIfSignal["security.severity_max"] != ">=HIGH" {
		t.Errorf("security classify.fail_if_signal = %+v", sec.Classify)
	}
}

func TestSpecDefaults(t *testing.T) {
	cases := []struct {
		name      string
		spec      PhaseSpec
		wantKind  string
		wantAgent string
		wantModel string
	}{
		{"explicit", PhaseSpec{Name: "scout", Kind: "native", Agent: "evolve-scout", Model: "opus"}, "native", "evolve-scout", "opus"},
		{"defaults", PhaseSpec{Name: "my-phase"}, "llm", "evolve-my-phase", "auto"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.spec.KindOrDefault(); got != tc.wantKind {
				t.Errorf("KindOrDefault = %q, want %q", got, tc.wantKind)
			}
			if got := tc.spec.AgentName(); got != tc.wantAgent {
				t.Errorf("AgentName = %q, want %q", got, tc.wantAgent)
			}
			if got := tc.spec.ModelOrDefault(); got != tc.wantModel {
				t.Errorf("ModelOrDefault = %q, want %q", got, tc.wantModel)
			}
		})
	}
}

func TestCatalog_NamesAndOrder(t *testing.T) {
	cat, err := Load(writeRegistry(t, fullRegistry))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// All() preserves registry (insertion) order; Names() returns a sorted snapshot.
	all := cat.All()
	if len(all) != 2 || all[0].Name != "scout" || all[1].Name != "security-scan" {
		t.Errorf("All order = %v, want [scout security-scan] (registry order)", names(all))
	}
	got := cat.Names()
	if !sort.StringsAreSorted(got) {
		t.Errorf("Names = %v, want sorted", got)
	}
	if len(got) != 2 {
		t.Errorf("Names len = %d, want 2", len(got))
	}
}

func TestLoad_DuplicateName_FirstWins(t *testing.T) {
	// Locks the merge contract Stage 4 depends on: when the same name appears
	// twice, the first entry wins and the second is dropped (built-ins precede
	// user overlays at merge time).
	dup := `{ "phases": [
		{ "name": "scout", "model": "opus" },
		{ "name": "scout", "model": "haiku" }
	] }`
	cat, err := Load(writeRegistry(t, dup))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cat.All()) != 1 {
		t.Fatalf("expected 1 spec after dedup, got %d", len(cat.All()))
	}
	s, _ := cat.Get("scout")
	if s.Model != "opus" {
		t.Errorf("Model = %q, want opus (first wins)", s.Model)
	}
}

func TestLoad_MissingFileErrors(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Fatal("expected error for missing registry file")
	}
}

func TestLoad_MalformedJSONErrors(t *testing.T) {
	if _, err := Load(writeRegistry(t, "{not json")); err == nil {
		t.Fatal("expected error for malformed registry")
	}
}

func names(s []PhaseSpec) []string {
	out := make([]string, len(s))
	for i, p := range s {
		out[i] = p.Name
	}
	return out
}
