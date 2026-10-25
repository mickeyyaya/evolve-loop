// Package consensusdispatch runs the cross-CLI consensus audit: it filters the voters, fans out one worker per CLI and aggregates by vote.
// See docs/architecture/packages/internal-consensusdispatch.md.
package consensusdispatch

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/capability"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

const (
	ExitOK            = 0
	ExitConsensusFAIL = 1
	ExitRuntimeErr    = 2
	ExitProfileErr    = 10
)

type Inputs struct {
	Cycle           string
	WorkspacePath   string
	ProfilePath     string
	PromptFile      string
	AdaptersDir     string
	DispatchDir     string
	ConsensusEnvOff bool

	TierFor func(cli string) (string, error)

	resolveDispatch func(dispatchDir, subcmd string, subArgs []string) (*exec.Cmd, error)
}

type Profile struct {
	ModelTierDefault string
	Enabled          bool
	CLIVoters        []string
	Quorum           int
	RequireMinTier   string
}

func Run(in Inputs, stdout, stderr io.Writer) int {
	logf := func(format string, args ...any) {
		fmt.Fprintf(stderr, "[consensus-dispatch] "+format+"\n", args...)
	}

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

	logf("validating voter capabilities (require_min_tier=%s)...", prof.RequireMinTier)
	tierFor := in.TierFor
	if tierFor == nil {
		tierFor = func(cli string) (string, error) {
			return capability.QualityTier(in.AdaptersDir, cli, nil)
		}
	}
	eligible, declared := filterEligible(prof.CLIVoters, prof.RequireMinTier, tierFor, stderr)
	logf("voters: %d declared, %d eligible (after tier filter)", declared, len(eligible))
	logf("eligible: %s", strings.Join(eligible, " "))

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

	workersDir := filepath.Join(in.WorkspacePath, "consensus-workers")
	if err := os.MkdirAll(workersDir, 0o755); err != nil {
		logf("FAIL: cannot create workers dir: %v", err)
		return ExitRuntimeErr
	}
	commandsTSV := filepath.Join(workersDir, ".commands.tsv")
	resultsTSV := filepath.Join(workersDir, ".results.tsv")
	evolveBin := resolveEvolveBin(in.DispatchDir)
	if evolveBin == "" {
		evolveBin = "evolve"
	}
	dispatch := in.resolveDispatch
	if dispatch == nil {
		dispatch = resolveNativeDispatch
	}
	tsv, workerCount := BuildCommandsTSV(eligible, in.ProfilePath, in.PromptFile,
		in.Cycle, workersDir, evolveBin, prof.ModelTierDefault)
	if skipped := driverlessVoters(eligible); len(skipped) > 0 {
		logf("WARN: skipped %d eligible voter(s) with no registered driver: %s", len(skipped), strings.Join(skipped, " "))
	}
	if workerCount < 2 {
		logf("FAIL: after filter, only %d workers ready (need ≥2)", workerCount)
		return ExitRuntimeErr
	}
	if err := os.WriteFile(commandsTSV, []byte(tsv), 0o644); err != nil {
		logf("FAIL: cannot write commands TSV: %v", err)
		return ExitRuntimeErr
	}

	logf("dispatching %d parallel cross-CLI workers...", workerCount)
	cmd, err := dispatch(in.DispatchDir, "fanout-dispatch", []string{commandsTSV, resultsTSV})
	if err != nil {
		logf("FAIL: %v", err)
		return ExitRuntimeErr
	}
	cmd.Stdout = stderr
	cmd.Stderr = stderr
	cmd.Env = os.Environ()
	_ = cmd.Run()
	fanoutRC := exitCodeFromErr(cmd.ProcessState)
	logf("fanout completed: rc=%d", fanoutRC)

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

	aggOutput := filepath.Join(in.WorkspacePath, phasecontract.ArtifactFilename("audit"))
	logf("aggregating via cross-cli-vote...")
	aggSubArgs := append([]string{"cross-cli-vote", aggOutput}, workerArtifacts...)
	aggCmd, err := dispatch(in.DispatchDir, "aggregator", aggSubArgs)
	if err != nil {
		logf("FAIL: %v", err)
		return ExitRuntimeErr
	}
	aggCmd.Stdout = stderr
	aggCmd.Stderr = stderr
	aggCmd.Env = os.Environ()
	_ = aggCmd.Run()
	aggRC := exitCodeFromErr(aggCmd.ProcessState)

	logf("DONE: consensus dispatch rc=%d; aggregate at %s", aggRC, aggOutput)
	return aggRC
}

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

func filterEligible(voters []string, requireMinTier string, tierFor func(string) (string, error), stderr io.Writer) (eligible []string, declaredCount int) {
	tiers := make(map[string]string, len(voters))
	for _, cli := range voters {
		declaredCount++
		tier, err := tierFor(cli)
		if err != nil || tier == "" {
			tier = "unknown"
		}
		tiers[cli] = tier
	}
	elig := FilterEligibleAgainstTiers(voters, tiers, requireMinTier)
	eligSet := make(map[string]bool, len(elig))
	for _, e := range elig {
		eligSet[e] = true
	}
	for _, cli := range voters {
		if !eligSet[cli] {
			fmt.Fprintf(stderr, "[consensus-dispatch]   excluded %s (tier=%s, require>=%s)\n", cli, tiers[cli], requireMinTier)
		}
	}
	return elig, declaredCount
}

func BuildCommandsTSV(eligible []string, profilePath, promptFile, cycle, workersDir, evolveBin, model string) (string, int) {
	var sb strings.Builder
	count := 0
	sorted := make([]string, len(eligible))
	copy(sorted, eligible)
	sort.Strings(sorted)
	for _, cli := range sorted {
		driver, ok := voterDriver(cli)
		if !ok {
			continue
		}
		artifact := filepath.Join(workersDir, cli+"-audit.md")
		stdout := filepath.Join(workersDir, cli+"-stdout.log")
		stderr := filepath.Join(workersDir, cli+"-stderr.log")
		cmd := fmt.Sprintf(
			"%s bridge launch --cli='%s' --profile='%s' --model='%s' --prompt-file='%s' --cycle='%s' --workspace='%s' --stdout-log='%s' --stderr-log='%s' --artifact='%s' --allow-bypass",
			evolveBin, driver, profilePath, model, promptFile, cycle, workersDir, stdout, stderr, artifact,
		)
		fmt.Fprintf(&sb, "%s\t%s\n", cli, cmd)
		count++
	}
	return sb.String(), count
}

func voterDriver(cli string) (string, bool) {
	driver, err := bridge.DriverFor(cli)
	if err != nil {
		return "", false
	}
	_, ok := bridge.LookupDriver(driver)
	return driver, ok
}

func driverlessVoters(eligible []string) []string {
	var out []string
	for _, cli := range eligible {
		if _, ok := voterDriver(cli); !ok {
			out = append(out, cli)
		}
	}
	return out
}

func exitCodeFromErr(ps *os.ProcessState) int {
	if ps == nil {
		return 1
	}
	if ps.Success() {
		return 0
	}
	return ps.ExitCode()
}
