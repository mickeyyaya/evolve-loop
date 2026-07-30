package dossier

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

// These tests cover the failure-identity ingestion (dossier-carries-failure-
// reason): a FAIL Build reads <workspace>/failure-digest.json +
// <workspace>/audit-fail-reason.json — the artifacts core.ensureFailureDigest
// and the coherence floor have already written by dossier time — into
// Dossier.Failure, so the committed record says WHY the cycle failed instead
// of only pointing at gitignored workspace forensics. Ingestion is best-effort
// per artifact: absent/malformed files degrade to a smaller (or nil) block and
// never fail Build.

// writeReasonArtifact writes audit-fail-reason.json carrying reasons.
func writeReasonArtifact(t *testing.T, ws string, reasons []string) {
	t.Helper()
	writeWorkspaceJSON(t, ws, auditFailReasonFile, map[string]any{
		"schema_version": 1, "phase": "audit", "reasons": reasons,
	})
}

// writeWorkspaceJSON marshals v into <ws>/<name>.
func writeWorkspaceJSON(t *testing.T, ws, name string, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal %s: %v", name, err)
	}
	if err := os.WriteFile(filepath.Join(ws, name), b, 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func TestBuild_FailIngestsFailureIdentity(t *testing.T) {
	ws := t.TempDir()
	writeWorkspaceJSON(t, ws, "audit-fail-reason.json", map[string]any{
		"schema_version": 1,
		"phase":          "audit",
		"reasons":        []string{"EGPS floor blocked ship: red_count=1", "predicate failed to compile"},
	})
	writeWorkspaceJSON(t, ws, "failure-digest.json", map[string]any{
		"cycle": 3, "fingerprint": "audit|gate-block|ab12cd34ef56", "pre_class": "gate-block",
	})
	d, err := Build(3, BuildOpts{WorkspacePath: ws, Goal: "g", FinalVerdict: VerdictFail})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if d.Failure == nil {
		t.Fatal("FAIL Build with both artifacts present must ingest a Failure block")
	}
	if d.Failure.Fingerprint != "audit|gate-block|ab12cd34ef56" {
		t.Errorf("Failure.Fingerprint = %q", d.Failure.Fingerprint)
	}
	if d.Failure.PreClass != "gate-block" {
		t.Errorf("Failure.PreClass = %q", d.Failure.PreClass)
	}
	if len(d.Failure.Reasons) != 2 || d.Failure.Reasons[0] != "EGPS floor blocked ship: red_count=1" {
		t.Errorf("Failure.Reasons = %v", d.Failure.Reasons)
	}
	if err := d.Validate(); err != nil {
		t.Errorf("dossier with failure block must stay valid: %v", err)
	}
}

func TestBuild_FailWithoutArtifacts_NilFailure(t *testing.T) {
	d, err := Build(4, BuildOpts{WorkspacePath: t.TempDir(), Goal: "g", FinalVerdict: VerdictFail})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if d.Failure != nil {
		t.Errorf("no artifacts ⇒ Failure must stay nil (never fabricated); got %+v", d.Failure)
	}
}

func TestBuild_PassIgnoresFailureArtifacts(t *testing.T) {
	ws := t.TempDir()
	writeWorkspaceJSON(t, ws, "audit-fail-reason.json", map[string]any{
		"schema_version": 1, "phase": "audit", "reasons": []string{"stale reason from a failed retry"},
	})
	writeWorkspaceJSON(t, ws, "failure-digest.json", map[string]any{
		"cycle": 5, "fingerprint": "audit|verdict-fail|001122334455", "pre_class": "verdict-fail",
	})
	d, err := Build(5, BuildOpts{WorkspacePath: ws, Goal: "g", FinalVerdict: VerdictPass})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if d.Failure != nil {
		t.Errorf("PASS Build must ignore stale failure artifacts; got %+v", d.Failure)
	}
}

// TestBuild_MalformedDigestStillCarriesReasons pins per-artifact degradation:
// a corrupt digest must not discard the perfectly good reasons[], and vice
// versa the block still forms from the digest alone when reasons are absent.
func TestBuild_MalformedDigestStillCarriesReasons(t *testing.T) {
	ws := t.TempDir()
	writeWorkspaceJSON(t, ws, "audit-fail-reason.json", map[string]any{
		"schema_version": 1, "phase": "audit", "reasons": []string{"the one real reason"},
	})
	if err := os.WriteFile(filepath.Join(ws, "failure-digest.json"), []byte("{not-json"), 0o644); err != nil {
		t.Fatalf("write corrupt digest: %v", err)
	}
	d, err := Build(6, BuildOpts{WorkspacePath: ws, Goal: "g", FinalVerdict: VerdictFail})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if d.Failure == nil || len(d.Failure.Reasons) != 1 || d.Failure.Reasons[0] != "the one real reason" {
		t.Fatalf("corrupt digest must not discard reasons; got %+v", d.Failure)
	}
	if d.Failure.Fingerprint != "" {
		t.Errorf("corrupt digest ⇒ no fingerprint (never fabricated); got %q", d.Failure.Fingerprint)
	}
}

// TestBuild_FailureReasonsTruncatedAndCapped bounds the carried evidence at
// the ingestion seam: ≤5 reasons, each ≤200 chars, head kept, blanks dropped.
func TestBuild_FailureReasonsTruncatedAndCapped(t *testing.T) {
	ws := t.TempDir()
	reasons := []string{"  ", ""} // blanks are dropped, not carried
	for i := 0; i < 7; i++ {
		reasons = append(reasons, string(rune('a'+i))+strings.Repeat("x", 300))
	}
	writeWorkspaceJSON(t, ws, "audit-fail-reason.json", map[string]any{
		"schema_version": 1, "phase": "audit", "reasons": reasons,
	})
	d, err := Build(7, BuildOpts{WorkspacePath: ws, Goal: "g", FinalVerdict: VerdictFail})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if d.Failure == nil {
		t.Fatal("reasons present ⇒ Failure block present")
	}
	if len(d.Failure.Reasons) != 5 {
		t.Errorf("got %d reasons, want cap of 5", len(d.Failure.Reasons))
	}
	for i, r := range d.Failure.Reasons {
		if len(r) > 200 {
			t.Errorf("reasons[%d] is %d chars, want <= 200", i, len(r))
		}
		if !strings.HasPrefix(r, string(rune('a'+i))) {
			t.Errorf("reasons[%d] lost its head (or a blank slipped through): %q", i, r[:1])
		}
	}
}

// TestRenderMarkdown_FailureSection pins the human-readable projection: the
// fingerprint, class and each reason appear under a "## Failure" heading.
func TestRenderMarkdown_FailureSection(t *testing.T) {
	d := &Dossier{
		Cycle:        8,
		Goal:         "g",
		FinalVerdict: VerdictFail,
		Phases:       []PhaseRecord{{Name: "audit", Verdict: VerdictFail}},
		Defects:      []Defect{{ID: "audit-fail", Summary: "s"}},
		Carryover:    []Carryover{{ID: "c", Action: "a"}},
		Failure: &FailureRecord{
			Fingerprint: "audit|gate-block|ab12cd34ef56",
			PreClass:    "gate-block",
			Reasons:     []string{"reason one", "reason two"},
		},
	}
	md, err := RenderMarkdown(d)
	if err != nil {
		t.Fatalf("RenderMarkdown: %v", err)
	}
	out := string(md)
	for _, want := range []string{"## Failure", "audit|gate-block|ab12cd34ef56", "gate-block", "- reason one", "- reason two"} {
		if !strings.Contains(out, want) {
			t.Errorf("markdown missing %q; got:\n%s", want, out)
		}
	}
}

// TestFailureReasons_MultibyteTruncationStaysValidUTF8 is the degenerate-impl
// killer for the byte-bound cut (review MEDIUM: every other truncation test
// uses ASCII, so a plain r[:200] would pass them all and commit mojibake into
// knowledge-base/cycles/*.json).
func TestFailureReasons_MultibyteTruncationStaysValidUTF8(t *testing.T) {
	ws := t.TempDir()
	long := strings.Repeat("é", 300) // 600 bytes; the cut lands mid-rune
	writeReasonArtifact(t, ws, []string{long})
	got := failureReasons(ws)
	if len(got) != 1 {
		t.Fatalf("reasons = %v, want one entry", got)
	}
	if !utf8.ValidString(got[0]) {
		t.Fatalf("truncated reason is not valid UTF-8 (%q) — a mid-rune byte cut committed mojibake", got[0])
	}
	if len(got[0]) > maxFailureReasonBytes {
		t.Errorf("len = %d bytes, want <= %d", len(got[0]), maxFailureReasonBytes)
	}
}

// TestFailureReasons_MultilineReasonCollapses — a reason carrying newlines (test
// output excerpts do) must never reach the md bullet renderer intact: a "## "
// line inside a reason becomes a fake heading in the committed record.
func TestFailureReasons_MultilineReasonCollapses(t *testing.T) {
	ws := t.TempDir()
	writeReasonArtifact(t, ws, []string{"EGPS red\n## Phases\n- fake bullet\n"})
	got := failureReasons(ws)
	if len(got) != 1 {
		t.Fatalf("reasons = %v, want one entry", got)
	}
	if strings.ContainsAny(got[0], "\n\r") {
		t.Fatalf("reason still carries newlines: %q", got[0])
	}
	if got[0] != "EGPS red ## Phases - fake bullet" {
		t.Errorf("collapsed reason = %q, want whitespace runs collapsed to single spaces", got[0])
	}
}

// TestFailureRecord_StaleDigestCycleIsRejected — a digest left by a DIFFERENT
// cycle must not become this cycle's committed identity (review MEDIUM: a false
// forensic identity is worse than an absent one).
func TestFailureRecord_StaleDigestCycleIsRejected(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, failureDigestFile),
		[]byte(`{"cycle":41,"fingerprint":"audit|verdict-fail|deadbeef","pre_class":"verdict-fail"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	writeReasonArtifact(t, ws, []string{"this cycle's real reason"})
	rec, ok := failureRecord(ws, 42)
	if !ok {
		t.Fatal("reasons alone must still yield a record")
	}
	if rec.Fingerprint != "" || rec.PreClass != "" {
		t.Errorf("stale digest (cycle 41) adopted by cycle 42: %+v", rec)
	}
	if len(rec.Reasons) != 1 {
		t.Errorf("reasons must survive the digest rejection: %v", rec.Reasons)
	}
	// Matching cycle ⇒ adopted.
	if fresh, _ := failureRecord(ws, 41); fresh == nil || fresh.Fingerprint == "" {
		t.Errorf("a digest whose cycle MATCHES must be adopted: %+v", fresh)
	}
}
