package dossier

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
)

func mustBuild(t *testing.T, cycle int, opts BuildOpts) *Dossier {
	t.Helper()
	d, err := Build(cycle, opts)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if verr := d.Validate(); verr != nil {
		t.Fatalf("Validate: %v", verr)
	}
	return d
}

func baseOpts(ws string) BuildOpts {
	return BuildOpts{WorkspacePath: ws, Goal: "g", RunID: "R", FinalVerdict: "PASS"}
}

func TestBuild_LivePhaseTimingsAreAuthoritative(t *testing.T) {
	ws := t.TempDir() // deliberately EMPTY: no phase-timing.json on disk
	opts := baseOpts(ws)
	opts.PhaseTimings = []phasetiming.Entry{
		{Phase: "scout", Verdict: "PASS", DurationMS: 1000},
		{Phase: "build", Verdict: "WARN", DurationMS: 2000},
		{Phase: "ship", Verdict: "PASS", DurationMS: 300},
	}
	d := mustBuild(t, 1623, opts)
	var names []string
	for _, p := range d.Phases {
		names = append(names, p.Name)
	}
	if got := strings.Join(names, ","); got != "scout,build,ship" {
		t.Fatalf("phases = %q, want the live timings verbatim", got)
	}
	if d.Phases[1].Verdict != "WARN" || d.Phases[1].DurationMS != 2000 {
		t.Errorf("per-phase evidence must survive the projection: %+v", d.Phases[1])
	}
}

func TestBuild_AbsentEvidenceIsLoudNeverASynthesizedPass(t *testing.T) {
	d := mustBuild(t, 1623, baseOpts(t.TempDir()))
	if len(d.Phases) != 1 {
		t.Fatalf("want exactly one degradation marker, got %+v", d.Phases)
	}
	p := d.Phases[0]
	if p.Name == "cycle-recorded" {
		t.Fatal("the synthesized phase is back — absence must not be dressed as a recorded cycle")
	}
	if p.Verdict == "PASS" {
		t.Errorf("a record with no evidence must not carry a PASS phase verdict; got %+v", p)
	}
	if !strings.Contains(strings.ToLower(p.KeyFindings), "unavailable") &&
		!strings.Contains(strings.ToLower(p.KeyFindings), "degraded") {
		t.Errorf("the marker must name the degradation; got %q", p.KeyFindings)
	}
	if d.FinalVerdict != "PASS" {
		t.Errorf("FinalVerdict must stay the cycle's real outcome; got %q", d.FinalVerdict)
	}
}

func TestBuild_ProjectsTheShippedCommit(t *testing.T) {
	ws := t.TempDir()
	writeJSON(t, filepath.Join(ws, ShipBindingFile), ShipBinding{
		CommitSHA:        "dc00395aa41c890805aa77c5a674387b8f2c9307",
		TreeSHACommitted: "3388ca0b573a5a5fe9bdeeb4771e4fb7b517b06e",
	})
	d := mustBuild(t, 1623, baseOpts(ws))
	if d.CommitSHA != "dc00395aa41c890805aa77c5a674387b8f2c9307" {
		t.Fatalf("CommitSHA = %q, want the shipped commit from ship-binding.json", d.CommitSHA)
	}
	if d.TreeSHA != "3388ca0b573a5a5fe9bdeeb4771e4fb7b517b06e" {
		t.Errorf("TreeSHA = %q, want the committed tree from the same binding", d.TreeSHA)
	}
	if d2 := mustBuild(t, 1623, baseOpts(t.TempDir())); d2.CommitSHA != "" {
		t.Errorf("absent ship-binding must leave CommitSHA empty; got %q", d2.CommitSHA)
	}
}

func TestBuild_ProjectsTheCommittedTasks(t *testing.T) {
	ws := t.TempDir()
	writeJSON(t, filepath.Join(ws, committedset.DecisionFile), map[string]any{
		"top_n": []map[string]string{{"id": "alpha"}, {"id": "beta"}},
	})
	d := mustBuild(t, 1623, baseOpts(ws))
	if d.Tasks == nil || strings.Join(*d.Tasks, ",") != "alpha,beta" {
		t.Fatalf("Tasks = %v, want the committed top_n ids", d.Tasks)
	}
	empty := t.TempDir()
	writeJSON(t, filepath.Join(empty, committedset.DecisionFile), map[string]any{"top_n": []any{}})
	if d2 := mustBuild(t, 1623, baseOpts(empty)); d2.Tasks == nil || len(*d2.Tasks) != 0 {
		t.Errorf("an explicit EMPTY commitment must be recorded as empty, not absent; got %v", d2.Tasks)
	}
}

