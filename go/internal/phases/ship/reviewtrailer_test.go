package ship

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/commitgate"
)

func TestReviewTrailer(t *testing.T) {
	repo := t.TempDir()
	writeAttestation(t, repo, "deadbeef") // reviewers_run: simplifier, reviewer, go-reviewer

	t.Run("manual + valid attestation → one trailer line per reviewer", func(t *testing.T) {
		got := reviewTrailer(&Options{Class: ClassManual, ProjectRoot: repo})
		want := "\n\nReviewed-by: code-simplifier\nReviewed-by: code-reviewer\nReviewed-by: go-reviewer"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("non-manual class → empty (no trailer)", func(t *testing.T) {
		if g := reviewTrailer(&Options{Class: ClassCycle, ProjectRoot: repo}); g != "" {
			t.Errorf("cycle class got %q, want empty", g)
		}
	})

	t.Run("bypass + valid attestation → empty (not reviewed)", func(t *testing.T) {
		opts := &Options{Class: ClassManual, ProjectRoot: repo, BypassCommitGate: true}
		if g := reviewTrailer(opts); g != "" {
			t.Errorf("bypassed commit got trailer %q, want empty", g)
		}
	})

	t.Run("missing attestation → empty", func(t *testing.T) {
		if g := reviewTrailer(&Options{Class: ClassManual, ProjectRoot: t.TempDir()}); g != "" {
			t.Errorf("missing attestation got %q, want empty", g)
		}
	})

	t.Run("malformed attestation → empty", func(t *testing.T) {
		dir := t.TempDir()
		mustMkdir(t, filepath.Join(dir, ".commit-gate"))
		mustWrite(t, filepath.Join(dir, ".commit-gate", "attestation.json"), "{not json")
		if g := reviewTrailer(&Options{Class: ClassManual, ProjectRoot: dir}); g != "" {
			t.Errorf("malformed got %q, want empty", g)
		}
	})

	t.Run("a waived attestation names its waiver", func(t *testing.T) {
		dir := t.TempDir()
		writeGateAttestation(t, dir, commitgate.Attestation{TreeStateSHA: "x", ReviewWaiver: "comment-only"})
		if g := reviewTrailer(&Options{Class: ClassManual, ProjectRoot: dir}); g != "\n\nReview-waived: comment-only" {
			t.Errorf("waived got %q, want a Review-waived trailer so git log tells it from a bypass", g)
		}
		if g := reviewTrailer(&Options{Class: ClassManual, ProjectRoot: dir, BypassCommitGate: true}); g != "" {
			t.Errorf("a bypassed commit got %q, want no trailer even beside a waived attestation", g)
		}
	})

	t.Run("a waiver spanning lines is dropped", func(t *testing.T) {
		dir := t.TempDir()
		writeGateAttestation(t, dir, commitgate.Attestation{TreeStateSHA: "x", ReviewWaiver: "comment-only\nSigned-off-by: someone"})
		if g := reviewTrailer(&Options{Class: ClassManual, ProjectRoot: dir}); g != "" {
			t.Errorf("got %q, want no trailer: a corrupt attestation cannot inject trailer lines", g)
		}
	})

	t.Run("empty reviewers_run → empty", func(t *testing.T) {
		dir := t.TempDir()
		mustMkdir(t, filepath.Join(dir, ".commit-gate"))
		mustWrite(t, filepath.Join(dir, ".commit-gate", "attestation.json"), `{"tree_state_sha":"x","reviewers_run":[]}`)
		if g := reviewTrailer(&Options{Class: ClassManual, ProjectRoot: dir}); g != "" {
			t.Errorf("empty reviewers got %q, want empty", g)
		}
	})
}
