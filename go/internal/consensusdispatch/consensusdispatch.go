// Package consensusdispatch ports legacy/scripts/dispatch/consensus-dispatch.sh.
//
// Cross-CLI consensus auditor dispatch (v8.54.0+). Reads a profile's
// consensus block and dispatches N parallel audit invocations, each under
// a DIFFERENT CLI. Aggregates results via aggregator.sh's cross-cli-vote
// merge mode (MAJORITY-PASS with FAIL-VETO).
//
// Until Phase 3b lands fanout-dispatch + aggregator + capability-check in Go,
// this package handles the deterministic prep (env validation, profile
// parsing, voter filtering, quorum adjustment, TSV build) and shells out to
// the bash dependents for the orchestration steps.
package consensusdispatch

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Exit code contract (matches consensus-dispatch.sh):
//   0  — PASS or WARN consensus
//   1  — FAIL (aggregator returned 1)
//   2  — runtime error (insufficient voters, missing inputs)
//  10  — profile validation error
const (
	ExitOK            = 0
	ExitConsensusFAIL = 1
	ExitRuntimeErr    = 2
	ExitProfileErr    = 10
)

// Inputs collects the env-var inputs required by consensus-dispatch.sh.
type Inputs struct {
	Cycle           string // required — cycle number
	WorkspacePath   string // required — .evolve/runs/cycle-N/
	ProfilePath     string // required — absolute path to profile JSON w/ .consensus
	PromptFile      string // required — path to audit prompt
	AdaptersDir     string // optional — defaults to <scripts>/cli_adapters/
	DispatchDir     string // optional — defaults to <scripts>/dispatch/
	ConsensusEnvOff bool   // EVOLVE_CONSENSUS_AUDIT=0 → refuse
}

// Profile mirrors the consensus block from a profile JSON.
type Profile struct {
	ModelTierDefault string
	Enabled          bool
	CLIVoters        []string
	Quorum           int
	RequireMinTier   string
}

