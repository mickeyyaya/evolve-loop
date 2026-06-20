// reviewtrailer_test.go — reviewedByTrailer (commitgate.go).
//
// The trailer makes "reviewed before commit, by whom" a durable property of
// the commit SHA. It must fire ONLY for --class manual with a valid
// attestation, and stay empty otherwise (non-manual, missing, malformed,
// empty reviewers) — so trailer-present == reviewed.

package ship

import (
	"path/filepath"
	"testing"
)

func TestReviewedByTrailer(t *testing.T) {
	repo := t.TempDir()
	writeAttestation(t, repo, "deadbeef") // reviewers_run: simplifier, reviewer, go-reviewer

	t.Run("manual + valid attestation → one trailer line per reviewer", func(t *testing.T) {
		got := reviewedByTrailer(&Options{Class: ClassManual, ProjectRoot: repo})
		want := "\n\nReviewed-by: code-simplifier\nReviewed-by: code-reviewer\nReviewed-by: go-reviewer"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("non-manual class → empty (no trailer)", func(t *testing.T) {
		if g := reviewedByTrailer(&Options{Class: ClassCycle, ProjectRoot: repo}); g != "" {
			t.Errorf("cycle class got %q, want empty", g)
		}
	})

	// The trust-critical case: a bypass means review was SKIPPED, so a stale
	// on-disk attestation must NOT produce a trailer (else the commit falsely
	// asserts it was reviewed).
	t.Run("bypass + valid attestation → empty (not reviewed)", func(t *testing.T) {
		opts := &Options{Class: ClassManual, ProjectRoot: repo, BypassCommitGate: true}
		if g := reviewedByTrailer(opts); g != "" {
			t.Errorf("bypassed commit got trailer %q, want empty", g)
		}
	})

	t.Run("missing attestation → empty", func(t *testing.T) {
		if g := reviewedByTrailer(&Options{Class: ClassManual, ProjectRoot: t.TempDir()}); g != "" {
			t.Errorf("missing attestation got %q, want empty", g)
		}
	})

	t.Run("malformed attestation → empty", func(t *testing.T) {
		dir := t.TempDir()
		mustMkdir(t, filepath.Join(dir, ".commit-gate"))
		mustWrite(t, filepath.Join(dir, ".commit-gate", "attestation.json"), "{not json")
		if g := reviewedByTrailer(&Options{Class: ClassManual, ProjectRoot: dir}); g != "" {
			t.Errorf("malformed got %q, want empty", g)
		}
	})

	t.Run("empty reviewers_run → empty", func(t *testing.T) {
		dir := t.TempDir()
		mustMkdir(t, filepath.Join(dir, ".commit-gate"))
		mustWrite(t, filepath.Join(dir, ".commit-gate", "attestation.json"), `{"tree_state_sha":"x","reviewers_run":[]}`)
		if g := reviewedByTrailer(&Options{Class: ClassManual, ProjectRoot: dir}); g != "" {
			t.Errorf("empty reviewers got %q, want empty", g)
		}
	})
}
