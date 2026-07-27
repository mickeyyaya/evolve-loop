//go:build acs

// Package cycle1123 materializes the cycle-1123 acceptance criteria for this
// fleet lane's sole committed item, `fatalpane-strip-agent-content` (lane-scope
// pins this lane to that one todo-id; per R9.3 no predicate here binds to a
// deferred or other-lane item).
//
// TASK NATURE — the SECOND call site. Cycle-1117 closed the bridge half: the
// stop-review fast-fail (bridge.fatalPaneVerdict) now reads an agent-STRIPPED
// pane, so an agent editing the fatal registry is no longer killed on its own
// edit buffer. The registry's OTHER consumer stayed raw:
// core.adviseOnUnclassifiedFailure (failure_hook.go:87) calls
// det.Detect(report.FinalPane) unstripped. (scout-report.md named this site
// "fatalpane_persistence.go:73" — no such file exists; the real second raw
// Detect consumer is failure_hook.go:87. Same class, corrected file.)
//
// There the defect is INVERTED: the raw match drives the deterministic-first
// SHORT-CIRCUIT ("pane already classified; skipping advisor"), so agent-authored
// diff content quoting a seed makes a genuinely novel wedge look known — no C3
// consultation, no promotion, and every recurrence burns the maxExtends
// backstop again. ADR-0044's learning loop, disabled by the agent's own text.
//
// AC map (1:1 with test-report.md ## AC-Materialization):
//
//	AC1 "a single exported stripper recovery.StripAgentContent(pane,
//	     injectedPrompt, protected) implements the cycle-1117 semantics
//	     (blank-in-place D1, protect-list D2, fail-open empty prompt)"
//	    -> C1123_001 (named PASS for the recovery contract tests)
//	     + C1123_005 (exercised IN-PROCESS against the live registry).
//	AC2 "bridge.strippedForFatalPaneScan DELEGATES to it — one copy of the
//	     rules, not two"
//	    -> C1123_003 (mutation: neuter recovery.StripAgentContent => the
//	       cycle-1117 bridge diff test MUST die. A bridge that kept its own
//	       copy survives this and is rejected).
//	AC3 "adviseOnUnclassifiedFailure strips before Detect: an agent-diff line
//	     quoting a seed no longer suppresses the advisor"
//	    -> C1123_001 + C1123_002 (mutation: pass-through strip => the core
//	       diff test MUST die — the only predicate that tells a WIRED strip
//	       from an inert helper that merely exists).
//	AC4 "deterministic-first is NOT weakened: real CLI chrome, and a genuine
//	     newline-anchored dead shell sitting under agent diff content, both
//	     still short-circuit"
//	    -> C1123_001 (both are negative tests) + C1123_004 (mutation: the
//	       delete-based strip => the anchored test MUST die, replaying D1 at
//	       this call site).
//	AC5 "go test ./internal/core/... ./internal/bridge/... ./internal/recovery/...
//	     green, no regression"
//	    -> C1123_006, which additionally requires a NAMED pass for every
//	       pre-existing hook and fatal-pane test (a bare exit 0 cannot see a
//	       deleted inconvenient test).
//
// Adversarial axes. NEGATIVE: C1123_004 and the two negative core tests assert
// the system must NOT stop classifying real fatal panes — the lazy over-fix
// (strip everything) is killed there; 002/003/004 assert the new tests
// themselves DIE under mutation, so a tautological test is killed here. EDGE:
// 001 rejects `go test -run <nonexistent>`'s vacuous exit 0; 005 covers the
// empty-pane, empty-prompt, blank-protect-entry and nil-registry boundaries;
// 006 rejects exit-0-with-a-test-deleted. SEMANTIC: wiring (002), single-source
// delegation (003), line-preservation (004) and suite health (006) are four
// distinct behaviours, not one restated.
//
// No source-grep predicates (cycle-85 rule): every predicate below either
// exercises the system in-process (005) or runs it as a subprocess and asserts
// on real emitted output (001-004, 006). Mutants are applied via
// `go test -overlay` — the real tree is never written.
package cycle1123

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	corePkg     = "github.com/mickeyyaya/evolve-loop/go/internal/core"
	bridgePkg   = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	recoveryPkg = "github.com/mickeyyaya/evolve-loop/go/internal/recovery"

	// recoveryDir holds the contracted stripper; the mutation helper scans it
	// for stripFunc rather than pinning a filename, so Builder is free to
	// choose the file.
	recoveryDir = "go/internal/recovery"

	// stripFunc is the contracted API. Its parameter NAMES are part of the
	// contract (test-report.md pins them): the mutants below are Go source
	// this package emits, so they compile only against
	// (pane, injectedPrompt, protected).
	stripFunc = "func StripAgentContent(pane, injectedPrompt string, protected []string) string"

	c1123Run = "^TestC1123_"

	// The cycle-1123 contracted tests. Naming them individually is what lets
	// the mutation predicates say WHICH test a mutant must kill.
	coreDiffTest     = "TestC1123_AgentDiffQuotedSignatureStillReachesAdvisor"
	coreBareDiffTest = "TestC1123_BareDiffPrefixedSignatureStillReachesAdvisor"
	coreChromeTest   = "TestC1123_RealChromeStillSkipsAdvisor"
	coreAnchorTest   = "TestC1123_AnchoredSeedUnderDiffLineStillSkipsAdvisor"

	recoveryDiffTest    = "TestC1123_StripAgentContentBlanksAgentDiffLines"
	recoveryAnchorTest  = "TestC1123_StripAgentContentPreservesNewlineAnchor"
	recoveryProtectTest = "TestC1123_StripAgentContentProtectsSeededSignatureFromEchoStrip"
	recoveryEdgeTest    = "TestC1123_StripAgentContentEdgeCases"

	// bridgeDiffTest is cycle-1117's agent-diff test. It is the delegation
	// witness: it can only die under a mutation of recovery.StripAgentContent
	// if the bridge seam actually routes through it (AC2).
	bridgeDiffTest = "TestC1117_AgentDiffSeedTextDoesNotFastFail"
)

