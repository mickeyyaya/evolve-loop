package recovery

// promote_test.go — ADR-0044 Slice 5 RED tests: the Reflexion-style promotion
// loop. When the LLM failure-advisor classifies a NOVEL fatal pane state, its
// classification is promoted into the deterministic registry — in-memory (the
// same batch's later phases get the fast catch) and durably
// (.evolve/instincts/fatal-signatures/<id>.yaml, replayed at startup) — so
// the deterministic frontier grows and the LLM never re-pays for a known
// failure. Judgment at the frontier, determinism in the core.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPromote_InMemory_DetectsImmediately(t *testing.T) {
	t.Parallel()
	d := SeedDetector()
	pane := "⚠ session credential vault locked; interactive unlock required"
	if _, _, ok := d.Detect(pane); ok {
		t.Fatal("fixture must be novel before promotion")
	}
	d.Promote(FatalSignature{Substr: "credential vault locked", Cause: CauseDeadShell, Note: "test"})
	cause, sig, ok := d.Detect(pane)
	if !ok || cause != CauseDeadShell || sig != "credential vault locked" {
		t.Fatalf("promoted signature must catch immediately; got cause=%v sig=%q ok=%v", cause, sig, ok)
	}
}

func TestPromote_SeedsKeepPrecedence(t *testing.T) {
	t.Parallel()
	d := SeedDetector()
	// A promoted signature that ALSO matches a seeded pane must not shadow
	// the vetted seed: promotions append after seeds (first match wins).
	d.Promote(FatalSignature{Substr: "issue with the selected model", Cause: CauseDeadShell, Note: "malicious shadow attempt"})
	cause, _, ok := d.Detect("⏺ There's an issue with the selected model (auto).")
	if !ok || cause != CauseModelInvalid {
		t.Fatalf("seeded signature must keep precedence over promotions; got %v", cause)
	}
}

func TestPromoteSignature_DurableAbsentOnly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	sig := FatalSignature{Substr: "credential vault locked", Cause: CauseDeadShell, Note: "novel, advisor-classified"}
	id1, err := PromoteSignature(dir, sig)
	if err != nil {
		t.Fatalf("PromoteSignature: %v", err)
	}
	if id1 == "" {
		t.Fatal("promotion must return a stable id")
	}
	// Second promotion of the same substring is idempotent: same id, no
	// clobber (absent-only — an operator-edited file wins).
	id2, err := PromoteSignature(dir, FatalSignature{Substr: sig.Substr, Cause: CauseModelInvalid, Note: "different note"})
	if err != nil {
		t.Fatalf("re-promote: %v", err)
	}
	if id1 != id2 {
		t.Errorf("promotion id must be deterministic per substring; %q vs %q", id1, id2)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("want exactly 1 promotion file, got %d (err=%v)", len(entries), err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, entries[0].Name()))
	if !strings.Contains(string(data), "dead_shell") {
		t.Errorf("first write wins (absent-only); file content: %s", data)
	}
	if !strings.Contains(string(data), "confidence: 0.5") {
		t.Errorf("promoted signatures carry confidence 0.5 (below LLM-authored lessons ≥0.9); got: %s", data)
	}
}

func TestSeedDetectorWithPromotions_Replays(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if _, err := PromoteSignature(dir, FatalSignature{Substr: "credential vault locked", Cause: CauseCLISelfUpdated, Note: "n"}); err != nil {
		t.Fatal(err)
	}
	d := SeedDetectorWithPromotions(dir)
	// Seeds still present…
	if cause, _, ok := d.Detect("zsh: command not found: x"); !ok || cause != CauseDeadShell {
		t.Fatalf("seeds must survive replay; got %v ok=%v", cause, ok)
	}
	// …and the durable promotion is caught deterministically (zero advisor
	// calls — the Slice-5 acceptance).
	cause, _, ok := d.Detect("⚠ credential vault locked")
	if !ok || cause != CauseCLISelfUpdated {
		t.Fatalf("replayed promotion must catch; got %v ok=%v", cause, ok)
	}
}

func TestSeedDetectorWithPromotions_MissingOrCorruptDirSafe(t *testing.T) {
	t.Parallel()
	// Absent dir → seeds only, no error.
	d := SeedDetectorWithPromotions(filepath.Join(t.TempDir(), "nope"))
	if _, _, ok := d.Detect("zsh: command not found: x"); !ok {
		t.Fatal("absent promotions dir must degrade to seeds")
	}
	// Corrupt file → skipped, seeds + valid files still load (a bad
	// promotion must never brick boot).
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "corrupt.yaml"), []byte(":::not yaml:::"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := PromoteSignature(dir, FatalSignature{Substr: "credential vault locked", Cause: CauseDeadShell}); err != nil {
		t.Fatal(err)
	}
	d2 := SeedDetectorWithPromotions(dir)
	if _, _, ok := d2.Detect("credential vault locked"); !ok {
		t.Fatal("valid promotion must load despite a corrupt sibling")
	}
}

func TestPromoteAdvice_ValidatesBeforePromoting(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	d := SeedDetector()
	cases := []struct {
		name   string
		advice FailureAdvice
	}{
		{"unknown_cause", FailureAdvice{Cause: "totally_new_cause", PaneSubstr: "long enough substring here", Justification: "j"}},
		{"short_substr", FailureAdvice{Cause: string(CauseDeadShell), PaneSubstr: "tiny", Justification: "j"}},
		{"empty_substr", FailureAdvice{Cause: string(CauseDeadShell), Justification: "j"}},
	}
	for _, tc := range cases {
		if err := PromoteAdvice(d, dir, tc.advice); err == nil {
			t.Errorf("%s: invalid advice must be rejected (escalate, never promote garbage into the hot loop)", tc.name)
		}
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("rejected advice must write nothing; found %d files", len(entries))
	}
	ok := FailureAdvice{Cause: string(CauseModelInvalid), PaneSubstr: "provider returned model_not_available", Justification: "boot error variant"}
	if err := PromoteAdvice(d, dir, ok); err != nil {
		t.Fatalf("valid advice must promote: %v", err)
	}
	if cause, _, hit := d.Detect("xx provider returned model_not_available xx"); !hit || cause != CauseModelInvalid {
		t.Fatal("PromoteAdvice must promote in-memory")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Fatal("PromoteAdvice must promote durably")
	}
}
