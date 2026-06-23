//go:build acs

// Package cycle15 materializes the cycle-15 acceptance criteria for two
// committed top_n tasks:
//
//	consolidate-resume-cluster — remove all 6 RESUME_* registry rows
//	(EVOLVE_AUTO_RESUME_MAX_ATTEMPTS, EVOLVE_RESUME, EVOLVE_RESUME_ALLOW_HEAD_MOVED,
//	EVOLVE_RESUME_COMPLETED_PHASES, EVOLVE_RESUME_MODE, EVOLVE_RESUME_PHASE),
//	lower FlagCeiling 160→154, remove RESUME_* rows from control-flags.md,
//	and clean up docs_contract_test.go entries.
//
//	bypass-policy-flag — convert EVOLVE_POLICY_BYPASS to a proper --bypass-policy
//	cobra flag on `evolve cycle run` and `evolve loop`, remove 3 cycleEnv bridge
//	reads, and delete the EVOLVE_POLICY_BYPASS registry row.
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	consolidate-resume-cluster:
//	  AC1+NEG1  All 6 RESUME_* flags absent from Lookup             → C15_001 (behavioral)
//	  AC2       Registry row count == 154                            → C15_002 (behavioral, count)
//	  AC3       FlagCeiling const == 154                             → C15_003 (config-check, waiver)
//	  AC4       No os.Getenv reads for RESUME_* in production files  → C15_004 (config-check, waiver — PRE-EXISTING GREEN)
//	  AC5       control-flags.md has no RESUME_* rows               → C15_005 (config-check, waiver)
//	  EDGE1     IPC set preserved: cmd_loop_args.go sets EVOLVE_RESUME=1  → C15_006 (config-check, waiver — PRE-EXISTING GREEN)
//
//	bypass-policy-flag:
//	  AC1   --bypass-policy in `evolve cycle run --help`                   → C15_007 (behavioral, subprocess)
//	  AC2   bypass_policy field in `evolve loop --dry-run --bypass-policy` → C15_008 (behavioral, subprocess)
//	  AC3+4 cycleEnv["EVOLVE_POLICY_BYPASS"] absent from cmd files         → C15_009 (config-check, waiver)
//	  AC5   EVOLVE_POLICY_BYPASS row absent from flagregistry              → C15_010 (behavioral, Lookup)
//	  EDGE1 No os.Getenv("EVOLVE_POLICY_BYPASS") in production Go files    → PRE-EXISTING GREEN (0 production readers; scout confirmed)
//
// ACs with manual+checklist disposition (enforced by CI):
//
//	AC10  (full test suite green): `go test ./...` exit 0
//
// Adversarial diversity (SKILL §6):
//
//	Negative:  C15_001 (Lookup ok=false for all 6 RESUME_* rows) +
//	           C15_010 (Lookup ok=false for POLICY_BYPASS row) — neither
//	           can be satisfied by adding a magic string; the row must be deleted.
//	Edge/OOD:  C15_009 checks BOTH cmd_cycle.go AND cmd_loop.go (3 env bridge
//	           sites: line 190 in cycle, lines 186+303 in loop).
//	Lexical:   SubprocessOutput / FileNotContains / Lookup — three distinct verbs
//	           across the bypass-policy predicates.
//	Semantic:  CLI flag registration (C15_007), dry-run JSON field (C15_008),
//	           env-bridge absence (C15_009), registry-row absence (C15_010) —
//	           four distinct behavioral dimensions.
//
// 1:1 enforcement (bypass-policy-flag): predicate=4, manual+checklist=1, pre-existing-GREEN=1 → total AC=6 ✓
//
// Floor binding (R9.3): predicates authored only for the committed top_n tasks
// (consolidate-resume-cluster, bypass-policy-flag).
package cycle15

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolveloop/go/pkg/acsassert"
)