var c1123CoreTests = []string{coreDiffTest, coreBareDiffTest, coreChromeTest, coreAnchorTest}

var c1123RecoveryTests = []string{recoveryDiffTest, recoveryAnchorTest, recoveryProtectTest, recoveryEdgeTest}

// preExistingHookTests must survive untouched (AC4/AC5). This cycle adds a
// transform to the hook's Detect input; it may not weaken the C3 gating
// contract (stage discipline, deterministic-first, best-effort failure).
var preExistingHookTests = []string{
	"TestPhaseRecovery_ShadowDefault_NoCorrectiveAction",
	"TestPhaseRecovery_Enforce_AdvisesAndPromotes",
	"TestPhaseRecovery_Enforce_KnownPaneSkipsAdvisor",
	"TestPhaseRecovery_Enforce_AdvisorErrorIsBestEffort",
}

// preExistingFatalPaneTests guard the cycle-1117 half against a regression
// introduced while lifting its stripper into recovery (AC2/AC5).
var preExistingFatalPaneTests = []string{
	"TestC1117_AnchoredSeedSurvivesEchoStripping",
	"TestC1117_PromptQuotingSeedDoesNotSuppressBanner",
	bridgeDiffTest,
	"TestFatalPaneVerdict_EnforcePreemptsWithStop",
	"TestFatalPaneVerdict_BusyPaneNeverPreempted",
}

// TestC1123_001_contracted_tests_exist_and_pass is the coverage half of
// AC1/AC3/AC4. The "no tests to run" guard is load-bearing: `go test -run <no
// match>` exits 0, so an exit-code-only predicate greens on an empty file.
func TestC1123_001_contracted_tests_exist_and_pass(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-run", c1123Run, corePkg, recoveryPkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -run %s %s %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			c1123Run, corePkg, recoveryPkg, code, err, stdout, stderr)
	}
	if strings.Contains(stdout, "no tests to run") || strings.Contains(stderr, "no tests to run") {
		t.Fatalf("no test matches %s — the cycle-1123 contract was never authored (exit 0 here is the vacuous pass this predicate rejects)\nstdout:\n%s", c1123Run, stdout)
	}
	for _, name := range append(append([]string{}, c1123CoreTests...), c1123RecoveryTests...) {
		if !strings.Contains(stdout, "--- PASS: "+name+" ") {
			t.Errorf("missing PASS for %s (renamed, skipped, deleted, or not run)\nstdout:\n%s", name, stdout)
		}
	}
}

// TestC1123_002_core_diff_test_dies_when_the_strip_is_a_pass_through is AC3's
// wiring discriminator: with the shared stripper returning the pane unchanged,
// the hook is back to matching the registry against raw agent content, so the
// core diff test MUST fail. A helper that exists but is never called from
// failure_hook.go cannot survive this.
func TestC1123_002_core_diff_test_dies_when_the_strip_is_a_pass_through(t *testing.T) {
	overlay := mutateStrip(t, passThroughMutant)
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-overlay", overlay, "-run", c1123Run, corePkg)
	assertMutantKills(t, coreDiffTest, "a pass-through (unwired) strip", stdout, stderr, code)
}

