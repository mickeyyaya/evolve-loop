//go:build acs

// Package cycle8 materializes the cycle-1 acceptance criteria for two committed tasks:
//
//   - campaign-cmd-driver — add go/cmd/evolve/cmd_campaign.go implementing
//     `evolve campaign` with three subcommands (study, replan, run) and wire
//     it into registry.go.
//
//   - campaign-adr-and-citations — write ADR-0056 and the companion
//     docs/architecture/campaign-planning-citations.md.
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	campaign-cmd-driver:
//	  AC1  "campaign" entry in registry.go / lookupCommand returns non-nil → C1_001
//	  AC2  `evolve campaign` (no args) prints usage and exits non-zero         → C1_002
//	  AC3  `study` subcommand accessible (not "unknown command")               → C1_003
//	  AC4  `replan --feedback` subcommand accessible                           → C1_004
//	  AC5  `run --plan <path> --simulate` exits 0 with valid plan              → C1_005
//	  [adversarial] run with invalid plan exits non-zero                       → C1_005neg
//	  AC6  go build ./cmd/evolve/... and go vet pass                           → C1_006 (pre-existing GREEN)
//
//	campaign-adr-and-citations:
//	  AC1  ADR-0056 file exists and is git-tracked                             → C2_001
//	  AC2  ADR has ## Status / ## Context / ## Decision / ## Consequences      → C2_002
//	  AC3  ADR describes all four slices S1–S4                                 → C2_003
//	  AC4  campaign-planning-citations.md exists, non-empty (≥5 lines)         → C2_004
//	  AC5  No placeholder URLs (example.com / TODO) in ADR                    → C2_005
//
// Floor binding (R9.3): predicates only for committed top_n tasks.
package cycle8

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/pkg/acsassert"
)

// binPath returns the absolute path to the built evolve binary in the worktree.
func binPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go", "bin", "evolve")
}

// goDir returns the go module directory for subprocess calls.
func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

// minimalValidPlanJSON returns a minimal campaign-plan.json content that passes
// campaign.Verify(): version ≥ 1, non-empty goal, at least one cycle with a
// non-empty id, acyclic DAG.
func minimalValidPlanJSON(t *testing.T) []byte {
	t.Helper()
	type todo struct {
		ID string `json:"id"`
	}
	type plan struct {
		Version int    `json:"version"`
		Goal    string `json:"goal"`
		Cycles  []todo `json:"cycles"`
	}
	data, err := json.Marshal(plan{
		Version: 1,
		Goal:    "test campaign goal for predicate",
		Cycles:  []todo{{ID: "c1"}},
	})
	if err != nil {
		t.Fatalf("minimalValidPlanJSON: %v", err)
	}
	return data
}

// runEvolve runs "evolve <args...>" via bash from the repo root and returns
// combined stdout+stderr, the exit code, and any exec error.
func runEvolve(t *testing.T, args ...string) (combined string, code int) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	bin := binPath(t)
	quotedArgs := make([]string, len(args))
	for i, a := range args {
		quotedArgs[i] = "'" + strings.ReplaceAll(a, "'", "'\\''") + "'"
	}
	cmd := "cd " + "'" + root + "'" + " && " + "'" + bin + "'" + " " + strings.Join(quotedArgs, " ")
	out, errOut, c, _ := acsassert.SubprocessOutput("bash", "-c", cmd)
	return strings.TrimSpace(out + "\n" + errOut), c
}

// ── campaign-cmd-driver ──────────────────────────────────────────────────────

// TestC1_001_CampaignEntryInRegistry verifies that `registry.go` contains a
// "campaign" entry AND that running `evolve campaign` does not return an
// "unknown command" error.
//
// MIXED: FileContains on registry.go (config-check waiver — the registry table
// is the authoritative routing source, checking its content IS checking
// behavior) plus a behavioral subprocess that discriminates "registered" from
// "unregistered".
//
// // acs-predicate: config-check
//
// RED: registry.go currently has no "campaign" entry → FileContains fails;
// subprocess prints "unknown command: campaign" → string check fires.
func TestC1_001_CampaignEntryInRegistry(t *testing.T) {
	root := acsassert.RepoRoot(t)
	registryPath := filepath.Join(root, "go", "cmd", "evolve", "registry.go")

	// Config-check: "campaign" must appear in the registry table.
	// acs-predicate: config-check
	if !acsassert.FileContains(t, registryPath, `"campaign"`) {
		t.Errorf("RED: registry.go missing \"campaign\" entry.\n"+
			"Builder must add {Name: \"campaign\", ...} row to go/cmd/evolve/registry.go.\n"+
			"File: %s", registryPath)
	}

	// Behavioral: running `evolve campaign` must NOT print "unknown command".
	combined, _ := runEvolve(t, "campaign")
	if strings.Contains(combined, "unknown command") {
		t.Errorf("RED: `evolve campaign` printed \"unknown command\" — command is not registered.\n"+
			"Builder must add the campaign entry to registry.go.\nOutput:\n%s", combined)
	}
}

