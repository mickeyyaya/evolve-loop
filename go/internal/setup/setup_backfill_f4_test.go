package setup

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelquery"
)

func TestComputeLatest_AFreshestThatResolvesToNothingKeepsTheCurrentModel(t *testing.T) {
	t.Parallel()
	fp := modelquery.FreshnessPolicy{PreferAlias: true, AliasIDs: []string{""}}

	latest, stale, observed := ComputeLatest("5", []string{""}, fp)

	if latest != "5" || stale || !observed {
		t.Fatalf("latest=%q stale=%v observed=%v: an unresolvable freshest must not blank or stale the current model", latest, stale, observed)
	}
	if latest, stale, observed := ComputeLatest("", []string{"gpt-5.5"}, fp); latest != "" || stale || observed {
		t.Errorf("no current model means nothing to compare: %q %v %v", latest, stale, observed)
	}
}

func TestBiasTier_MinAndMaxFallBackToTheBaseWhenTheEnvelopeLeavesThemUnset(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		bias        string
		defaultTier string
		env         Envelope
		want        string
	}{
		{"min without an envelope floor", "min", "deep", Envelope{}, "deep"},
		{"max without an envelope ceiling", "max", "deep", Envelope{}, "deep"},
		{"min with a floor", "min", "deep", Envelope{Min: "fast"}, "fast"},
		{"max with a ceiling", "max", "fast", Envelope{Max: "top"}, "top"},
		{"unknown bias keeps the base", "sideways", "balanced", Envelope{}, "balanced"},
		{"down never leaves the lowest tier", "down", "fast", Envelope{}, "fast"},
		{"up never leaves the highest tier", "up", "top", Envelope{}, "top"},
		{"base falls back to the envelope default", "min", "", Envelope{Default: "deep"}, "deep"},
		{"base falls back to balanced", "max", "", Envelope{}, "balanced"},
	}
	for _, tc := range cases {
		if got := biasTier(tc.bias, tc.defaultTier, tc.env); got != tc.want {
			t.Errorf("%s: biasTier(%q,%q,%+v) = %q, want %q", tc.name, tc.bias, tc.defaultTier, tc.env, got, tc.want)
		}
	}
}

func TestChooseCrossFamilyPair_MovesWhicheverSideStillHasAnAlternative(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		b, a    PhaseStatus
		avail   []string
		crossOK bool
		wantB   string
		wantA   string
	}{
		{"auditor moves when it has an alternative", PhaseStatus{DefaultCLI: "claude"}, PhaseStatus{DefaultCLI: "claude"}, []string{"claude", "codex"}, true, "claude", "codex"},
		{"builder moves when only it has an alternative", PhaseStatus{DefaultCLI: "claude"}, PhaseStatus{DefaultCLI: "claude", AllowedCLIs: []string{"claude"}}, []string{"claude", "codex"}, true, "codex", "claude"},
		{"neither side can move", PhaseStatus{DefaultCLI: "claude", AllowedCLIs: []string{"claude"}}, PhaseStatus{DefaultCLI: "claude", AllowedCLIs: []string{"claude"}}, []string{"claude", "codex"}, true, "claude", "claude"},
		{"cross-family not required", PhaseStatus{DefaultCLI: "claude"}, PhaseStatus{DefaultCLI: "claude"}, []string{"claude", "codex"}, false, "claude", "claude"},
		{"nothing available", PhaseStatus{DefaultCLI: "claude"}, PhaseStatus{DefaultCLI: "claude"}, nil, true, "", ""},
	}
	for _, tc := range cases {
		b, a := chooseCrossFamilyPair(tc.b, tc.a, tc.avail, tc.crossOK)
		if b != tc.wantB || a != tc.wantA {
			t.Errorf("%s: got (%q,%q), want (%q,%q)", tc.name, b, a, tc.wantB, tc.wantA)
		}
	}
}

func TestComplete_EachWriteStageFailureIsNamedAndLeavesNoStateFile(t *testing.T) {
	now := func() time.Time { return time.Date(2026, 10, 1, 2, 3, 4, 0, time.UTC) }

	t.Run("marshal fails", func(t *testing.T) {
		orig := stateMarshalIndent
		t.Cleanup(func() { stateMarshalIndent = orig })
		boom := errors.New("encoder down")
		stateMarshalIndent = func(any, string, string) ([]byte, error) { return nil, boom }
		dir := t.TempDir()
		stamp, err := Complete(CompleteOptions{EvolveDir: dir, Now: now})
		if !errors.Is(err, boom) || !strings.Contains(err.Error(), "setup complete: marshal") || stamp != "" {
			t.Fatalf("stamp=%q err=%v", stamp, err)
		}
		if _, statErr := os.Stat(filepath.Join(dir, "state.json")); !os.IsNotExist(statErr) {
			t.Errorf("no state file may be written: %v", statErr)
		}
	})
	t.Run("temp file cannot be written", func(t *testing.T) {
		dir := t.TempDir()
		tmp := filepath.Join(dir, "state.json.tmp."+strconv.Itoa(os.Getpid()))
		if err := os.Mkdir(tmp, 0o755); err != nil {
			t.Fatal(err)
		}
		stamp, err := Complete(CompleteOptions{EvolveDir: dir, Now: now})
		if err == nil || !strings.Contains(err.Error(), "setup complete: write temp") || stamp != "" {
			t.Fatalf("stamp=%q err=%v", stamp, err)
		}
	})
	t.Run("state path is an occupied directory so the rename fails", func(t *testing.T) {
		dir := t.TempDir()
		state := filepath.Join(dir, "state.json")
		if err := os.MkdirAll(filepath.Join(state, "inner"), 0o755); err != nil {
			t.Fatal(err)
		}
		stamp, err := Complete(CompleteOptions{EvolveDir: dir, Now: now})
		if err == nil || !strings.Contains(err.Error(), "setup complete: atomic rename") || stamp != "" {
			t.Fatalf("stamp=%q err=%v", stamp, err)
		}
		if left, _ := filepath.Glob(filepath.Join(dir, "state.json.tmp.*")); len(left) != 0 {
			t.Errorf("the temp file must be cleaned up: %v", left)
		}
	})
}