// TestC1123_003_bridge_diff_test_dies_under_the_same_mutation is AC2, the
// single-source proof. The cycle-1117 bridge test can only notice a mutation of
// recovery.StripAgentContent if bridge.strippedForFatalPaneScan DELEGATES to
// it. A bridge that keeps its own copy of the rules passes its own suite and is
// rejected here — which is exactly the drift this cycle exists to prevent.
func TestC1123_003_bridge_diff_test_dies_under_the_same_mutation(t *testing.T) {
	overlay := mutateStrip(t, passThroughMutant)
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-overlay", overlay, "-run", "^TestC1117_", bridgePkg)
	assertMutantKills(t, bridgeDiffTest, "a pass-through strip in recovery (proves the bridge seam delegates rather than duplicating)", stdout, stderr, code)
}

// TestC1123_004_anchor_test_dies_under_the_delete_based_strip is AC4's
// mutation: it replays defect D1 at the hook. With matched lines DELETED and
// rejoined, the survivor below loses its leading "\n", the four newline-
// anchored dead-shell seeds stop matching, and a genuinely wedged shell reads
// as novel — so the anchored core test MUST fail. A strip test that does not
// depend on line POSITIONS survives this and is rejected.
func TestC1123_004_anchor_test_dies_under_the_delete_based_strip(t *testing.T) {
	overlay := mutateStrip(t, deleteBasedMutant)
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-overlay", overlay, "-run", c1123Run, corePkg)
	assertMutantKills(t, coreAnchorTest, "the delete-and-rejoin strip (D1)", stdout, stderr, code)
}

// TestC1123_005_strip_behaves_against_the_live_registry is AC1, exercised
// IN-PROCESS against the real seeded registry rather than a fixture — the
// protect-list must come from the registry it is protecting, and the boundaries
// the callers actually pass (empty prompt from failure_hook.go, blank protect
// entries) must be safe.
func TestC1123_005_strip_behaves_against_the_live_registry(t *testing.T) {
	det := recovery.SeedDetector()
	protected := det.Signatures()
	if len(protected) == 0 {
		t.Fatal("Signatures() is empty — a protect-list built from it protects nothing")
	}

	// Agent-authored seed text is removed, and line positions are preserved.
	pane := "⏺ Editing detector.go\n    72 +\t\tSubstr: \"There's an issue with the selected model\",\ntail"
	got := recovery.StripAgentContent(pane, "", protected)
	if _, _, ok := det.Detect(got); ok {
		t.Errorf("Detect fires on a pane whose only signature is agent diff content\nstripped:\n%s", got)
	}
	if want, have := strings.Count(pane, "\n"), strings.Count(got, "\n"); have != want {
		t.Errorf("stripped pane has %d newlines, want %d — lines were deleted, not blanked (D1)", have, want)
	}

	// Every seeded signature still fires when it is genuinely on-pane, even
	// with a prompt that quotes it verbatim (D2) — including the four
	// newline-anchored ones.
	for _, sig := range protected {
		raw := "boot\n" + strings.TrimPrefix(sig, "\n") + "\ntail"
		if _, _, ok := det.Detect(recovery.StripAgentContent(raw, raw, protected)); !ok {
			t.Errorf("seeded signature %q stopped matching after stripping against a prompt that quotes it (D2/D1)", sig)
		}
	}

	// Boundaries the production callers actually pass.
	plain := "boot\nordinary agent sentence\ntail"
	if got := recovery.StripAgentContent(plain, "", nil); got != plain {
		t.Errorf("empty prompt + nil protect-list must strip no echoes (fail-open); got:\n%s", got)
	}
	if got := recovery.StripAgentContent("", "prompt", nil); got != "" {
		t.Errorf("empty pane must yield an empty pane; got %q", got)
	}
	if got := recovery.StripAgentContent("    9 +\tThere's an issue with the selected model", "", []string{"", " "}); strings.Contains(got, "issue with the selected model") {
		t.Errorf("a blank protect-list entry suppressed the diff strip — blank entries match every line and must be ignored; got %q", got)
	}
	if got := recovery.NewFatalPaneDetector(nil).Signatures(); len(got) != 0 {
		t.Errorf("empty registry Signatures() = %v, want no entries (the hook calls it unconditionally)", got)
	}
}

// TestC1123_006_suites_stay_green is AC5. The named-PASS sweep rejects the
// exit-0-after-deleting-an-inconvenient-test shape a bare `go test` cannot see.
func TestC1123_006_suites_stay_green(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", corePkg, bridgePkg, recoveryPkg)
	if code != 0 || err != nil {
		t.Fatalf("go test %s %s %s exited %d (err=%v) — the second-call-site fix regressed a sibling package\nstdout:\n%s\nstderr:\n%s",
			corePkg, bridgePkg, recoveryPkg, code, err, stdout, stderr)
	}
	for _, name := range append(append([]string{}, preExistingHookTests...), preExistingFatalPaneTests...) {
		if !strings.Contains(stdout, "--- PASS: "+name+" ") {
			t.Errorf("pre-existing test %s no longer reports PASS (deleted, renamed, or skipped) — neither the C3 gating contract nor the cycle-1117 bridge seam may be weakened to land this change", name)
		}
	}
}