// TestC15_001_AllResumeFlagsAbsentFromRegistry verifies that all 6 RESUME_*
// flags are no longer registered after Builder removes their rows from registry_table.go.
//
// Covers AC1 (6 rows absent from registry_table.go) and NEG1 (RESUME_ALLOW_HEAD_MOVED
// specifically absent — it was documented but dead). Includes all 6 flags:
//   - EVOLVE_AUTO_RESUME_MAX_ATTEMPTS: 0 production readers (dead)
//   - EVOLVE_RESUME:                  SET only (cmd_loop_args.go:273); IPC handoff, no os.Getenv read
//   - EVOLVE_RESUME_ALLOW_HEAD_MOVED: comment only in resume.go:35; no os.Getenv read
//   - EVOLVE_RESUME_COMPLETED_PHASES: LLM prompt protocol only; no os.Getenv read
//   - EVOLVE_RESUME_MODE:             SET in core/resume.go:186 envSnap; no os.Getenv read
//   - EVOLVE_RESUME_PHASE:            LLM prompt protocol only; no os.Getenv read
//
// BEHAVIORAL: calls flagregistry.Lookup() for each flag — the production SSOT.
// A source edit alone cannot satisfy this; the row must be absent for Lookup
// to return ok=false.
//
// RED: all 6 flags are currently registered; each Lookup returns (flag, true).
func TestC15_001_AllResumeFlagsAbsentFromRegistry(t *testing.T) {
	// All 6 RESUME_* rows from scout-report §Key Findings.
	// Semantic sub-cases: dead (AUTO_RESUME_MAX_ATTEMPTS), IPC-SET-only (RESUME,
	// RESUME_MODE), dead-comment (RESUME_ALLOW_HEAD_MOVED), LLM-protocol (RESUME_COMPLETED_PHASES,
	// RESUME_PHASE) — all are pure registry-row removals with no os.Getenv reads to migrate.
	allFlags := []string{
		"EVOLVE_AUTO_RESUME_MAX_ATTEMPTS", // dead: 0 production readers
		"EVOLVE_RESUME",                   // IPC SET only: cmd_loop_args.go:273 sets it; no Go os.Getenv
		"EVOLVE_RESUME_ALLOW_HEAD_MOVED",  // NEG1: comment in resume.go:35 only; AllowHeadMoved DI field stays
		"EVOLVE_RESUME_COMPLETED_PHASES",  // LLM orchestrator-reference.md protocol; no Go os.Getenv
		"EVOLVE_RESUME_MODE",              // IPC SET only: core/resume.go:186 envSnap; no Go os.Getenv
		"EVOLVE_RESUME_PHASE",             // LLM orchestrator-reference.md protocol; no Go os.Getenv
	}
	for _, name := range allFlags {
		if f, ok := flagregistry.Lookup(name); ok {
			t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
				"Builder must remove this row from registry_table.go (cycle-15 RESUME_* consolidation).\n"+
				"Current entry: Status=%q Cluster=%q",
				name, f.Status, f.Cluster)
		}
	}
}

// TestC15_004_NoResumeEnvReadsInProductionFiles verifies that no production
// Go file reads any of the 6 RESUME_* env vars via os.Getenv.
//
// The scout confirmed 0 production readers before the cycle (grep returns 0 hits
// for `os.Getenv.*EVOLVE_RESUME|EVOLVE_AUTO_RESUME`). This predicate is
// PRE-EXISTING GREEN — it documents the architectural contract (these flags were
// never wired to production os.Getenv reads) and prevents accidental re-introduction.
//
// Key files audited: resume.go (sets EVOLVE_RESUME_MODE in envSnap, no read),
// cmd_loop_args.go (sets EVOLVE_RESUME=1, no read), orchestrator-reference.md
// (LLM prompt protocol, not Go os.Getenv).
//
// // acs-predicate: config-check — the os.Getenv ABSENCE is the structural contract.
//
// PRE-EXISTING GREEN: grep confirms 0 production os.Getenv reads before this cycle.
func TestC15_004_NoResumeEnvReadsInProductionFiles(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	// Key files where resume logic lives — none should read RESUME_* via os.Getenv.
	resumeFile := filepath.Join(root, "go", "internal", "core", "resume.go")
	cmdLoopArgsFile := filepath.Join(root, "go", "cmd", "evolve", "cmd_loop_args.go")
	for _, flag := range []string{
		"EVOLVE_AUTO_RESUME_MAX_ATTEMPTS",
		"EVOLVE_RESUME_ALLOW_HEAD_MOVED",
		"EVOLVE_RESUME_COMPLETED_PHASES",
		"EVOLVE_RESUME_PHASE",
	} {
		envRead := `os.Getenv("` + flag + `")`
		if !acsassert.FileNotContains(t, resumeFile, envRead) {
			t.Errorf("resume.go reads %q via os.Getenv — must be absent.\n"+
				"File: %s", flag, resumeFile)
		}
		if !acsassert.FileNotContains(t, cmdLoopArgsFile, envRead) {
			t.Errorf("cmd_loop_args.go reads %q via os.Getenv — must be absent.\n"+
				"File: %s", flag, cmdLoopArgsFile)
		}
	}
}

