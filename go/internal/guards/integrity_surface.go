package guards

import (
	"path/filepath"
	"strings"
)

// ProtectedSurfaceEntry is one row of ProtectedSurfaceManifest: a
// slash-normalized, lower-case path fragment plus the rationale for why the
// surface it names is pipeline control plane. Fragment is matched ANYWHERE in
// the slash-normalized, case-folded path (see IsProtectedSurface), so a
// directory entry keeps its trailing slash and a file entry names the full
// filename.
type ProtectedSurfaceEntry struct {
	Fragment  string // "/dir/" for a directory subtree, "/dir/file.ext" for a single file
	Rationale string // why this surface grades/gates a cycle
}

// ProtectedSurfaceManifest is the SINGLE SOURCE OF TRUTH for the pipeline
// INTEGRITY CONTROL PLANE: the deterministic gates that grade a cycle, the
// campaign metric SSOT, the guards themselves, the campaign contract, the
// grading rubrics, and the PreToolUse hook wiring.
//
// No autonomous phase agent may modify these — a cycle must never be able to
// edit the gate that judges it. This is the structural fix for the cycle-20
// breach, where the build agent edited
// go/acs/regression/flagreaders/readers_test.go (and the registry SSOT) to bless
// its own split-const dodge, and the audit approved the self-edit.
//
// Legitimate control-plane changes (e.g. an operator hardening a gate) go through
// human-gated `evolve ship --class manual` OUTSIDE any cycle, where the role
// guard's CycleID==0 path allows them — never an autonomous `--class cycle`.
//
// Fragments are matched anywhere in the slash-normalized path, so the boundary
// holds regardless of the file's physical root (a per-cycle worktree, the branch
// root, or main). The intentionally NARROW fragments (e.g. go/acs/regression/ but
// NOT go/acs/, registry_table.go but NOT the whole flagregistry package) preserve
// legitimate cycle behavior: a cycle still writes its own go/acs/cycleN/
// predicates and ordinary source.
//
// The manifest is deliberately a COMPILED Go value, not config: the boundary
// must not be config-softenable (a .evolve/policy.json knob here would let a
// cycle write the knob that disarms the guard). Its durable tripwire is
// go/acs/regression/protectedsurface, which walks the repo for gate-shaped Go
// files and RED-fails when one is not covered here — so the perimeter cannot
// silently rot as the trust kernel grows (L4, architecture review 2026-07-16).
var ProtectedSurfaceManifest = []ProtectedSurfaceEntry{
	{Fragment: "/go/acs/regression/", Rationale: "standing deterministic gates"},
	{Fragment: "/go/internal/acssuite/", Rationale: "the gate runner"},
	{Fragment: "/go/internal/guards/", Rationale: "the guards (incl. this manifest + role.go)"},
	{Fragment: "/go/internal/flagregistry/registry_table.go", Rationale: "the campaign metric SSOT"},
	{Fragment: "/go/internal/flagregistry/registry_ceiling_test.go", Rationale: "the ceiling ratchet gate"},
	{Fragment: "/knowledge-base/research/flag-campaign-plan.json", Rationale: "the campaign contract"},
	{Fragment: "/skills/audit/", Rationale: "the audit grading rubric"},
	{Fragment: "/skills/adversarial-testing/", Rationale: "the adversarial anti-gaming rubric (M4 goal-integrity)"},
	{Fragment: "/skills/fable/", Rationale: "the operating-discipline overlay preloaded into deep/top-tier phase agents (policy.ResolveOverlays → bridge skill-overlay injection); its SKILL.md persona is integrity-load-bearing once injected into every phase prompt (audit-F1)"},
	{Fragment: "/.claude/settings.json", Rationale: "PreToolUse hook wiring (repo + global ~/.claude)"},
	{Fragment: "/.evolve/policy.json", Rationale: "gate-default overrides (eval/contract/swarm gates)"},

	// L4 additions (architecture review 2026-07-16): gate-shaped surfaces the
	// pre-L4 list left silently in-cycle-writable. Dir entries where every file
	// in the package is control plane; file entries where the package (core,
	// phases/ship) is otherwise legitimate cycle territory.
	{Fragment: "/go/internal/commitgate/", Rationale: "the pre-commit quality gate (attestation writer the manual-ship reader trusts)"},
	{Fragment: "/go/internal/phaseintegrity/", Rationale: "the per-phase integrity chain's DigestSource (ADR-0065)"},
	{Fragment: "/go/internal/binaryguard/", Rationale: "the compiled-binary commit backstop at the commit chokepoint"},
	{Fragment: "/go/internal/guardslog/", Rationale: "the shared guards-log writer (the guards' evidence trail)"},
	{Fragment: "/go/internal/cli/guardcmd/", Rationale: "the CLI wiring of the trust-kernel guard + gate subcommands"},
	{Fragment: "/go/internal/core/workspace_guard.go", Rationale: "workspace-pollution guard (archives a polluted workspace)"},
	{Fragment: "/go/internal/core/activating_fields_guard_test.go", Rationale: "ADR-0058 registry trust anchor (drift-loud activating-fields guard)"},
	{Fragment: "/go/internal/core/orchestrator_guard_test.go", Rationale: "inserted-phase tree-diff guard pin (cycle-274 G)"},
	{Fragment: "/go/internal/core/phase_advisor_guard_test.go", Rationale: "advisor routing guard pin"},
	{Fragment: "/go/internal/phases/ship/binary_staging_guard.go", Rationale: "staging-time backstop against compiled-binary commits (tracked-binary-in-acs-dir class fix)"},
	{Fragment: "/go/internal/phases/ship/binary_staging_guard_test.go", Rationale: "the staging backstop's own regression pin"},
}

// IsProtectedSurface reports whether path targets the pipeline integrity control
// plane. path may be absolute or repo-relative; matching is on a slash-normalized
// fragment, so the boundary holds regardless of the file's physical root.
func IsProtectedSurface(path string) bool {
	if path == "" {
		return false
	}
	p := filepath.ToSlash(path)
	// Ensure a leading slash so a leading segment ("go/acs/regression/...") still
	// matches a "/go/acs/regression/" fragment.
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	// Case-fold: macOS/Windows filesystems are case-insensitive, so "Go/ACS/..."
	// is the same path as "go/acs/..."; fragments are already lower-case.
	p = strings.ToLower(p)
	for _, e := range ProtectedSurfaceManifest {
		if strings.Contains(p, e.Fragment) {
			return true
		}
	}
	return false
}
