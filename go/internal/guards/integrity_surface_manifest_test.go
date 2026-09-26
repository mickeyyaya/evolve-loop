package guards

import (
	"strings"
	"testing"
)

// preRefactorFragments is the original perimeter; each must stay in the manifest byte-for-byte.
var preRefactorFragments = []string{
	"/go/acs/regression/",
	"/go/internal/acssuite/",
	"/go/internal/guards/",
	"/go/internal/flagregistry/registry_table.go",
	"/go/internal/flagregistry/registry_ceiling_test.go",
	"/knowledge-base/research/flag-campaign-plan.json",
	"/skills/audit/",
	"/skills/adversarial-testing/",
	"/.claude/settings.json",
	"/.evolve/policy.json",
}

func TestProtectedSurfaceManifest_KeepsEveryPreRefactorFragment(t *testing.T) {
	var manifest []ProtectedSurfaceEntry = ProtectedSurfaceManifest
	got := make(map[string]bool, len(manifest))
	for _, e := range manifest {
		got[e.Fragment] = true
	}
	for _, frag := range preRefactorFragments {
		if !got[frag] {
			t.Errorf("pre-refactor fragment %q missing from ProtectedSurfaceManifest — "+
				"the SSOT refactor must be behavior-preserving (removals/edits of "+
				"pre-L4 entries are a regression)", frag)
		}
	}
}

func TestProtectedSurfaceManifest_EveryEntryDenies(t *testing.T) {
	if len(ProtectedSurfaceManifest) == 0 {
		t.Fatal("ProtectedSurfaceManifest is empty — the control plane has no boundary")
	}
	for _, e := range ProtectedSurfaceManifest {
		t.Run(e.Fragment, func(t *testing.T) {
			if e.Fragment != strings.ToLower(e.Fragment) {
				t.Errorf("fragment %q is not lower-case; IsProtectedSurface folds the PATH "+
					"to lower, so an upper-case fragment can never match", e.Fragment)
			}
			if strings.Contains(e.Fragment, "\\") {
				t.Errorf("fragment %q contains a backslash; fragments are slash-normalized", e.Fragment)
			}
			if !strings.HasPrefix(e.Fragment, "/") {
				t.Errorf("fragment %q lacks a leading slash; anchored dir/file fragments "+
					"start with / so 'go/acs' cannot match 'lego/acs'", e.Fragment)
			}
			if e.Rationale == "" {
				t.Errorf("fragment %q has no Rationale; every manifest entry must document "+
					"why its surface grades/gates a cycle", e.Fragment)
			}
			// A directory entry is sampled with a file under it.
			sample := e.Fragment
			if strings.HasSuffix(sample, "/") {
				sample += "x.go"
			}
			abs := "/users/x/.evolve/worktrees/cycle-9" + sample
			if !IsProtectedSurface(abs) {
				t.Errorf("IsProtectedSurface(%q) = false, want true (worktree-absolute form "+
					"of manifest entry %q must deny)", abs, e.Fragment)
			}
			rel := strings.TrimPrefix(sample, "/")
			if !IsProtectedSurface(rel) {
				t.Errorf("IsProtectedSurface(%q) = false, want true (repo-relative form of "+
					"manifest entry %q must deny)", rel, e.Fragment)
			}
		})
	}
}

func TestProtectedSurfaceManifest_NonProtectedPathsStillAllow(t *testing.T) {
	allowed := []string{
		// Neighbors of protected files in core, phases/ship, go/acs and flagregistry stay writable.
		"/wt/go/internal/core/observer.go",
		"/wt/go/internal/phases/ship/output.go",
		"/wt/go/acs/cycle21/predicates_test.go",
		"/wt/go/internal/flagregistry/registry.go",
		"/wt/README.md",
	}
	for _, p := range allowed {
		if IsProtectedSurface(p) {
			t.Errorf("IsProtectedSurface(%q) = true, want false (the manifest must stay "+
				"NARROW; over-blocking breaks legitimate cycle writes)", p)
		}
	}
}

func TestProtectedSurfaceManifest_CoversExplanationTrustBoundary(t *testing.T) {
	for _, path := range []string{
		"go/internal/explanationdocs/explanationdocs.go",
		"go/internal/core/build_explanation_handoff.go",
		"go/internal/core/build_floor_reviewer.go",
		"go/internal/core/reviewer.go",
		"go/internal/core/orchestrator.go",
		"go/internal/core/cyclerun.go",
		"go/internal/core/cyclerun_dispatch.go",
		"go/internal/core/cyclerun_review.go",
		"go/internal/core/cyclerun_remediate.go",
		"go/internal/core/continuation_stamp.go",
		"go/internal/core/evaluate_batch.go",
		"go/internal/core/failure_learning.go",
		"go/internal/core/ship_recovery.go",
		"go/internal/core/resume.go",
		"go/internal/core/ports.go",
		"go/internal/core/phase.go",
		"go/internal/cyclestate/state.go",
		"go/internal/phaseio/handoffs.go",
		"go/internal/reportdoc/reportdoc.go",
		"go/internal/phases/runner/runner.go",
		"go/internal/bridge/sandbox_wrap.go",
		"go/internal/bridge/engine.go",
		"go/internal/bridge/launch.go",
		"go/internal/bridge/driver_agy.go",
		"go/internal/bridge/driver_claudep.go",
		"go/internal/bridge/driver_codex.go",
		"go/internal/bridge/driver_tmux_repl.go",
		"go/internal/bridge/driver_agytmux.go",
		"go/internal/bridge/driver_claudetmux.go",
		"go/internal/bridge/driver_codextmux.go",
		"go/internal/bridge/driver.go",
		"go/internal/adapters/bridge/bridge.go",
		"go/internal/adapters/sandbox/sandbox.go",
		"go/internal/adapters/sandbox/confinement.go",
		"go/internal/looppreflight/checks.go",
		"go/internal/looppreflight/drivers.go",
		"go/internal/preflight/preflight.go",
		"go/internal/phases/audit/explanation_review_gate.go",
		"go/internal/phases/audit/audit.go",
		"go/internal/phases/retro/explanation_review_gate.go",
		"go/internal/phases/retro/retro.go",
		"go/internal/phases/ship/native_explanation_gate.go",
		"go/internal/phases/ship/native.go",
		"go/internal/phases/ship/ship.go",
		"go/internal/phases/ship/gitops.go",
		"agents/evolve-builder-reference.md",
		"agents/evolve-auditor-reference.md",
		"agents/evolve-retrospective.md",
		"schemas/handoff/build-report.schema.json",
		"schemas/handoff/audit-report.schema.json",
		"schemas/handoff/retrospective-report.schema.json",
		".evolve/profiles/builder.json",
		".evolve/profiles/auditor.json",
		".evolve/build-explanation-contracts/cycle-42.json",
	} {
		if !IsProtectedSurface(path) {
			t.Errorf("explanation trust-boundary path %q is not protected", path)
		}
	}
}