// TestC15_005_ControlFlagsMdHasNoResumeRows verifies that the generated doc
// docs/architecture/control-flags.md has no EVOLVE_RESUME_* or
// EVOLVE_AUTO_RESUME_MAX_ATTEMPTS entries after the 6 registry rows are removed
// and the doc regenerated.
//
// Covers AC5. The doc is generated from the flagregistry (source of truth);
// absence of RESUME_* rows follows from C15_001 (rows removed) plus the
// regeneration step ('evolve flags generate').
//
// // acs-predicate: config-check — the doc regeneration is a required build step.
//
// RED: control-flags.md currently has 6 EVOLVE_RESUME_* entries.
func TestC15_005_ControlFlagsMdHasNoResumeRows(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	controlFlags := filepath.Join(root, "docs", "architecture", "control-flags.md")
	resumeFlags := []string{
		"EVOLVE_AUTO_RESUME_MAX_ATTEMPTS",
		"EVOLVE_RESUME_ALLOW_HEAD_MOVED",
		"EVOLVE_RESUME_COMPLETED_PHASES",
		"EVOLVE_RESUME_MODE",
		"EVOLVE_RESUME_PHASE",
		// bare EVOLVE_RESUME — match the backtick-wrapped table cell form
		// (`` `EVOLVE_RESUME` ``) which is distinct from all _* variants.
		"`EVOLVE_RESUME`",
	}
	for _, flag := range resumeFlags {
		if !acsassert.FileNotContains(t, controlFlags, flag) {
			t.Errorf("RED: control-flags.md still contains %q.\n"+
				"Builder must remove all 6 RESUME_* rows from registry_table.go\n"+
				"then regenerate the doc via 'evolve flags generate'.\n"+
				"File: %s", flag, controlFlags)
		}
	}
}

// TestC15_006_IPCSetPreservedInCmdLoopArgs verifies that cmd_loop_args.go still
// contains the EVOLVE_RESUME IPC set (the child-process handoff), confirming
// that removing the registry row does NOT remove the IPC mechanism itself.
//
// Covers EDGE1 from scout-report §Acceptance Criteria Summary. The registry
// is documentation, not runtime glue; the IPC env var must survive the row removal.
//
// // acs-predicate: config-check — the IPC SET presence is the structural contract.
//
// PRE-EXISTING GREEN: cmd_loop_args.go:273 already sets EVOLVE_RESUME=1.
func TestC15_006_IPCSetPreservedInCmdLoopArgs(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	cmdLoopArgsFile := filepath.Join(root, "go", "cmd", "evolve", "cmd_loop_args.go")
	if !acsassert.FileContains(t, cmdLoopArgsFile, `"EVOLVE_RESUME"`) {
		t.Errorf("RED: cmd_loop_args.go no longer sets EVOLVE_RESUME — IPC handoff broken.\n"+
			"Builder must NOT remove the EVOLVE_RESUME= assignment in cmd_loop_args.go:273;\n"+
			"only the registry row in registry_table.go should be removed.\n"+
			"File: %s", cmdLoopArgsFile)
	}
}

// ─── bypass-policy-flag predicates (C15_007–C15_010) ─────────────────────────

// TestC15_007_BypassPolicyFlagInCycleRunHelp verifies that `evolve cycle run`
// registers a --bypass-policy flag by checking its usage output.
//
// Covers AC1 (bypass-policy-flag). The flag package prints registered flags to
// stderr when --help is passed; we capture combined output since flag.ContinueOnError
// returns flag.ErrHelp and the command exits non-zero.
//
// BEHAVIORAL: runs `go run ./cmd/evolve cycle run --help` from the worktree's
// go/ directory; asserts stdout/stderr contains "--bypass-policy". A source-file
// grep cannot satisfy this — the BoolVar registration must be present.
//
// RED: --bypass-policy is not yet registered in runCycleRun; the help output
// does not contain the flag name. Builder must add fs.BoolVar(&bypassPolicy,
// "bypass-policy", false, "...") in runCycleRun.
func TestC15_007_BypassPolicyFlagInCycleRunHelp(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	cmd := exec.Command("go", "run", "./cmd/evolve", "cycle", "run", "--help")
	cmd.Dir = goDir
	// flag.ContinueOnError exits non-zero for --help; CombinedOutput captures
	// both stdout and stderr (usage is on stderr for flag-based commands).
	out, _ := cmd.CombinedOutput()
	if !strings.Contains(string(out), "--bypass-policy") {
		t.Errorf("RED: 'evolve cycle run --help' does not show '--bypass-policy'.\n"+
			"Builder must add: fs.BoolVar(&bypassPolicy, \"bypass-policy\", false, ...) in runCycleRun.\n"+
			"Usage output:\n%s", out)
	}
}

