package dossier

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
)

func TestBuild_ProjectsModelSourceAndResolvedModel(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	entries := []phasetiming.Entry{
		{Phase: "scout", DurationMS: 100, Verdict: "PASS", Archetype: "plan", AttemptCount: 1, ModelSource: "profile", ResolvedModel: "sonnet"},
		{Phase: "build", DurationMS: 200, Verdict: "PASS", Archetype: "build", AttemptCount: 1, ModelSource: "advisor", ResolvedModel: "opus"},
		{Phase: "audit", DurationMS: 150, Verdict: "PASS", Archetype: "evaluate", AttemptCount: 1, ModelSource: "pin", ResolvedModel: "opus"},
	}
	data, _ := json.Marshal(entries)
	if err := os.WriteFile(filepath.Join(ws, phasetiming.FileName), data, 0o644); err != nil {
		t.Fatal(err)
	}

	d, err := Build(10, BuildOpts{WorkspacePath: ws, Goal: "g", FinalVerdict: VerdictPass})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if err := d.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	byName := map[string]PhaseRecord{}
	for _, p := range d.Phases {
		byName[p.Name] = p
	}
	for name, want := range map[string]struct{ source, model string }{
		"scout": {"profile", "sonnet"},
		"build": {"advisor", "opus"},
		"audit": {"pin", "opus"},
	} {
		rec, ok := byName[name]
		if !ok {
			t.Fatalf("missing phase record %q", name)
		}
		if rec.ModelSource != want.source {
			t.Errorf("%s.ModelSource=%q, want %q", name, rec.ModelSource, want.source)
		}
		if rec.ResolvedModel != want.model {
			t.Errorf("%s.ResolvedModel=%q, want %q", name, rec.ResolvedModel, want.model)
		}
	}
}

func TestBuild_LegacyWorkspaceWithoutModelMetadataDegradesSafely(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	// Deliberately the OLD shape: no model_source/resolved_model keys.
	legacy := `[{"phase":"scout","duration_ms":100,"verdict":"PASS","archetype":"plan","attempt_count":1}]`
	if err := os.WriteFile(filepath.Join(ws, phasetiming.FileName), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	d, err := Build(11, BuildOpts{WorkspacePath: ws, Goal: "g", FinalVerdict: VerdictPass})
	if err != nil {
		t.Fatalf("Build must not error on a legacy timing log: %v", err)
	}
	if err := d.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if len(d.Phases) != 1 {
		t.Fatalf("phases = %d, want 1", len(d.Phases))
	}
	if got := d.Phases[0].ModelSource; got != "" {
		t.Errorf("legacy record ModelSource=%q, want \"\" (absent, never fabricated)", got)
	}
}