// passThroughMutant neuters the strip while keeping every parameter and the
// "strings" import used, so the mutant compiles.
const passThroughMutant = `_ = injectedPrompt
	_ = protected
	return strings.Join(strings.Split(pane, "\n"), "\n")`

// deleteBasedMutant is cycle-1115's rejected shape: line-DELETING instead of
// blanking, which collapses the newline anchors (D1).
const deleteBasedMutant = `_ = injectedPrompt
	_ = protected
	lines := strings.Split(pane, "\n")
	kept := lines[:0]
	for _, ln := range lines {
		trimmed := strings.TrimLeft(ln, " \t")
		if strings.HasPrefix(trimmed, "+++") || strings.HasPrefix(trimmed, "---") {
			kept = append(kept, ln)
			continue
		}
		for len(trimmed) > 0 && trimmed[0] >= '0' && trimmed[0] <= '9' {
			trimmed = strings.TrimLeft(trimmed[1:], " \t")
		}
		if strings.HasPrefix(trimmed, "+") || strings.HasPrefix(trimmed, "-") {
			continue
		}
		kept = append(kept, ln)
	}
	return strings.Join(kept, "\n")`

// mutateStrip rewrites whichever file in go/internal/recovery declares
// stripFunc, replacing that function's body with body, and returns the path of
// a `go test -overlay` file mapping the real source at the mutant. The real
// tree is never written.
//
// It fails loudly when the function is absent or its signature drifted: a
// silently-unapplied mutation would make 002/003/004 pass for the wrong reason
// (the "mutant" would be the pristine source, whose tests are green).
func mutateStrip(t *testing.T, body string) string {
	t.Helper()
	dir := filepath.Join(acsassert.RepoRoot(t), recoveryDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var src, text string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		raw, rerr := os.ReadFile(p)
		if rerr != nil {
			t.Fatalf("read %s: %v", p, rerr)
		}
		if strings.Contains(string(raw), stripFunc) {
			src, text = p, string(raw)
			break
		}
	}
	if src == "" {
		t.Fatalf("contracted API absent from %s:\n\t%s\nEither the shared stripper was never written, or its name/parameter names drifted from the cycle-1123 contract (test-report.md pins them so these mutants compile). Restore the signature, or update this predicate deliberately — never delete it.", recoveryDir, stripFunc)
	}
	start := strings.Index(text, stripFunc)
	end := strings.Index(text[start:], "\n}\n")
	if end < 0 {
		t.Fatalf("could not find the closing brace of %s in %s", stripFunc, src)
	}
	end += start + len("\n}\n")
	mutantFn := stripFunc + " {\n\t" + body + "\n}\n"

	tmp := t.TempDir()
	mutant := filepath.Join(tmp, "strip_mutant.go")
	if err := os.WriteFile(mutant, []byte(text[:start]+mutantFn+text[end:]), 0o644); err != nil {
		t.Fatalf("write mutant: %v", err)
	}
	overlay := filepath.Join(tmp, "overlay.json")
	doc, err := json.Marshal(map[string]map[string]string{"Replace": {src: mutant}})
	if err != nil {
		t.Fatalf("marshal overlay: %v", err)
	}
	if err := os.WriteFile(overlay, doc, 0o644); err != nil {
		t.Fatalf("write overlay: %v", err)
	}
	return overlay
}

// assertMutantKills requires the named test to have FAILED under the mutation.
// A build failure is rejected too: a non-zero exit from a broken build proves
// nothing about the test's assertions.
func assertMutantKills(t *testing.T, name, mutation, stdout, stderr string, code int) {
	t.Helper()
	for _, marker := range []string{"build failed", "cannot use", "undefined:", "declared and not used", "imported and not used", "syntax error"} {
		if strings.Contains(stderr, marker) {
			t.Fatalf("mutant (%s) failed to COMPILE (%q) — a non-zero exit from a broken build is not evidence the test is load-bearing. The contracted stripper's file must stay compilable when only its body is replaced (keep helper funcs and imports used by more than this one function).\nstderr:\n%s", mutation, marker, stderr)
		}
	}
	if code == 0 {
		t.Fatalf("%s still PASSES with %s — the test is tautological (it does not depend on the behaviour it claims to pin)\nstdout:\n%s", name, mutation, stdout)
	}
	if !strings.Contains(stdout, "--- FAIL: "+name+" ") {
		t.Errorf("expected %s to FAIL under mutation (%s); it did not appear as a failure\nstdout:\n%s\nstderr:\n%s", name, mutation, stdout, stderr)
	}
}
