//go:build legacy

// Package cycle54 ports the cycle-54 ACS predicates (4 bash files).
//
// Per parent plan §4 Phase 4: the bash predicates remain authoritative
// for execution-flavored tests (subprocess invocations, mock fixtures,
// anti-tautology env-var seams). The Go counterparts assert the
// presence and wiring of the same markers — sufficient signal that the
// production code path hasn't regressed structurally.
package cycle54

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// TestC54_005_GeminiNativeInvocation ports cycle-54/005 (wiring-only).
// gemini.sh must have the NATIVE-mode detection + binary-override seam.
func TestC54_005_GeminiNativeInvocation(t *testing.T) {
	root := acsassert.RepoRoot(t)
	gemini := filepath.Join(root, "legacy", "scripts", "cli_adapters", "gemini.sh")
	if !acsassert.FileExists(t, gemini) {
		t.Skip("gemini.sh missing — skip cycle-54-005")
	}
	for _, marker := range []string{"detect_gemini_native", "EVOLVE_GEMINI_BINARY"} {
		if !acsassert.FileContains(t, gemini, marker) {
			return
		}
	}
}

// TestC54_006_CodexNativeInvocation ports cycle-54/006 (wiring-only).
func TestC54_006_CodexNativeInvocation(t *testing.T) {
	root := acsassert.RepoRoot(t)
	codex := filepath.Join(root, "legacy", "scripts", "cli_adapters", "codex.sh")
	if !acsassert.FileExists(t, codex) {
		t.Skip("codex.sh missing — skip cycle-54-006")
	}
	for _, marker := range []string{"detect_codex_native", "EVOLVE_CODEX_BINARY"} {
		if !acsassert.FileContains(t, codex, marker) {
			return
		}
	}
}

// TestC54_009_AdapterOverridesBlockHonored ports cycle-54/009 (wiring-only).
// subagent-run.sh has the adapter_overrides export block (ADR-6).
func TestC54_009_AdapterOverridesBlockHonored(t *testing.T) {
	root := acsassert.RepoRoot(t)
	sub := filepath.Join(root, "legacy", "scripts", "dispatch", "subagent-run.sh")
	if !acsassert.FileExists(t, sub) {
		t.Skip("subagent-run.sh missing — skip cycle-54-009")
	}
	for _, marker := range []string{"adapter_overrides", "ADAPTER_TOOLS_OVERRIDE"} {
		if !acsassert.FileContains(t, sub, marker) {
			return
		}
	}
}

// TestC54_010_TrustKernelCliIndependent ports cycle-54/010 (wiring-only).
// role-gate.sh has allow_for_phase() — phase-based, not CLI-based, enforcement.
func TestC54_010_TrustKernelCliIndependent(t *testing.T) {
	root := acsassert.RepoRoot(t)
	gate := filepath.Join(root, "legacy", "scripts", "guards", "role-gate.sh")
	if !acsassert.FileExists(t, gate) {
		t.Skip("role-gate.sh missing — skip cycle-54-010")
	}
	if !acsassert.FileContains(t, gate, "allow_for_phase") {
		return
	}
}