func TestBuild_Cycle1623Shape(t *testing.T) {
	ws := t.TempDir()
	writeJSON(t, filepath.Join(ws, ShipBindingFile), ShipBinding{CommitSHA: "dc00395a"})
	writeJSON(t, filepath.Join(ws, committedset.DecisionFile), map[string]any{"top_n": []any{}})
	opts := baseOpts(ws)
	for _, p := range []string{"scout", "triage", "tdd", "build", "audit", "tdd", "build", "audit", "tdd", "build", "audit", "ship"} {
		opts.PhaseTimings = append(opts.PhaseTimings, phasetiming.Entry{Phase: p, Verdict: "PASS", DurationMS: 1})
	}
	d := mustBuild(t, 1623, opts)
	if len(d.Phases) != 12 {
		t.Errorf("phases = %d, want 12 (the dispatches that actually ran)", len(d.Phases))
	}
	if d.CommitSHA != "dc00395a" {
		t.Errorf("the shipped commit must be recorded; got %q", d.CommitSHA)
	}
	if d.Tasks == nil || len(*d.Tasks) != 0 {
		t.Errorf("the empty commitment must be visible as empty; got %v", d.Tasks)
	}
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	body, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestShippedCommit_RoundTripsTheWriterType(t *testing.T) {
	ws := t.TempDir()
	writeJSON(t, filepath.Join(ws, ShipBindingFile), ShipBinding{
		AuditBoundTreeSHA: "3388ca0b",
		TreeSHACommitted:  "3388ca0b",
		CommitSHA:         "dc00395a",
		Cycle:             1623,
	})
	commit, tree, ok := shippedCommit(ws)
	if !ok || commit != "dc00395a" || tree != "3388ca0b" {
		t.Fatalf("shippedCommit = (%q,%q,%v), want the written binding round-tripped", commit, tree, ok)
	}
	if _, _, ok := shippedCommit(t.TempDir()); ok {
		t.Error("absent binding must report not-ok, never a fabricated commit")
	}
	empty := t.TempDir()
	writeJSON(t, filepath.Join(empty, ShipBindingFile), ShipBinding{Cycle: 1623})
	if _, _, ok := shippedCommit(empty); ok {
		t.Error("a binding carrying no commit_sha must report not-ok")
	}
}

func TestCommittedTasks_AbsentEmptyAndPopulated(t *testing.T) {
	if _, ok := committedTasks(t.TempDir()); ok {
		t.Error("absent decision must be not-ok (unknown), never an empty commitment")
	}

	empty := t.TempDir()
	writeJSON(t, filepath.Join(empty, committedset.DecisionFile), map[string]any{"top_n": []any{}})
	ids, ok := committedTasks(empty)
	if !ok || ids == nil || len(ids) != 0 {
		t.Errorf("present-but-empty top_n = (%v,%v), want ok with an empty non-nil slice", ids, ok)
	}

	full := t.TempDir()
	writeJSON(t, filepath.Join(full, committedset.DecisionFile), map[string]any{
		"top_n": []map[string]string{{"id": "alpha"}, {"id": ""}, {"id": "beta"}},
	})
	ids, ok = committedTasks(full)
	if !ok || strings.Join(ids, ",") != "alpha,beta" {
		t.Errorf("committedTasks = (%v,%v), want the non-blank ids in order", ids, ok)
	}

	bad := t.TempDir()
	if err := os.WriteFile(filepath.Join(bad, committedset.DecisionFile), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := committedTasks(bad); ok {
		t.Error("malformed decision must be not-ok, never a fabricated commitment")
	}
}

func TestBuild_TasksSerialization(t *testing.T) {
	marshal := func(ws string) string {
		t.Helper()
		body, err := json.Marshal(mustBuild(t, 1623, baseOpts(ws)))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}

	got := marshal(t.TempDir())
	if strings.Contains(got, `"tasks":null`) {
		t.Errorf("an absent decision must omit tasks, never emit null (the schema types it as an array): %s", got)
	}
	if strings.Contains(got, `"tasks"`) {
		t.Errorf("an absent decision must omit the field entirely: %s", got)
	}

	empty := t.TempDir()
	writeJSON(t, filepath.Join(empty, committedset.DecisionFile), map[string]any{"top_n": []any{}})
	if got := marshal(empty); !strings.Contains(got, `"tasks":[]`) {
		t.Errorf("an explicit empty commitment must serialize as []: %s", got)
	}

	full := t.TempDir()
	writeJSON(t, filepath.Join(full, committedset.DecisionFile), map[string]any{
		"top_n": []map[string]string{{"id": "alpha"}},
	})
	if got := marshal(full); !strings.Contains(got, `"tasks":["alpha"]`) {
		t.Errorf("a committed task must serialize as its id: %s", got)
	}
}

func TestRender_CommitmentIsVisibleToAHumanReader(t *testing.T) {
	ws := t.TempDir()
	writeJSON(t, filepath.Join(ws, committedset.DecisionFile), map[string]any{
		"top_n": []map[string]string{{"id": "alpha"}, {"id": "beta"}},
	})
	raw, err := RenderMarkdown(mustBuild(t, 1623, baseOpts(ws)))
	md := string(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(md, "**Committed:**") || !strings.Contains(md, "alpha") || !strings.Contains(md, "beta") {
		t.Errorf("the rendered record must name the committed tasks:\n%s", md)
	}

	empty := t.TempDir()
	writeJSON(t, filepath.Join(empty, committedset.DecisionFile), map[string]any{"top_n": []any{}})
	raw, err = RenderMarkdown(mustBuild(t, 1623, baseOpts(empty)))
	md = string(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(md, "**Committed:**") || !strings.Contains(md, "nothing") {
		t.Errorf("an empty commitment must state itself:\n%s", md)
	}

	raw, err = RenderMarkdown(mustBuild(t, 1623, baseOpts(t.TempDir())))
	md = string(raw)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(md, "**Committed:**") {
		t.Errorf("an unrecorded commitment must not render as a claim:\n%s", md)
	}
}
