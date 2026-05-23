// Package cycle95 ports the cycle-95 ACS predicates (2 bash files, ~10 ACs total).
//
// The bash predicates exercise the live `subagent-run.sh --resolve-tier`
// subprocess against synthetic .evolve/state.json fixtures (mastery gate
// behavior). The Go ports are SOURCE-PRESENCE regression guards: they
// verify the relevant code paths still exist in subagent-run.sh and the
// auditor.json profile, without re-running the full fixture battery.
//
// The bash predicates remain authoritative for runtime behavior;
// substitute with `bash acs/regression-suite/cycle-95/pred-*.sh` to
// re-execute against the live binary.
package cycle95

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// TestC95_AuditorMasteryGate ports pred-auditor-mastery-gate.sh.
// Verifies subagent-run.sh + auditor profile carry the mastery-gate
// resolution markers (--resolve-tier flag, mastery.consecutiveSuccesses
// branch, MODEL_TIER_HINT override).
func TestC95_AuditorMasteryGate(t *testing.T) {
	root := acsassert.RepoRoot(t)
	subagent := filepath.Join(root, "legacy", "scripts", "dispatch", "subagent-run.sh")
	auditor := filepath.Join(root, ".evolve", "profiles", "auditor.json")

	if !acsassert.FileExists(t, subagent) {
		t.Skip("subagent-run.sh missing — skip cycle-95 mastery-gate")
	}
	for _, marker := range []string{
		"--resolve-tier",
		"consecutiveSuccesses",
		"MODEL_TIER_HINT",
	} {
		if !acsassert.FileContains(t, subagent, marker) {
			return
		}
	}
	if !acsassert.FileExists(t, auditor) {
		t.Errorf("auditor profile missing: %s", auditor)
	}
}

// TestC95_FastFailCounterDocs ports pred-fastfail-counter-docs.sh.
// Verifies subagent-run.sh has the .fast-fail-counter declaration and a
// documentation block referencing per-workspace scoping + state.json
// fastFailCounters intentional-non-use.
func TestC95_FastFailCounterDocs(t *testing.T) {
	root := acsassert.RepoRoot(t)
	subagent := filepath.Join(root, "legacy", "scripts", "dispatch", "subagent-run.sh")

	if !acsassert.FileExists(t, subagent) {
		t.Skip("subagent-run.sh missing — skip cycle-95 fastfail-docs")
	}
	if !acsassert.FileContains(t, subagent, ".fast-fail-counter") {
		return
	}
	// At least one of the three documentation signals must appear in the file.
	if !acsassert.FileContainsAny(subagent,
		"per-workspace", "workspace-scoped", "per-cycle",
		"fastFailCounters", "structural dispatch", "single-cycle invocation",
	) {
		t.Errorf("subagent-run.sh: no per-workspace/per-cycle/structural-dispatch documentation found near fast-fail-counter")
	}
}
