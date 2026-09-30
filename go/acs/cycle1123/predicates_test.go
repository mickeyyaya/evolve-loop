//go:build acs

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

	recoveryDir = "go/internal/recovery"

	stripFunc = "func StripAgentContent(pane, injectedPrompt string, protected []string) string"

	c1123Run = "^TestC1123_"

	coreDiffTest     = "TestC1123_AgentDiffQuotedSignatureStillReachesAdvisor"
	coreBareDiffTest = "TestC1123_BareDiffPrefixedSignatureStillReachesAdvisor"
	coreChromeTest   = "TestC1123_RealChromeStillSkipsAdvisor"
	coreAnchorTest   = "TestC1123_AnchoredSeedUnderDiffLineStillSkipsAdvisor"

	recoveryDiffTest    = "TestC1123_StripAgentContentBlanksAgentDiffLines"
	recoveryAnchorTest  = "TestC1123_StripAgentContentPreservesNewlineAnchor"
	recoveryProtectTest = "TestC1123_StripAgentContentProtectsSeededSignatureFromEchoStrip"
	recoveryEdgeTest    = "TestC1123_StripAgentContentEdgeCases"

	bridgeDiffTest = "TestC1117_AgentDiffSeedTextDoesNotFastFail"
)

var c1123CoreTests = []string{coreDiffTest, coreBareDiffTest, coreChromeTest, coreAnchorTest}

var c1123RecoveryTests = []string{recoveryDiffTest, recoveryAnchorTest, recoveryProtectTest, recoveryEdgeTest}

var preExistingHookTests = []string{
	"TestPhaseRecovery_ShadowDefault_NoCorrectiveAction",
	"TestPhaseRecovery_Enforce_AdvisesAndPromotes",
	"TestPhaseRecovery_Enforce_KnownPaneSkipsAdvisor",
	"TestPhaseRecovery_Enforce_AdvisorErrorIsBestEffort",
}

var preExistingFatalPaneTests = []string{
	"TestC1117_AnchoredSeedSurvivesEchoStripping",
	"TestC1117_PromptQuotingSeedDoesNotSuppressBanner",
	bridgeDiffTest,
	"TestFatalPaneVerdict_EnforcePreemptsWithStop",
	"TestFatalPaneVerdict_BusyPaneNeverPreempted",
}

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

func TestC1123_002_core_diff_test_dies_when_the_strip_is_a_pass_through(t *testing.T) {
	overlay := mutateStrip(t, passThroughMutant)
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-overlay", overlay, "-run", c1123Run, corePkg)
	assertMutantKills(t, coreDiffTest, "a pass-through (unwired) strip", stdout, stderr, code)
}

func TestC1123_003_bridge_diff_test_dies_under_the_same_mutation(t *testing.T) {
	overlay := mutateStrip(t, passThroughMutant)
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-overlay", overlay, "-run", "^TestC1117_", bridgePkg)
	assertMutantKills(t, bridgeDiffTest, "a pass-through strip in recovery (proves the bridge seam delegates rather than duplicating)", stdout, stderr, code)
}

func TestC1123_004_anchor_test_dies_under_the_delete_based_strip(t *testing.T) {
	overlay := mutateStrip(t, deleteBasedMutant)
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-overlay", overlay, "-run", c1123Run, corePkg)
	assertMutantKills(t, coreAnchorTest, "the delete-and-rejoin strip (D1)", stdout, stderr, code)
}

func TestC1123_005_strip_behaves_against_the_live_registry(t *testing.T) {
	det := recovery.SeedDetector()
	protected := det.Signatures()
	if len(protected) == 0 {
		t.Fatal("Signatures() is empty — a protect-list built from it protects nothing")
	}

	pane := "⏺ Editing detector.go\n    72 +\t\tSubstr: \"There's an issue with the selected model\",\ntail"
	got := recovery.StripAgentContent(pane, "", protected)
	if _, _, ok := det.Detect(got); ok {
		t.Errorf("Detect fires on a pane whose only signature is agent diff content\nstripped:\n%s", got)
	}
	if want, have := strings.Count(pane, "\n"), strings.Count(got, "\n"); have != want {
		t.Errorf("stripped pane has %d newlines, want %d — lines were deleted, not blanked (D1)", have, want)
	}

	for _, sig := range protected {
		raw := "boot\n" + strings.TrimPrefix(sig, "\n") + "\ntail"
		if _, _, ok := det.Detect(recovery.StripAgentContent(raw, raw, protected)); !ok {
			t.Errorf("seeded signature %q stopped matching after stripping against a prompt that quotes it (D2/D1)", sig)
		}
	}

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

const passThroughMutant = `_ = injectedPrompt
	_ = protected
	return strings.Join(strings.Split(pane, "\n"), "\n")`

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
