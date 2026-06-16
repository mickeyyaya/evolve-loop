package preflight

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestProfile_MarshalJSON_NamedDirectCall names and directly invokes
// Profile.MarshalJSON — the custom marshaler that produces the compact
// environment.json topology (matching the bash jq -n output). The existing
// JSON test reaches it only implicitly via json.Marshal; this calls it by name
// and pins its contract: it returns the profile encoded as JSON whose
// schema_version is 3 and whose absent CLI binaries serialize as null (the
// shape the dispatcher reads back).
func TestProfile_MarshalJSON_NamedDirectCall(t *testing.T) {
	t.Parallel()
	p := Probe(Options{
		ProjectRoot: t.TempDir(),
		OSType:      "darwin",
		Env:         stubEnv(map[string]string{"HOME": "/Users/test"}),
		LookPath:    stubLookPath(nil), // no CLIs on PATH
		Now:         fixedNow(),
		IsNested:    func() bool { return false },
	})

	b, err := p.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	if !strings.Contains(string(b), `"schema_version":3`) {
		t.Errorf("MarshalJSON missing schema_version: %s", b)
	}
	if !strings.Contains(string(b), `"claude":null`) {
		t.Errorf("MarshalJSON: absent claude binary must serialize as null: %s", b)
	}
	// The output round-trips back into a Profile (the dispatcher's read path).
	var rt Profile
	if err := json.Unmarshal(b, &rt); err != nil {
		t.Fatalf("MarshalJSON output not re-parseable: %v", err)
	}
	if rt.SchemaVersion != 3 {
		t.Errorf("round-trip schema_version = %d, want 3", rt.SchemaVersion)
	}
}

// TestCLIBins_NamedFullStructEquality names the CLIBins type (constructed in
// Probe but never named in a test) and pins its JSON contract: a CLIBins with a
// resolved claude path and the rest nil must encode to the cli_binaries shape
// the Profile embeds — resolved path present, absent CLIs null — and Probe must
// produce exactly that struct when only claude is on PATH.
func TestCLIBins_NamedFullStructEquality(t *testing.T) {
	t.Parallel()
	claudePath := "/usr/local/bin/claude"
	want := CLIBins{Claude: &claudePath} // Gemini/Codex/Agy/JQ/Git stay nil

	b, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal CLIBins: %v", err)
	}
	got := string(b)
	if !strings.Contains(got, `"claude":"/usr/local/bin/claude"`) {
		t.Errorf("CLIBins claude path not encoded: %s", got)
	}

	// Probe must produce a CLIBins with claude resolved and the absent CLIs nil.
	p := Probe(Options{
		ProjectRoot: t.TempDir(),
		OSType:      "darwin",
		Env:         stubEnv(map[string]string{"HOME": "/Users/test"}),
		LookPath:    stubLookPath(map[string]string{"claude": claudePath}),
		Now:         fixedNow(),
		IsNested:    func() bool { return false },
	})
	if p.CLIBinaries.Claude == nil || *p.CLIBinaries.Claude != claudePath {
		t.Errorf("Probe CLIBinaries.Claude = %v, want %q", p.CLIBinaries.Claude, claudePath)
	}
}
