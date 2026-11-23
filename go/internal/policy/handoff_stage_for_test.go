package policy_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func writeStagePolicy(t *testing.T, text string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if text != "" {
		if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestHandoffStageFor_ReadsThePairAndDefaultsToShadow(t *testing.T) {
	t.Parallel()
	root := writeStagePolicy(t, `{"handoff": {"build-audit": {"stage": "enforce"}}}`)
	if got, err := policy.HandoffStageFor(root, "build-audit"); err != nil || got != "enforce" {
		t.Errorf("build-audit = %q, %v; want enforce", got, err)
	}
	if got, err := policy.HandoffStageFor(root, "tdd-build"); err != nil || got != "shadow" {
		t.Errorf("tdd-build = %q, %v; want shadow", got, err)
	}
	if got, err := policy.HandoffStageFor(root, "no-such-pair"); err != nil || got != "shadow" {
		t.Errorf("unknown pair = %q, %v; want shadow", got, err)
	}
}

func TestHandoffStageFor_ACorruptFileReturnsShadowAndTheError(t *testing.T) {
	t.Parallel()
	root := writeStagePolicy(t, `{not json`)
	got, err := policy.HandoffStageFor(root, "build-audit")
	if err == nil || got != "shadow" {
		t.Errorf("got %q, %v; want shadow and an error", got, err)
	}
}
