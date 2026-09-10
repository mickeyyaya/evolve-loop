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

// evidence_projection_test.go — cycle-1623 forensics: a cycle that ran 12
// phases and shipped 922 lines to origin/main recorded a dossier containing
// ONE synthetic phase ("cycle-recorded", PASS, zero tokens) and no commit.
//
// Root cause: phase-timing.json is written by a DEFERRED call in RunCycle, so
// it lands AFTER writeCycleDossier has already read it. On the normal path the
// dossier therefore always missed its own evidence; only a resumed cycle (which
// writes the log mid-run) recorded real phases. The healthier the cycle, the
// emptier its permanent record — and a synthesized PASS made the gap invisible.
//
// The contract these tests pin: a dossier is a PROJECTION OF EVIDENCE. Live
// evidence is authoritative, the on-disk log is the fallback, and absent
// evidence is recorded LOUDLY as a degraded record — never synthesized as a
// passing phase.

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

// TestBuild_LivePhaseTimingsAreAuthoritative: the in-memory timings the
// orchestrator already holds are passed straight in, so the dossier cannot
// depend on whether the deferred file write has landed yet. This is the
// ordering defect's structural fix: remove the dependency, don't re-time it.
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

// TestBuild_AbsentEvidenceIsLoudNeverASynthesizedPass: with no live timings and
// no log on disk, the record must SAY the evidence is missing. The old
// behavior — one "cycle-recorded" phase carrying the cycle's PASS — made a
// twelve-phase cycle indistinguishable from a one-phase cycle and fabricated
// per-phase evidence that no phase produced.
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
	// The cycle's own verdict is untouched — only the phase record degrades.
	if d.FinalVerdict != "PASS" {
		t.Errorf("FinalVerdict must stay the cycle's real outcome; got %q", d.FinalVerdict)
	}
}

// TestBuild_ProjectsTheShippedCommit: Dossier.CommitSHA existed in the schema
// but no producer ever set it, so every dossier omitted the one fact that
// proves delivery. ship-binding.json is the ship phase's own proof; the
// dossier reads it the way it already reads ci-watch-verdict.json.
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
	// No binding ⇒ no commit claimed. Never fabricated.
	if d2 := mustBuild(t, 1623, baseOpts(t.TempDir())); d2.CommitSHA != "" {
		t.Errorf("absent ship-binding must leave CommitSHA empty; got %q", d2.CommitSHA)
	}
}

// TestBuild_ProjectsTheCommittedTasks: "what was this cycle for" belongs in the
// permanent record. triage-decision.json's top_n is the committed set; an
// EMPTY commitment is itself the finding cycle-1623 needed to surface, so it is
// recorded as an explicit empty list rather than an absent field.
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

// TestBuild_Cycle1623Shape is the forensic regression: the exact evidence
// cycle-1623 left on disk must produce a record an operator (or a ship-rate
// query) can read correctly — twelve phases, the shipped commit, and an empty
// commitment, all visible at once.
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

// TestShippedCommit_RoundTripsTheWriterType marshals the SHARED ShipBinding
// type — the one phases/ship now writes — and reads it back. The previous
// version of this test hand-typed the key names, so a writer rename would have
// left it green while every dossier's delivery record silently emptied. Now the
// coupling is the type: a rename is a compile error at the write site.
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
	// A binding with no commit is not a delivery.
	empty := t.TempDir()
	writeJSON(t, filepath.Join(empty, ShipBindingFile), ShipBinding{Cycle: 1623})
	if _, _, ok := shippedCommit(empty); ok {
		t.Error("a binding carrying no commit_sha must report not-ok")
	}
}

// TestCommittedTasks_AbsentEmptyAndPopulated pins the three-way contract the
// dossier depends on, using the artifact constant the reader resolves
// (committedset.DecisionFile) so a filename change fails here rather than silently
// emptying every record's commitment. Absent ⇒ not-ok (unknown); present but
// empty ⇒ ok with an empty non-nil slice (the cycle committed to nothing —
// cycle-1623's finding); populated ⇒ the ids in order.
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

// TestBuild_TasksSerialization asserts on the MARSHALED BYTES, not the Go
// value, because the defect this pins was invisible at the Go level: a plain
// []string field left nil by an absent triage decision marshals to
// `"tasks": null`, which the schema's `"type": "array"` rejects — a record
// that fails its own contract. The three states must be distinguishable on
// the wire: omitted (unknown), [] (explicit empty commitment), and the ids.
func TestBuild_TasksSerialization(t *testing.T) {
	marshal := func(ws string) string {
		t.Helper()
		body, err := json.Marshal(mustBuild(t, 1623, baseOpts(ws)))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}

	// 1. No triage decision at all ⇒ the field is OMITTED, never null.
	got := marshal(t.TempDir())
	if strings.Contains(got, `"tasks":null`) {
		t.Errorf("an absent decision must omit tasks, never emit null (the schema types it as an array): %s", got)
	}
	if strings.Contains(got, `"tasks"`) {
		t.Errorf("an absent decision must omit the field entirely: %s", got)
	}

	// 2. Present-but-empty commitment ⇒ [] on the wire, never omitted.
	empty := t.TempDir()
	writeJSON(t, filepath.Join(empty, committedset.DecisionFile), map[string]any{"top_n": []any{}})
	if got := marshal(empty); !strings.Contains(got, `"tasks":[]`) {
		t.Errorf("an explicit empty commitment must serialize as []: %s", got)
	}

	// 3. Populated commitment ⇒ the ids.
	full := t.TempDir()
	writeJSON(t, filepath.Join(full, committedset.DecisionFile), map[string]any{
		"top_n": []map[string]string{{"id": "alpha"}},
	})
	if got := marshal(full); !strings.Contains(got, `"tasks":["alpha"]`) {
		t.Errorf("a committed task must serialize as its id: %s", got)
	}
}

// TestRender_CommitmentIsVisibleToAHumanReader: the JSON half is what queries
// read; knowledge-base/cycles/cycle-N.md is what people read, and the personas
// point them there. A commitment recorded only in JSON leaves "what was this
// cycle for" unanswerable in the half that gets read.
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

	// An explicit EMPTY commitment states itself rather than rendering blank.
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

	// No decision at all ⇒ the line is absent (unknown, not "nothing").
	raw, err = RenderMarkdown(mustBuild(t, 1623, baseOpts(t.TempDir())))
	md = string(raw)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(md, "**Committed:**") {
		t.Errorf("an unrecorded commitment must not render as a claim:\n%s", md)
	}
}