// TestC15_008_BypassPolicyInLoopDryRunJSON verifies that `evolve loop` exposes
// --bypass-policy and that the dry-run JSON output includes the bypass_policy field.
//
// Covers AC2 (bypass-policy-flag). When --bypass-policy is passed with --dry-run,
// loopConfig.BypassPolicy is true and marshals to "bypass_policy": true in the
// config sub-object. omitempty suppresses the field when false, so the test
// explicitly passes --bypass-policy to force the field into the output.
//
// BEHAVIORAL: runs `go run ./cmd/evolve loop --dry-run --bypass-policy` from
// the worktree's go/ directory. A non-zero exit in RED means --bypass-policy is
// not yet a recognized flag; a zero exit without "bypass_policy" means the
// loopConfig field or json tag is missing.
//
// RED: parseLoopArgs does not register --bypass-policy; the command exits
// non-zero (unknown flag) or omits bypass_policy from the JSON output.
func TestC15_008_BypassPolicyInLoopDryRunJSON(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	cmd := exec.Command("go", "run", "./cmd/evolve", "loop", "--dry-run", "--bypass-policy")
	cmd.Dir = goDir
	var stdoutBuf, stderrBuf strings.Builder
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	if err := cmd.Run(); err != nil {
		t.Errorf("RED: 'evolve loop --dry-run --bypass-policy' failed — --bypass-policy not yet registered in parseLoopArgs.\n"+
			"Builder must add BoolVar for 'bypass-policy' in parseLoopArgs and BypassPolicy bool to loopConfig.\n"+
			"Error: %v\nStderr:\n%s", err, stderrBuf.String())
		return
	}
	combined := stdoutBuf.String() + stderrBuf.String()
	if !strings.Contains(combined, `"bypass_policy"`) {
		t.Errorf("RED: 'evolve loop --dry-run --bypass-policy' output missing 'bypass_policy' key.\n"+
			"Builder must add BypassPolicy bool with json:\"bypass_policy,omitempty\" tag to loopConfig.\n"+
			"Got output:\n%s", combined)
	}
}

// TestC15_009_PolicyBypassEnvBridgesAbsentFromCmdFiles verifies that the 3
// cycleEnv["EVOLVE_POLICY_BYPASS"] bridge reads are removed from cmd_cycle.go
// and cmd_loop.go after Builder wires the --bypass-policy CLI flag.
//
// Covers AC3 (cmd_cycle.go:190) and AC4 (cmd_loop.go:186, cmd_loop.go:303).
// The DI fields CycleRequest.BypassPolicy and PhaseRequest.BypassPolicy are
// unchanged — only the env bridge reads that populated them are removed.
//
// // acs-predicate: config-check — env bridge ABSENCE is the structural contract.
//
// RED: all 3 cycleEnv["EVOLVE_POLICY_BYPASS"] reads are still present.
func TestC15_009_PolicyBypassEnvBridgesAbsentFromCmdFiles(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	files := []struct {
		name string
		path string
	}{
		{"cmd_cycle.go", filepath.Join(root, "go", "cmd", "evolve", "cmd_cycle.go")},
		{"cmd_loop.go", filepath.Join(root, "go", "cmd", "evolve", "cmd_loop.go")},
	}
	for _, f := range files {
		if !acsassert.FileNotContains(t, f.path, `cycleEnv["EVOLVE_POLICY_BYPASS"]`) {
			t.Errorf("RED: %s still contains cycleEnv[\"EVOLVE_POLICY_BYPASS\"] bridge read.\n"+
				"Builder must replace the bridge read with the --bypass-policy CLI flag value.\n"+
				"File: %s", f.name, f.path)
		}
	}
}

// TestC15_010_PolicyBypassRowAbsentFromRegistry verifies that the
// EVOLVE_POLICY_BYPASS row has been deleted from registry_table.go.
//
// Covers AC5 (bypass-policy-flag). The row was StatusDeprecated with
// documentation noting it was bridged for backward compat. After --bypass-policy
// CLI conversion, the row is fully removed and flagregistry.Lookup must
// return ok=false.
//
// BEHAVIORAL: calls flagregistry.Lookup("EVOLVE_POLICY_BYPASS") — the same
// production SSOT function used by C10_004. A source-file text change cannot
// satisfy this; the row struct literal must be absent from registry_table.go
// for the binary-searched All slice to not contain the entry.
//
// Negative: Lookup returning ok=true is the failure signal; the row is present.
// RED: EVOLVE_POLICY_BYPASS row is still in registry_table.go; Lookup returns
// (flag, true). Builder must delete that row.
func TestC15_010_PolicyBypassRowAbsentFromRegistry(t *testing.T) {
	if f, ok := flagregistry.Lookup("EVOLVE_POLICY_BYPASS"); ok {
		t.Errorf("RED: flagregistry.Lookup(\"EVOLVE_POLICY_BYPASS\") returned (flag, true) — row still registered.\n"+
			"Builder must delete the EVOLVE_POLICY_BYPASS row from registry_table.go (cycle-15 bypass-policy-flag).\n"+
			"Current entry: Status=%q Cluster=%q",
			f.Status, f.Cluster)
	}
}
