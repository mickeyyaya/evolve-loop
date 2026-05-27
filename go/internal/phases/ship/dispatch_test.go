// dispatch_test.go — covers the PhaseRunner dispatcher's NATIVE branch
// (runNative) and the useNativeShip env matrix. ship_test.go exercises only
// the legacy shell-out path (EVOLVE_NATIVE_SHIP=0); the default native path
// through Phase.Run → runNative was previously 0% covered.
package ship

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestUseNativeShip_EnvMatrix(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{"explicit-0-legacy", map[string]string{"EVOLVE_NATIVE_SHIP": "0"}, false},
		{"explicit-1-native", map[string]string{"EVOLVE_NATIVE_SHIP": "1"}, true},
		{"other-value-native", map[string]string{"EVOLVE_NATIVE_SHIP": "yes"}, true},
		{"unset-defaults-native", map[string]string{}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Hermetic: the unset case consults os.Getenv, so pin it empty.
			t.Setenv("EVOLVE_NATIVE_SHIP", "")
			if v, ok := tc.env["EVOLVE_NATIVE_SHIP"]; ok {
				t.Setenv("EVOLVE_NATIVE_SHIP", v)
			}
			if got := useNativeShip(tc.env); got != tc.want {
				t.Errorf("useNativeShip(%v)=%v, want %v", tc.env, got, tc.want)
			}
		})
	}
}

// TestPhaseRun_NativeDispatch_Ships drives Phase.Run through the native Go
// implementation end-to-end against a real repo and asserts the
// PhaseResponse translation (VerdictPASS, NextPhase=retro).
func TestPhaseRun_NativeDispatch_Ships(t *testing.T) {
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\nnative dispatch\n")
	seedAudit(t, repo, "PASS")
	addRemote(t, repo)

	p := New(Config{Runner: execRunner})
	resp, err := p.Run(context.Background(), core.PhaseRequest{
		Cycle:       1,
		ProjectRoot: repo,
		Workspace:   filepath.Join(repo, ".evolve", "runs", "cycle-1"),
		Context:     map[string]string{"commit_message": "feat: native dispatch ship"},
		Env:         map[string]string{"EVOLVE_NATIVE_SHIP": "1", "EVOLVE_PLUGIN_ROOT": repo},
	})
	if err != nil {
		t.Fatalf("native dispatch errored: %v (diags=%v)", err, resp.Diagnostics)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Fatalf("want VerdictPASS, got %q (diags=%v)", resp.Verdict, resp.Diagnostics)
	}
	if resp.NextPhase != string(core.PhaseRetro) {
		t.Errorf("want NextPhase=retro, got %q", resp.NextPhase)
	}
}

// TestPhaseRun_NativeDispatch_NoAuditor_FailVerdict drives the native path
// into an integrity refusal (no Auditor entry) and asserts it surfaces as
// a FAIL verdict plus a non-nil error.
func TestPhaseRun_NativeDispatch_NoAuditor_FailVerdict(t *testing.T) {
	repo := makeRepo(t) // no seedAudit → audit binding fails

	p := New(Config{Runner: execRunner})
	resp, err := p.Run(context.Background(), core.PhaseRequest{
		Cycle:       1,
		ProjectRoot: repo,
		Workspace:   filepath.Join(repo, ".evolve", "runs", "cycle-1"),
		Context:     map[string]string{"commit_message": "feat: no auditor"},
		Env:         map[string]string{"EVOLVE_NATIVE_SHIP": "1", "EVOLVE_PLUGIN_ROOT": repo},
	})
	if err == nil {
		t.Fatal("want error from native dispatch with no auditor entry")
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("want VerdictFAIL, got %q", resp.Verdict)
	}
	if len(resp.Diagnostics) == 0 {
		t.Error("want diagnostics explaining the refusal")
	}
}