// TestC1_002_NoArgsExitsNonZeroWithCampaignUsage verifies that `evolve campaign`
// with no subcommand exits non-zero AND that stdout/stderr mentions the expected
// subcommands (study, replan, or run) — i.e. a real usage message, not an
// "unknown command" error.
//
// BEHAVIORAL: runs the real evolve binary; a magic-string addition to the source
// cannot satisfy this without actually registering and implementing the command.
//
// RED: `evolve campaign` currently exits non-zero with "unknown command: campaign"
// (the generic dispatcher error), which does NOT contain "study", "replan", or
// "run". Builder must implement cmd_campaign.go to produce a proper usage message.
func TestC1_002_NoArgsExitsNonZeroWithCampaignUsage(t *testing.T) {
	combined, code := runEvolve(t, "campaign")
	if code == 0 {
		t.Errorf("RED: `evolve campaign` (no args) exited 0 — expected non-zero (usage error).\n"+
			"Output:\n%s", combined)
	}
	// "study" only appears in campaign usage, not in the generic dispatcher message.
	if !strings.Contains(combined, "study") {
		t.Errorf("RED: `evolve campaign` (no args) output does not mention \"study\".\n"+
			"Expected campaign-specific usage message with subcommand names; got:\n%s\n"+
			"Builder must implement cmd_campaign.go with proper usage output.", combined)
	}
}

// TestC1_003_StudySubcommandAccessible verifies that `evolve campaign study`
// does NOT return "unknown command: campaign" — i.e. the study subcommand is
// registered and dispatched (though it may fail without a workspace argument).
//
// BEHAVIORAL: discriminates "subcommand registered" from "command unknown".
//
// RED: currently `evolve campaign` itself is unknown, so the output contains
// "unknown command: campaign". After Builder adds cmd_campaign.go, `evolve
// campaign study` fails for the correct reason (missing workspace), not
// because campaign is unregistered.
func TestC1_003_StudySubcommandAccessible(t *testing.T) {
	combined, _ := runEvolve(t, "campaign", "study")
	if strings.Contains(combined, "unknown command") {
		t.Errorf("RED: `evolve campaign study` printed \"unknown command\" — "+
			"campaign command is not registered.\n"+
			"Builder must implement go/cmd/evolve/cmd_campaign.go with a study subcommand.\n"+
			"Output:\n%s", combined)
	}
}

// TestC1_004_ReplanSubcommandAccessible verifies that `evolve campaign replan
// --feedback "some feedback"` does NOT return "unknown command: campaign".
//
// BEHAVIORAL: same discriminator as C1_003 — registered vs unregistered.
//
// RED: campaign command unregistered → "unknown command: campaign".
// GREEN: command exists, exits non-zero for missing plan/workspace (correct error).
func TestC1_004_ReplanSubcommandAccessible(t *testing.T) {
	combined, _ := runEvolve(t, "campaign", "replan", "--feedback", "some feedback text")
	if strings.Contains(combined, "unknown command") {
		t.Errorf("RED: `evolve campaign replan --feedback ...` printed \"unknown command\" — "+
			"campaign replan subcommand is not registered.\n"+
			"Builder must implement the replan subcommand in cmd_campaign.go.\n"+
			"Output:\n%s", combined)
	}
}

// TestC1_005_RunSimulateExitsZero verifies that `evolve campaign run
// --plan <path> --simulate` exits 0 when given a valid campaign-plan.json.
//
// BEHAVIORAL: constructs a minimal valid plan, writes it to a temp file, runs
// the real binary. Simulate mode skips all LLM calls so this test is
// deterministic. Source-only edits cannot satisfy it — a real implementation
// that loads, verifies, and executes waves is required.
//
// RED: command does not exist → "unknown command: campaign" → exit non-zero.
// GREEN: command exists, loads plan, runs waves in simulate mode → exit 0.
func TestC1_005_RunSimulateExitsZero(t *testing.T) {
	planData := minimalValidPlanJSON(t)
	planFile := filepath.Join(t.TempDir(), "campaign-plan.json")
	if err := os.WriteFile(planFile, planData, 0o644); err != nil {
		t.Fatalf("write plan fixture: %v", err)
	}

	combined, code := runEvolve(t, "campaign", "run", "--plan", planFile, "--simulate")
	if code != 0 {
		t.Errorf("RED: `evolve campaign run --plan <valid> --simulate` exited %d.\n"+
			"Builder must implement cmd_campaign.go: load plan, verify, iterate waves with "+
			"--simulate flag passed to execCycleLaunch.\n"+
			"Output:\n%s", code, combined)
	}
}