// Run performs the full consensus-dispatch pipeline. stderr receives the
// human-readable [consensus-dispatch] log prefix lines; stdout is reserved
// for the final aggregator artifact path. Returns the bash-compatible exit
// code (0/1/2/10).
func Run(in Inputs, stdout, stderr io.Writer) int {
	logf := func(format string, args ...any) {
		fmt.Fprintf(stderr, "[consensus-dispatch] "+format+"\n", args...)
	}

	// ── input validation ──
	if in.Cycle == "" {
		logf("FAIL: CYCLE required")
		return ExitRuntimeErr
	}
	if in.WorkspacePath == "" {
		logf("FAIL: WORKSPACE_PATH required")
		return ExitRuntimeErr
	}
	if in.ProfilePath == "" {
		logf("PROFILE-ERROR: PROFILE_PATH required")
		return ExitProfileErr
	}
	if in.PromptFile == "" {
		logf("FAIL: PROMPT_FILE required")
		return ExitRuntimeErr
	}
	if _, err := os.Stat(in.ProfilePath); err != nil {
		logf("PROFILE-ERROR: profile not found: %s", in.ProfilePath)
		return ExitProfileErr
	}
	if _, err := os.Stat(in.PromptFile); err != nil {
		logf("FAIL: prompt file missing: %s", in.PromptFile)
		return ExitRuntimeErr
	}
	if err := os.MkdirAll(in.WorkspacePath, 0o755); err != nil {
		logf("FAIL: cannot create workspace: %v", err)
		return ExitRuntimeErr
	}
	if in.ConsensusEnvOff {
		logf("FAIL: EVOLVE_CONSENSUS_AUDIT=0 — refusing to run consensus dispatch (operator opt-out)")
		return ExitRuntimeErr
	}

	// ── profile parse ──
	prof, perr := ParseProfile(in.ProfilePath)
	if perr != nil {
		logf("PROFILE-ERROR: %v", perr)
		return ExitProfileErr
	}
	if !prof.Enabled {
		logf("PROFILE-ERROR: profile.consensus.enabled is false; set to true and re-run, or invoke standard dispatch_parallel")
		return ExitProfileErr
	}
	if len(prof.CLIVoters) == 0 {
		logf("PROFILE-ERROR: profile.consensus.cli_voters is empty")
		return ExitProfileErr
	}
	if prof.Quorum < 0 {
		logf("PROFILE-ERROR: profile.consensus.quorum must be integer")
		return ExitProfileErr
	}

	// ── voter eligibility filtering ──
	logf("validating voter capabilities (require_min_tier=%s)...", prof.RequireMinTier)
	eligible, declared := filterEligible(prof.CLIVoters, prof.RequireMinTier, in.AdaptersDir, stderr)
	logf("voters: %d declared, %d eligible (after tier filter)", declared, len(eligible))
	logf("eligible: %s", strings.Join(eligible, " "))

	// ── quorum adjustment ──
	effectiveQuorum := prof.Quorum
	if len(eligible) < effectiveQuorum {
		effectiveQuorum = (len(eligible) + 1) / 2
		logf("WARN: eligible count (%d) < declared quorum (%d); reducing quorum to ceil(%d / 2)",
			len(eligible), prof.Quorum, len(eligible))
		logf("  effective quorum: %d", effectiveQuorum)
	}
	if len(eligible) < 2 {
		logf("FAIL: consensus requires at least 2 eligible voters; got %d", len(eligible))
		return ExitRuntimeErr
	}

	// ── build commands TSV ──
	workersDir := filepath.Join(in.WorkspacePath, "consensus-workers")
	if err := os.MkdirAll(workersDir, 0o755); err != nil {
		logf("FAIL: cannot create workers dir: %v", err)
		return ExitRuntimeErr
	}
	commandsTSV := filepath.Join(workersDir, ".commands.tsv")
	resultsTSV := filepath.Join(workersDir, ".results.tsv")
	tsv, workerCount, terr := BuildCommandsTSV(eligible, in.ProfilePath, in.PromptFile,
		in.Cycle, workersDir, in.AdaptersDir, prof.ModelTierDefault)
	if terr != nil {
		logf("FAIL: %v", terr)
		return ExitRuntimeErr
	}
	if workerCount < 2 {
		logf("FAIL: after filter, only %d workers ready (need ≥2)", workerCount)
		return ExitRuntimeErr
	}
	if err := os.WriteFile(commandsTSV, []byte(tsv), 0o644); err != nil {
		logf("FAIL: cannot write commands TSV: %v", err)
		return ExitRuntimeErr
	}

	// ── shell-out to fanout-dispatch (native binary preferred; bash fallback) ──
	logf("dispatching %d parallel cross-CLI workers...", workerCount)
	cmd := resolveBashOrNative(in.DispatchDir, "fanout-dispatch", []string{commandsTSV, resultsTSV})
	cmd.Stdout = stderr
	cmd.Stderr = stderr
	cmd.Env = os.Environ()
	_ = cmd.Run()
	fanoutRC := exitCodeFromErr(cmd.ProcessState)
	logf("fanout completed: rc=%d", fanoutRC)

	// ── collect artifacts ──
	workerArtifacts := []string{}
	for _, cli := range eligible {
		artifact := filepath.Join(workersDir, cli+"-audit.md")
		if info, err := os.Stat(artifact); err == nil && info.Size() > 0 {
			workerArtifacts = append(workerArtifacts, artifact)
		} else {
			logf("WARN: %s produced no usable artifact; consensus may be reduced", cli)
		}
	}
	if len(workerArtifacts) == 0 {
		logf("FAIL: no worker artifacts produced; cannot aggregate")
		return ExitRuntimeErr
	}

	// ── shell-out to aggregator (native binary preferred; bash fallback) ──
	aggOutput := filepath.Join(in.WorkspacePath, "audit-report.md")
	logf("aggregating via cross-cli-vote...")
	aggSubArgs := append([]string{"cross-cli-vote", aggOutput}, workerArtifacts...)
	aggCmd := resolveBashOrNative(in.DispatchDir, "aggregator", aggSubArgs)
	aggCmd.Stdout = stderr
	aggCmd.Stderr = stderr
	aggCmd.Env = os.Environ()
	_ = aggCmd.Run()
	aggRC := exitCodeFromErr(aggCmd.ProcessState)

	logf("DONE: consensus dispatch rc=%d; aggregate at %s", aggRC, aggOutput)
	return aggRC
}

