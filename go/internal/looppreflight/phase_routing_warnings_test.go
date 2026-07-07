package looppreflight

// phase_routing_warnings_test.go — RED contract for cycle-591's
// phase-routing-warning-escalation task.
//
// SCOUT-REPORT PIVOT (Rule 3, documented in test-report.md): scout's Task 1
// ("Fix memo phase routing collision") re-described a defect that cycles
// 547/554/563 already fixed in full — ValidateUserSpecWithCatalog,
// ApplyUserRouting(3-arg), and Catalog.Merge all already exempt/adopt the
// optional-builtin-name overlay shape (see validate_builtin_exempt_test.go /
// merge_builtin_exempt_test.go in internal/phasespec), and `go test
// ./internal/phasespec/...` is GREEN today. Scout's Task 2 (untracked binary
// in go/acs/cycle536 + .gitignore + staging guard) is likewise already fully
// shipped: `git ls-files go/acs/cycle536/` has no binary, .gitignore already
// has `go/acs/**/evolve`, and internal/binaryguard exists with passing tests.
// Writing RED tests against either already-GREEN surface would violate
// "RED is success" (a test that fails to fail proves nothing).
//
// The one genuinely unimplemented piece from scout's own "Beyond-the-Ask
// Hypotheses" section survives: every caller of phasespec's warning-producing
// functions (DiscoverUserSpecsFromRoots, Catalog.Merge, ApplyUserRouting) —
// go/cmd/evolve/cmd_cycle.go:386-399 and
// go/internal/core/routing_dispatch.go:155-166 — only fmt.Fprintf them to
// stderr and discard them; nothing in the codebase escalates a dropped/invalid
// user-phase spec into a structured, gate-visible signal (confirmed: no
// HealthSignal/SignalCenter integration anywhere consumes these warnings).
//
// FIX CONTRACT (this cycle's new surface — undefined until Builder adds it,
// so this package fails to compile today; that compile failure IS the RED
// evidence, mirroring the phasespec package's own cycle-547/554 precedent):
//
//   - Options gains a new seam field, PhaseRoutingWarnings func() []string.
//     nil (the production default) wires to phasespec.MergedCatalog(projectRoot)
//     with the error swallowed (fail-open, matching DiscoverUserSpecs'
//     existing "missing dir → no specs" posture — a preflight gate must never
//     itself become the reason a batch can't start).
//   - A new check, checkPhaseRoutingWarnings(o resolved) CheckResult, named
//     "phase-routing-warnings": LevelPass when the seam returns no warnings;
//     LevelWarn (never LevelHalt — a dropped user phase is degraded-but-
//     runnable, the built-in spine is untouched) when it returns any,
//     joining them into Detail so the operator sees every one, not just a
//     count.
//   - Run() adds checkPhaseRoutingWarnings(o) to its checks slice, so an
//     invalid/dropped user-phase spec surfaces in the SAME accumulated,
//     gate-visible Result that every other readiness problem does — reusing
//     looppreflight's existing CheckResult/CheckLevel machinery
//     (never_duplicate_centralize_via_design_patterns) instead of inventing a
//     second WARN-collection type.
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Positive : TestRun_PhaseRoutingWarnings_WarningsPresent_Warn — the core
//     ask itself: a warning that used to vanish into stderr now surfaces in
//     the structured Result.
//   - Negative : TestRun_PhaseRoutingWarnings_NoWarnings_Pass — an empty seam
//     must not fabricate a warning (no-op-detector: a stub that always warns
//     would fail this).
//   - Anti-gaming (the critical negative) :
//     TestRun_PhaseRoutingWarnings_DoesNotHalt — many warnings must still be
//     LevelWarn, never LevelHalt. The cheapest gaming fake for "escalate
//     WARN to a health signal" is to route it straight to LevelHalt (trivially
//     "escalated"); that would turn every legitimate, working
//     memo-overlay-style deployment into a batch-blocking failure the moment
//     ANY unrelated user phase.json has a typo — a regression this test
//     exists to prevent.
//   - E2E      : TestRun_PhaseRoutingWarnings_DefaultUsesRealMergedCatalog
//     drives the actual production default (no injected seam) against a real
//     on-disk registry + a hijack-shaped user overlay and asserts the warning
//     reaches Result — proving the wiring end to end, not just the check
//     function in isolation.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun_PhaseRoutingWarnings_NoWarnings_Pass(t *testing.T) {
	opts := goodPipelineOptions(t)
	opts.PhaseRoutingWarnings = func() []string { return nil }
	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	c := findCheck(t, r, "phase-routing-warnings")
	if c.Level != LevelPass {
		t.Fatalf("want LevelPass, got %s (%s)", c.Level, c.Detail)
	}
	if r.Halted() {
		t.Fatalf("expected no halt, got OverallLevel=%s checks=%+v", r.OverallLevel, r.Checks)
	}
}