// TestC1_005neg_RunWithInvalidPlanExitsNonZero is the adversarial negative test:
// `evolve campaign run --plan /dev/null --simulate` must exit non-zero because
// /dev/null is empty JSON and campaign.Load returns a parse error.
//
// This prevents a no-op implementation from satisfying C1_005 by always exiting 0.
// A correct implementation must reject malformed plans.
//
// NOTE: pre-existing GREEN — currently exits non-zero because the command does
// not exist ("unknown command: campaign"). After Builder: exits non-zero because
// campaign.Load fails on empty input.
func TestC1_005neg_RunWithInvalidPlanExitsNonZero(t *testing.T) {
	combined, code := runEvolve(t, "campaign", "run", "--plan", "/dev/null", "--simulate")
	if code == 0 {
		t.Errorf("FAIL: `evolve campaign run --plan /dev/null --simulate` exited 0.\n"+
			"An empty/invalid plan must be rejected by campaign.Load/campaign.Verify.\n"+
			"Output:\n%s", combined)
	}
}

// TestC1_006_BuildAndVetPass verifies that `go build ./cmd/evolve/...` and
// `go vet ./cmd/evolve/...` both exit 0 after Builder adds cmd_campaign.go.
//
// BEHAVIORAL: runs the actual Go toolchain on the source tree.
//
// NOTE: pre-existing GREEN — the build currently passes (no campaign code at
// all). This test becomes the regression lock: if Builder introduces a
// compilation error or vet warning in cmd_campaign.go, it will fail here.
func TestC1_006_BuildAndVetPass(t *testing.T) {
	gd := goDir(t)

	buildOut, buildErr, buildCode, _ := acsassert.SubprocessOutput(
		"go", "build", "-C", gd, "./cmd/evolve/...",
	)
	if buildCode != 0 {
		t.Errorf("RED: `go build ./cmd/evolve/...` failed (exit=%d).\n"+
			"Builder's cmd_campaign.go must compile cleanly.\n"+
			"stdout: %s\nstderr: %s", buildCode, buildOut, buildErr)
	}

	vetOut, vetErr, vetCode, _ := acsassert.SubprocessOutput(
		"go", "vet", "-C", gd, "./cmd/evolve/...",
	)
	if vetCode != 0 {
		t.Errorf("RED: `go vet ./cmd/evolve/...` failed (exit=%d).\n"+
			"Builder's cmd_campaign.go must pass go vet.\n"+
			"stdout: %s\nstderr: %s", vetCode, vetOut, vetErr)
	}
}

// ── campaign-adr-and-citations ───────────────────────────────────────────────

// TestC2_001_ADRFileExistsAndTracked verifies that the ADR-0056 file exists on
// disk AND is tracked by git (not gitignored or untracked).
//
// BEHAVIORAL: disk presence + git tracking check prevents a gitignored worktree
// file from being silently dropped at ship (cycle-92 defect mode).
//
// RED: file does not exist yet.
func TestC2_001_ADRFileExistsAndTracked(t *testing.T) {
	root := acsassert.RepoRoot(t)
	rel := filepath.Join("docs", "architecture", "adr", "0056-advisor-driven-preliminary-study-cycle.md")
	abs := filepath.Join(root, rel)

	if !acsassert.FileExists(t, abs) {
		t.Fatalf("RED: %s missing on disk.\n"+
			"Builder must create docs/architecture/adr/0056-advisor-driven-preliminary-study-cycle.md.", rel)
	}
	_, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel)
	if code != 0 {
		t.Errorf("RED: %s exists on disk but is not git-tracked — may be gitignored (dropped at ship).\n"+
			"Builder must `git add` the ADR file.", rel)
	}
}

