package preflight

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestProfile_MarshalJSON_NamedDirectCall(t *testing.T) {
	t.Parallel()
	p := Probe(Options{
		ProjectRoot: t.TempDir(),
		OSType:      "darwin",
		Env:         stubEnv(map[string]string{"HOME": "/Users/test"}),
		LookPath:    stubLookPath(nil),
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
	var rt Profile
	if err := json.Unmarshal(b, &rt); err != nil {
		t.Fatalf("MarshalJSON output not re-parseable: %v", err)
	}
	if rt.SchemaVersion != 3 {
		t.Errorf("round-trip schema_version = %d, want 3", rt.SchemaVersion)
	}
}

func TestMeasuredSandboxCapability_Named(t *testing.T) {
	capable, checked := MeasuredSandboxCapability()
	if capable && !checked {
		t.Error("MeasuredSandboxCapability: capable=true with checked=false violates capable ⇒ checked")
	}
}

func TestCLIBins_NamedFullStructEquality(t *testing.T) {
	t.Parallel()
	claudePath := "/usr/local/bin/claude"
	want := CLIBins{Claude: &claudePath}

	b, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal CLIBins: %v", err)
	}
	got := string(b)
	if !strings.Contains(got, `"claude":"/usr/local/bin/claude"`) {
		t.Errorf("CLIBins claude path not encoded: %s", got)
	}

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