func TestRun_PhaseRoutingWarnings_WarningsPresent_Warn(t *testing.T) {
	opts := goodPipelineOptions(t)
	const wantWarning = `phase widget not routed (invalid): user phase must be optional:true`
	opts.PhaseRoutingWarnings = func() []string { return []string{wantWarning} }
	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	c := findCheck(t, r, "phase-routing-warnings")
	if c.Level != LevelWarn {
		t.Fatalf("want LevelWarn, got %s (%s)", c.Level, c.Detail)
	}
	if !strings.Contains(c.Detail, wantWarning) {
		t.Fatalf("Detail must carry the full warning text (proving it is no longer silently swallowed to stderr); got %q", c.Detail)
	}
	if r.OverallLevel != LevelWarn {
		t.Fatalf("Result.OverallLevel = %s, want LevelWarn (the warning must surface at the top-level verdict)", r.OverallLevel)
	}
}

// TestRun_PhaseRoutingWarnings_DoesNotHalt is the anti-gaming negative: the
// escalation must land at LevelWarn, never LevelHalt, no matter how many
// warnings accumulate — a dropped/invalid user phase never blocks the
// built-in spine from running.
func TestRun_PhaseRoutingWarnings_DoesNotHalt(t *testing.T) {
	opts := goodPipelineOptions(t)
	opts.PhaseRoutingWarnings = func() []string {
		return []string{
			"phase widget not routed (invalid): name must be multi-word kebab-case",
			"phase audit clashes with a built-in — built-in kept, user definition ignored",
			"skipped .evolve/phases/broken/phase.json: malformed JSON",
		}
	}
	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if r.Halted() {
		t.Fatalf("phase-routing warnings must never halt the batch; got OverallLevel=%s", r.OverallLevel)
	}
	c := findCheck(t, r, "phase-routing-warnings")
	if c.Level == LevelHalt {
		t.Fatalf("checkPhaseRoutingWarnings must never return LevelHalt; got %s", c.Level)
	}
	if c.Level != LevelWarn {
		t.Fatalf("want LevelWarn for 3 accumulated warnings, got %s", c.Level)
	}
}

// TestRun_PhaseRoutingWarnings_DefaultUsesRealMergedCatalog drives the actual
// production default (Options.PhaseRoutingWarnings left nil) against a real
// on-disk registry + a hijack-shaped user overlay (an operator names an
// overlay "audit", a non-optional built-in) — the exact scenario
// merge_builtin_exempt_test.go's TestCatalog_Merge_NonOptionalBuiltinClashStillDropped
// proves warns at the phasespec layer. This test proves that warning actually
// reaches the preflight Result by default, with no seam override, closing the
// gap scout's Beyond-the-Ask Hypothesis identified.
func TestRun_PhaseRoutingWarnings_DefaultUsesRealMergedCatalog(t *testing.T) {
	root := t.TempDir()
	registryDir := filepath.Join(root, "docs", "architecture")
	if err := os.MkdirAll(registryDir, 0o755); err != nil {
		t.Fatalf("MkdirAll registry dir: %v", err)
	}
	registry := `{"phases":[{"name":"audit","optional":false},{"name":"build","optional":false}]}`
	if err := os.WriteFile(filepath.Join(registryDir, "phase-registry.json"), []byte(registry), 0o644); err != nil {
		t.Fatalf("write phase-registry.json: %v", err)
	}
	overlayDir := filepath.Join(root, ".evolve", "phases", "audit")
	if err := os.MkdirAll(overlayDir, 0o755); err != nil {
		t.Fatalf("MkdirAll overlay dir: %v", err)
	}
	overlay := `{"name":"audit","optional":true,"agent":"evolve-hijack"}`
	if err := os.WriteFile(filepath.Join(overlayDir, "phase.json"), []byte(overlay), 0o644); err != nil {
		t.Fatalf("write phase.json: %v", err)
	}

	opts := goodPipelineOptions(t)
	opts.ProjectRoot = root
	opts.PhaseRoutingWarnings = nil // exercise the real production default

	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	c := findCheck(t, r, "phase-routing-warnings")
	if c.Level != LevelWarn {
		t.Fatalf("want LevelWarn from the real audit-name-hijack overlay, got %s (%s)", c.Level, c.Detail)
	}
	if !strings.Contains(c.Detail, "audit") || !strings.Contains(c.Detail, "clashes with a built-in") {
		t.Fatalf("Detail must name the real clash warning from phasespec.MergedCatalog; got %q", c.Detail)
	}
	if r.Halted() {
		t.Fatalf("a dropped hijack overlay must warn, not halt; got OverallLevel=%s", r.OverallLevel)
	}
}