// TestC2_002_ADRHasRequiredSections verifies that ADR-0056 contains the four
// standard ADR section headers.
//
// // acs-predicate: config-check
// Structural doc sections are inherently a config-presence check; the waiver
// applies. Each section name is the unambiguous identity of the required ADR
// structure per the project's ADR convention.
//
// RED: file does not exist (FileContains fails on read error).
func TestC2_002_ADRHasRequiredSections(t *testing.T) {
	root := acsassert.RepoRoot(t)
	// acs-predicate: config-check
	adrPath := filepath.Join(root, "docs", "architecture", "adr", "0056-advisor-driven-preliminary-study-cycle.md")
	for _, section := range []string{"## Status", "## Context", "## Decision", "## Consequences"} {
		if !acsassert.FileContains(t, adrPath, section) {
			t.Errorf("RED: ADR-0056 is missing required section %q.\n"+
				"Builder must include all four standard ADR sections.\nFile: %s", section, adrPath)
		}
	}
}

// TestC2_003_ADRDescribesAllFourSlices verifies that ADR-0056 describes all four
// implementation slices: S1 (wave engine / dag.Levels), S2 (preliminary-study
// phase), S3 (CLI driver / cmd_campaign), and S4 (documentation).
//
// // acs-predicate: config-check
// The slice descriptions are required content in the Decision section;
// checking their presence is a structural doc requirement.
//
// RED: file does not exist (FileContains fails on read error).
func TestC2_003_ADRDescribesAllFourSlices(t *testing.T) {
	root := acsassert.RepoRoot(t)
	// acs-predicate: config-check
	adrPath := filepath.Join(root, "docs", "architecture", "adr", "0056-advisor-driven-preliminary-study-cycle.md")
	sliceMarkers := []struct {
		slice   string
		keyword string
	}{
		{"S1 (wave engine)", "dag.Levels"},
		{"S2 (preliminary-study phase)", "preliminary-study"},
		{"S3 (CLI driver)", "cmd_campaign"},
		{"S4 (documentation)", "S4"},
	}
	for _, m := range sliceMarkers {
		if !acsassert.FileContains(t, adrPath, m.keyword) {
			t.Errorf("RED: ADR-0056 does not mention %q (required keyword for %s).\n"+
				"Builder must describe all four slices in the ADR.\nFile: %s",
				m.keyword, m.slice, adrPath)
		}
	}
}

// TestC2_004_CitationsFileExistsAndNonEmpty verifies that the companion
// campaign-planning-citations.md file exists, is git-tracked, and has at least
// 5 non-empty lines.
//
// BEHAVIORAL: the line-count check prevents an empty placeholder from satisfying
// the "non-empty" criterion.
//
// RED: file does not exist yet.
func TestC2_004_CitationsFileExistsAndNonEmpty(t *testing.T) {
	root := acsassert.RepoRoot(t)
	rel := filepath.Join("docs", "architecture", "campaign-planning-citations.md")
	abs := filepath.Join(root, rel)

	if !acsassert.FileExists(t, abs) {
		t.Fatalf("RED: %s missing on disk.\n"+
			"Builder must create docs/architecture/campaign-planning-citations.md.", rel)
	}
	_, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel)
	if code != 0 {
		t.Errorf("RED: %s exists but is not git-tracked.\nBuilder must `git add` the citations file.", rel)
	}

	raw, err := os.ReadFile(abs)
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	lineCount := 0
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) != "" {
			lineCount++
		}
	}
	if lineCount < 5 {
		t.Errorf("RED: %s has only %d non-empty line(s); need ≥ 5.\n"+
			"Builder must populate the citations file with meaningful content.", rel, lineCount)
	}
}

// TestC2_005_NoPlaceholderURLsInADR verifies that ADR-0056 does not contain
// placeholder URLs: no "example.com" and no "TODO" in a URL context.
//
// // acs-predicate: config-check
// Absence of placeholder patterns is a structural constraint on doc quality.
//
// RED: file does not exist (FileNotContains returns false on read error).
func TestC2_005_NoPlaceholderURLsInADR(t *testing.T) {
	root := acsassert.RepoRoot(t)
	// acs-predicate: config-check
	adrPath := filepath.Join(root, "docs", "architecture", "adr", "0056-advisor-driven-preliminary-study-cycle.md")
	if !acsassert.FileNotContains(t, adrPath, "example.com") {
		t.Errorf("RED: ADR-0056 contains placeholder URL \"example.com\".\n"+
			"Builder must replace all placeholder URLs with verified-live citations.\nFile: %s", adrPath)
	}
	if !acsassert.FileNotContains(t, adrPath, "TODO") {
		t.Errorf("RED: ADR-0056 contains \"TODO\" — unresolved placeholder.\n"+
			"Builder must replace all TODO-tagged content with verified information.\nFile: %s", adrPath)
	}
}
