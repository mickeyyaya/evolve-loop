//go:build acs

// Package cycle1064 materialises the cycle-1064 acceptance criteria for the two
// fleet-scoped tasks pinned to this lane (inbox item `ship-stage-explicit-paths`,
// operator_note items 2 and 3):
//
//   - dedicated-manifest-gate-error-code → the enforce-mode manifest-gate block
//     must carry a DEDICATED core.CodeManifestGate, not the generic
//     CodeGitStageFailed a real failing `git add` also emits (which is classified
//     TRANSIENT, so an integrity block currently inherits a retry-friendly class).
//   - manifest-gate-policy-wiring → `.evolve/policy.json` gates.manifest_gate must
//     resolve through policy.GatesConfig() AND thread into the ship phase's
//     Options.ManifestGate, so the dial is activatable without a code edit.
//     Today Options.ManifestGate is never assigned at the sole production
//     construction site (ship.go runNative) — the gate is permanently shadow.
//
// Predicate strategy — every predicate EXERCISES the system under test (the
// cycle-85 degenerate-predicate ban): 001/003 call the real constructors and the
// real policy resolver in-process; 002/004 shell the package's behavioural unit
// tests, which drive reconcileManifest and the PhaseRequest→Options translation
// through their production paths. No predicate asserts on source text.
package cycle1064

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// goDir is the worktree's Go module root — the -C target for the shelled test
// lanes, so they compile the CYCLE's tree, not main's stale copy.
func goDir(t *testing.T) string { return filepath.Join(acsassert.RepoRoot(t), "go") }

// runGoTest runs one named test in the worktree module and requires a real PASS
// line (a filtered-away or renamed test exits 0 with no PASS — that is a FAIL
// here, not a silent green).
func runGoTest(t *testing.T, pkg, name string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", goDir(t), "-count=1", "-v", "-run", "^"+name+"$", pkg)
	out := stdout + stderr
	if err != nil {
		t.Fatalf("go test failed to launch (not a test failure): %v\n%s", err, out)
	}
	if code != 0 {
		t.Fatalf("%s -run %s exited %d\n%s", pkg, name, code, out)
	}
	if !strings.Contains(out, "--- PASS: "+name) {
		t.Fatalf("no PASS line for %s in %s (renamed, skipped, or never ran?)\n%s", name, pkg, out)
	}
}

// TestC1064_001_ManifestGateCodeIsDedicatedAndDualRegistered exercises the error
// vocabulary in-process: the dedicated code must exist on BOTH sides of the
// shiperr→core re-export (consumers import core), carry its own wire string, be
// distinct from the generic git-stage code it replaces, and survive the real
// NewShipError/AsShipError round trip the ledger and debugger use.
func TestC1064_001_ManifestGateCodeIsDedicatedAndDualRegistered(t *testing.T) {
	if core.CodeManifestGate != shiperr.CodeManifestGate {
		t.Fatalf("core.CodeManifestGate (%q) must re-export shiperr.CodeManifestGate (%q)",
			core.CodeManifestGate, shiperr.CodeManifestGate)
	}
	if string(core.CodeManifestGate) != "MANIFEST_GATE" {
		t.Errorf("wire string = %q, want %q", core.CodeManifestGate, "MANIFEST_GATE")
	}
	// Negative axis: the whole deliverable is DISTINGUISHABILITY.
	if core.CodeManifestGate == core.CodeGitStageFailed {
		t.Errorf("CodeManifestGate must not alias CodeGitStageFailed (%q)", core.CodeGitStageFailed)
	}
	se, ok := core.AsShipError(core.NewShipError(core.CodeManifestGate,
		core.ShipClassPrecondition, core.StageAtomicShip, "manifest-gate block"))
	if !ok || se.Code != core.CodeManifestGate || se.Class != core.ShipClassPrecondition || se.Stage != core.StageAtomicShip {
		t.Errorf("round trip = %+v (ok=%v), want MANIFEST_GATE/precondition/atomic-ship", se, ok)
	}
}

// TestC1064_002_EnforceBlockCarriesManifestGateCode drives reconcileManifest
// itself (via its package-internal behavioural tests): an enforce-mode ship
// facing an undeclared cross-lane leak must fail closed with MANIFEST_GATE, and
// the SHADOW default must still return nil and only log — the regression axis
// that keeps every cycle shippable today.
func TestC1064_002_EnforceBlockCarriesManifestGateCode(t *testing.T) {
	runGoTest(t, "./internal/phases/ship/", "TestReconcileManifest_EnforceCarriesManifestGateCode")
	runGoTest(t, "./internal/phases/ship/", "TestReconcileManifest_ShadowUnaffectedByCodeChange")
}

// TestC1064_003_PolicyManifestGateResolvesFromJSON exercises the real policy
// resolver over REAL policy.json bytes (the `manifest_gate` tag must parse — a
// mistagged field is exactly today's unreachable state), asserts the
// behavior-preserving shadow default, and pins the six pre-existing gate
// defaults against collateral drift.
func TestC1064_003_PolicyManifestGateResolvesFromJSON(t *testing.T) {
	if got := (policy.Policy{}).GatesConfig().ManifestGate; got != "shadow" {
		t.Errorf("default ManifestGate = %q, want %q (behavior-preserving)", got, "shadow")
	}
	for _, tc := range []struct{ raw, want string }{
		{`{"gates":{"manifest_gate":"enforce"}}`, "enforce"},
		{`{"gates":{"manifest_gate":"shadow"}}`, "shadow"},
		{`{"gates":{"manifest_gate":""}}`, "shadow"},
		{`{"gates":{"topn_gate":"off"}}`, "shadow"},
	} {
		var p policy.Policy
		if err := json.Unmarshal([]byte(tc.raw), &p); err != nil {
			t.Fatalf("unmarshal %s: %v", tc.raw, err)
		}
		if got := p.GatesConfig().ManifestGate; got != tc.want {
			t.Errorf("%s → ManifestGate = %q, want %q", tc.raw, got, tc.want)
		}
	}
	var p policy.Policy
	if err := json.Unmarshal([]byte(`{"gates":{"manifest_gate":"enforce"}}`), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	g := p.GatesConfig()
	for _, tc := range []struct{ name, have, want string }{
		{"ContractGate", g.ContractGate, "enforce"},
		{"EvalGate", g.EvalGate, "enforce"},
		{"TriageCapGate", g.TriageCapGate, "enforce"},
		{"ReviewGate", g.ReviewGate, "off"},
		{"ReportSizeGate", g.ReportSizeGate, "shadow"},
		{"TopNGate", g.TopNGate, "enforce"},
	} {
		if tc.have != tc.want {
			t.Errorf("%s = %q, want %q (unchanged by the new gate)", tc.name, tc.have, tc.want)
		}
	}
}

// TestC1064_004_ShipPhaseThreadsResolvedGate is the wiring crux: the ship
// PhaseRunner's PhaseRequest→Options translation must carry the config-sourced
// mode, and a policy-resolved "enforce" must reach an actual block. Shelled
// because the translation seam is package-internal; both named tests drive the
// production path end to end (policy JSON → GatesConfig → ship Config → Options
// → reconcileManifest block).
func TestC1064_004_ShipPhaseThreadsResolvedGate(t *testing.T) {
	runGoTest(t, "./internal/phases/ship/", "TestShipOptions_ThreadsManifestGate")
	runGoTest(t, "./internal/phases/ship/", "TestManifestGate_PolicyToBlockEndToEnd")
}
