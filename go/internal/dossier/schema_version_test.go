package dossier

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// legacyRetroMislabelRecord mirrors the unversioned corpus: no discriminator,
// one synthesized phase, and retro listed in skipped_phases.
const legacyRetroMislabelRecord = `{
  "cycle": 823,
  "goal": "legacy fixture",
  "final_verdict": "FAIL",
  "phases": [{"name": "cycle-recorded", "verdict": "FAIL"}],
  "defects": [{"id": "audit-fail", "severity": "HIGH", "summary": "cycle did not pass audit"}],
  "carryover": [{"id": "address-audit-findings", "action": "resolve the audit findings"}],
  "skipped_phases": [{"phase": "retro", "reason": "FAIL"}]
}`

func wireMap(t *testing.T, d *Dossier) map[string]any {
	t.Helper()
	raw, err := RenderJSON(d)
	if err != nil {
		t.Fatalf("RenderJSON: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal rendered dossier: %v", err)
	}
	return m
}

func TestSchemaVersion_BuildStampsTheDiscriminator(t *testing.T) {
	if CurrentSchemaVersion < 2 {
		t.Fatalf("CurrentSchemaVersion = %d, want >= 2 (1 is the implicit version of the unstamped pre-discriminator corpus)", CurrentSchemaVersion)
	}
	d, err := Build(4300, BuildOpts{WorkspacePath: t.TempDir(), Goal: "discriminator stamp"})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if d.SchemaVersion != CurrentSchemaVersion {
		t.Errorf("Build left SchemaVersion = %d, want CurrentSchemaVersion (%d)", d.SchemaVersion, CurrentSchemaVersion)
	}
	m := wireMap(t, d)
	got, ok := m["schema_version"]
	if !ok {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		t.Fatalf("RED: rendered record carries no \"schema_version\" key; top-level keys=%v", keys)
	}
	n, isNum := got.(float64)
	if !isNum || n != float64(CurrentSchemaVersion) || n != float64(int(n)) {
		t.Errorf("schema_version on the wire = %#v, want the integer %d", got, CurrentSchemaVersion)
	}
	raw, _ := RenderJSON(d)
	back, err := ParseJSON(raw)
	if err != nil {
		t.Fatalf("ParseJSON round-trip: %v", err)
	}
	if back.SchemaVersion != CurrentSchemaVersion {
		t.Errorf("round-trip SchemaVersion = %d, want %d", back.SchemaVersion, CurrentSchemaVersion)
	}
	if err := back.Validate(); err != nil {
		t.Errorf("a stamped record must still Validate: %v", err)
	}
}

func TestSchemaVersion_LegacyRecordStaysUnstamped(t *testing.T) {
	d, err := ParseJSON([]byte(legacyRetroMislabelRecord))
	if err != nil {
		t.Fatalf("ParseJSON legacy: %v", err)
	}
	if d.SchemaVersion != 0 {
		t.Errorf("legacy record parsed with SchemaVersion = %d, want 0 (absent)", d.SchemaVersion)
	}
	if d.SchemaVersion >= CurrentSchemaVersion {
		t.Errorf("a record with no schema_version must be BELOW CurrentSchemaVersion (%d), got %d", CurrentSchemaVersion, d.SchemaVersion)
	}
	raw, err := RenderJSON(d)
	if err != nil {
		t.Fatalf("RenderJSON legacy: %v", err)
	}
	if strings.Contains(string(raw), `"schema_version"`) {
		t.Errorf("re-rendering a legacy record stamped it — the discriminator is forward-only, never a silent backfill:\n%s", raw)
	}
	if len(d.SkippedPhases) != 1 || d.SkippedPhases[0].Phase != "retro" {
		t.Errorf("legacy skipped_phases must round-trip untouched, got %+v", d.SkippedPhases)
	}
}

func TestSchemaVersion_SchemaDeclaresTheField(t *testing.T) {
	path := filepath.Join(repoRootFromTest(t), "schemas", "cycle-dossier.schema.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	var doc struct {
		Required   []string `json:"required"`
		Properties map[string]struct {
			Type string `json:"type"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	prop, ok := doc.Properties["schema_version"]
	if !ok {
		t.Fatalf("RED: schemas/cycle-dossier.schema.json declares no root property \"schema_version\"")
	}
	if prop.Type != "integer" {
		t.Errorf("schema_version type = %q, want \"integer\"", prop.Type)
	}
	for _, r := range doc.Required {
		if r == "schema_version" {
			t.Errorf("schema_version is listed in required — the legacy corpus omits it and would be rejected")
		}
	}
}