// ParseProfile reads a profile JSON file and extracts the consensus block.
func ParseProfile(path string) (Profile, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Profile{}, fmt.Errorf("read profile: %w", err)
	}
	var doc struct {
		ModelTierDefault string `json:"model_tier_default"`
		Consensus        struct {
			Enabled        bool     `json:"enabled"`
			CLIVoters      []string `json:"cli_voters"`
			Quorum         int      `json:"quorum"`
			RequireMinTier string   `json:"require_min_tier"`
		} `json:"consensus"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return Profile{}, fmt.Errorf("parse profile: %w", err)
	}
	p := Profile{
		ModelTierDefault: doc.ModelTierDefault,
		Enabled:          doc.Consensus.Enabled,
		CLIVoters:        doc.Consensus.CLIVoters,
		Quorum:           doc.Consensus.Quorum,
		RequireMinTier:   doc.Consensus.RequireMinTier,
	}
	if p.ModelTierDefault == "" {
		p.ModelTierDefault = "sonnet"
	}
	if p.RequireMinTier == "" {
		p.RequireMinTier = "hybrid"
	}
	return p, nil
}

// FilterEligibleAgainstTiers takes a slice of voter CLIs + their resolved
// quality tiers and returns the subset that meets requireMinTier. Pure
// function, deterministic — exposed for testing.
func FilterEligibleAgainstTiers(voters []string, tiers map[string]string, requireMinTier string) []string {
	out := []string{}
	for _, cli := range voters {
		tier := tiers[cli]
		switch requireMinTier {
		case "full":
			if tier == "full" {
				out = append(out, cli)
			}
		case "hybrid":
			if tier == "full" || tier == "hybrid" {
				out = append(out, cli)
			}
		case "degraded":
			if tier == "full" || tier == "hybrid" || tier == "degraded" {
				out = append(out, cli)
			}
		case "none", "":
			out = append(out, cli)
		default:
			out = append(out, cli)
		}
	}
	return out
}

// filterEligible runs the bash _capability-check.sh probe for each voter
// and filters by tier. Logs exclusions to stderr.
func filterEligible(voters []string, requireMinTier, adaptersDir string, stderr io.Writer) (eligible []string, declaredCount int) {
	capCheck := filepath.Join(adaptersDir, "_capability-check.sh")
	if _, err := os.Stat(capCheck); err != nil {
		// no capability check available — include all, mirror bash WARN behavior
		fmt.Fprintf(stderr, "[consensus-dispatch] WARN: capability-check missing; cannot validate voters — including all\n")
		out := make([]string, len(voters))
		copy(out, voters)
		return out, len(voters)
	}
	tiers := make(map[string]string, len(voters))
	for _, cli := range voters {
		declaredCount++
		tier := probeQualityTier(capCheck, cli)
		tiers[cli] = tier
	}
	elig := FilterEligibleAgainstTiers(voters, tiers, requireMinTier)
	// log exclusions
	for _, cli := range voters {
		excluded := true
		for _, e := range elig {
			if e == cli {
				excluded = false
				break
			}
		}
		if excluded {
			fmt.Fprintf(stderr, "[consensus-dispatch]   excluded %s (tier=%s, require>=%s)\n", cli, tiers[cli], requireMinTier)
		}
	}
	return elig, declaredCount
}

func probeQualityTier(capCheck, cli string) string {
	cmd := exec.Command("bash", capCheck, cli)
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	var doc struct {
		QualityTier string `json:"quality_tier"`
	}
	if jerr := json.Unmarshal(out, &doc); jerr != nil {
		return "unknown"
	}
	if doc.QualityTier == "" {
		return "unknown"
	}
	return doc.QualityTier
}

// BuildCommandsTSV constructs the worker dispatch TSV. Each line is
// <cli>\t<command-string>. Workers without an executable adapter are skipped.
func BuildCommandsTSV(eligible []string, profilePath, promptFile, cycle, workersDir, adaptersDir, model string) (string, int, error) {
	var sb strings.Builder
	count := 0
	// deterministic order
	sorted := make([]string, len(eligible))
	copy(sorted, eligible)
	sort.Strings(sorted)
	for _, cli := range sorted {
		adapter := filepath.Join(adaptersDir, cli+".sh")
		info, err := os.Stat(adapter)
		if err != nil || info.Mode()&0o111 == 0 {
			continue
		}
		artifact := filepath.Join(workersDir, cli+"-audit.md")
		stdout := filepath.Join(workersDir, cli+"-stdout.log")
		stderr := filepath.Join(workersDir, cli+"-stderr.log")
		cmd := fmt.Sprintf(
			"PROFILE_PATH='%s' RESOLVED_MODEL='%s' PROMPT_FILE='%s' CYCLE='%s' WORKSPACE_PATH='%s' STDOUT_LOG='%s' STDERR_LOG='%s' ARTIFACT_PATH='%s' bash '%s'",
			profilePath, model, promptFile, cycle, workersDir, stdout, stderr, artifact, adapter,
		)
		fmt.Fprintf(&sb, "%s\t%s\n", cli, cmd)
		count++
	}
	return sb.String(), count, nil
}

// exitCodeFromErr extracts the exit code from a finished os.ProcessState.
func exitCodeFromErr(ps *os.ProcessState) int {
	if ps == nil {
		return 1
	}
	if ps.Success() {
		return 0
	}
	return ps.ExitCode()
}

// MissingInputError represents a structural input error (compat alias).
var MissingInputError = errors.New("consensusdispatch: missing required input")
