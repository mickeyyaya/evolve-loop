package subagent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// ModelTier names recognized by the resolver. Adapters accept these as -m
// values; the resolver itself does not validate against the LLM provider.
const (
	TierHaiku  = "haiku"
	TierSonnet = "sonnet"
	TierOpus   = "opus"
)

// ResolveModelTierRequest captures every input the bash resolve_model_tier
// function reads from the environment + filesystem. Tests inject pure values;
// production code uses NewResolveModelTierRequestFromEnv.
type ResolveModelTierRequest struct {
	ProfilePath string // path to .evolve/profiles/<agent>.json
	Cycle       int    // current cycle (reserved for future tier rules)

	// Env overrides — empty string means "unset".
	ModelTierHint          string // MODEL_TIER_HINT — wins over everything
	AuditorTierOverride    string // EVOLVE_AUDITOR_TIER_OVERRIDE — wins inside auditor branch
	DiffComplexityDisabled bool   // EVOLVE_DIFF_COMPLEXITY_DISABLE=1
	WorktreePath           string // WORKTREE_PATH — passed to DiffComplexity callable

	// ProjectRoot is where .evolve/state.json lives (mastery streak source).
	ProjectRoot string
}

// ResolveModelTierOptions injects all the filesystem + sub-process seams the
// bash version shelled out to. Production: defaults; tests: in-memory stubs.
type ResolveModelTierOptions struct {
	// ReadProfile returns the contents of the profile JSON at path. Defaults
	// to os.ReadFile.
	ReadProfile func(path string) (string, error)
	// ReadState returns the contents of <projectRoot>/.evolve/state.json,
	// or ("", os.ErrNotExist) when absent. Defaults to os.ReadFile.
	ReadState func(projectRoot string) (string, error)
	// DiffComplexity returns "trivial" / "standard" / "complex" / "" for the
	// given worktree. Empty string ⇒ tier unknown ⇒ fall through to profile
	// default. Defaults to a no-op that returns "" (no diff-complexity helper
	// in Go yet; bash callers still drive that path).
	DiffComplexity func(worktree string) (string, error)
}

// ResolveModelTier mirrors resolve_model_tier in
// legacy/scripts/dispatch/subagent-run.sh (lines 189–261). Precedence:
//
//  1. MODEL_TIER_HINT wins for every agent.
//  2. For auditor only:
//     a. EVOLVE_AUDITOR_TIER_OVERRIDE wins inside auditor.
//     b. consecutiveSuccesses < 1 (from .evolve/state.json) ⇒ opus.
//     c. EVOLVE_DIFF_COMPLEXITY_DISABLE != "1" AND DiffComplexity returns
//     "trivial" ⇒ sonnet.
//     d. Otherwise fall through to profile.model_tier_default.
//  3. For non-auditor agents: profile.model_tier_default.
//
// Returns (tier, err). err is non-nil only when the profile is unreadable or
// the JSON shape is missing model_tier_default — bash treats those as fail.
func ResolveModelTier(req ResolveModelTierRequest, opts ResolveModelTierOptions) (string, error) {
	if opts.ReadProfile == nil {
		opts.ReadProfile = defaultReadProfile
	}
	if opts.ReadState == nil {
		opts.ReadState = defaultReadState
	}
	if opts.DiffComplexity == nil {
		opts.DiffComplexity = func(string) (string, error) { return "", nil }
	}

	// Rule 1: MODEL_TIER_HINT wins.
	if req.ModelTierHint != "" {
		return req.ModelTierHint, nil
	}

	profileBody, err := opts.ReadProfile(req.ProfilePath)
	if err != nil {
		return "", fmt.Errorf("subagent/modeltier: read profile %s: %w", req.ProfilePath, err)
	}

	role := matchField(profileBody, reFieldRole)
	if role == "" {
		role = matchField(profileBody, reFieldName)
	}

	if role == "auditor" {
		// Rule 2a.
		if req.AuditorTierOverride != "" {
			return req.AuditorTierOverride, nil
		}
		// Rule 2b: mastery gate.
		streak := readConsecutiveSuccesses(opts.ReadState, req.ProjectRoot)
		if streak < 1 {
			return TierOpus, nil
		}
		// Rule 2c: diff complexity (only when not disabled).
		if !req.DiffComplexityDisabled {
			tier, _ := opts.DiffComplexity(req.WorktreePath)
			if tier == "trivial" {
				return TierSonnet, nil
			}
			// "standard", "complex", or unknown — fall through.
		}
		// Rule 2d: fall through to profile default.
	}

	// Rule 3 (and auditor 2d): profile.model_tier_default.
	defaultTier := matchField(profileBody, reFieldTierDefault)
	if defaultTier == "" {
		return "", fmt.Errorf("subagent/modeltier: profile %s missing model_tier_default", req.ProfilePath)
	}
	return applyModelTierOverride(defaultTier, profileBody, req), nil
}

// applyModelTierOverride consumes profile.model_tier_overrides: when the
// request's active situation matches an override key, the override tier is
// clamped to the profile's envelope max (reusing policy.TierRank — never a new
// rank table) and applied as a FLOOR over the base tier (max(base, clamped) by
// rank). Vocabulary stays abstract. An empty/nil override map, an inactive
// situation, an absent key, or an unparseable body all leave base unchanged.
func applyModelTierOverride(base, profileBody string, req ResolveModelTierRequest) string {
	var p profiles.Profile
	if err := json.Unmarshal([]byte(profileBody), &p); err != nil {
		return base // unmodeled/invalid body ⇒ base tier stands (defensive)
	}
	if len(p.ModelTierOverrides) == 0 {
		return base
	}
	situation := activeSituation(req)
	if situation == "" {
		return base
	}
	override := strings.TrimSpace(p.ModelTierOverrides[situation])
	if override == "" {
		return base
	}
	// Clamp to the envelope max (skip when max is unset or unclassifiable).
	if p.ModelTierEnvelope != nil && p.ModelTierEnvelope.Max != "" {
		if maxRank := policy.TierRank(p.ModelTierEnvelope.Max); maxRank > 0 &&
			policy.TierRank(override) > maxRank {
			override = p.ModelTierEnvelope.Max
		}
	}
	// Floor: apply only when the override outranks the base tier.
	if policy.TierRank(override) > policy.TierRank(base) {
		return override
	}
	return base
}

// activeSituation maps real request signals to a model_tier_overrides key.
// Primary (only currently plumbed) producer: cycle_1_or_low_goal fires for the
// first cycle. Additional situation keys (audit_retry_2plus, cold_start, …)
// remain inert until their producer signals are plumbed — see builder-notes.
func activeSituation(req ResolveModelTierRequest) string {
	if req.Cycle <= 1 {
		return "cycle_1_or_low_goal"
	}
	return ""
}

// readConsecutiveSuccesses returns the streak count from
// .evolve/state.json, defaulting to 0 on any error (missing file, bad JSON,
// missing field). Mirrors bash's `grep -o '"consecutiveSuccesses":[0-9]*'`
// approach — defensive, no jq dependency.
func readConsecutiveSuccesses(reader func(string) (string, error), projectRoot string) int {
	body, err := reader(projectRoot)
	if err != nil {
		return 0
	}
	m := consecutiveSuccessesRE.FindStringSubmatch(body)
	if len(m) < 2 {
		return 0
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0
	}
	return n
}

var (
	consecutiveSuccessesRE  = regexp.MustCompile(`"consecutiveSuccesses"\s*:\s*([0-9]+)`)
	reFieldCLI              = regexp.MustCompile(`"cli"\s*:\s*"([^"]*)"`)
	reFieldOutputArtifact   = regexp.MustCompile(`"output_artifact"\s*:\s*"([^"]*)"`)
	reFieldRole             = regexp.MustCompile(`"role"\s*:\s*"([^"]*)"`)
	reFieldName             = regexp.MustCompile(`"name"\s*:\s*"([^"]*)"`)
	reFieldTierDefault      = regexp.MustCompile(`"model_tier_default"\s*:\s*"([^"]*)"`)
	reFieldParallelEligible = regexp.MustCompile(`"parallel_eligible"\s*:\s*(true|false)`)
	reFieldCtxTokens        = regexp.MustCompile(`"context_clear_trigger_tokens"\s*:\s*([0-9]+)`)
)

// matchField returns the first capture from a precompiled JSON-field regexp.
func matchField(body string, re *regexp.Regexp) string {
	m := re.FindStringSubmatch(body)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}

func defaultReadProfile(path string) (string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func defaultReadState(projectRoot string) (string, error) {
	body, err := os.ReadFile(filepath.Join(projectRoot, ".evolve", "state.json"))
	if err != nil {
		return "", err
	}
	return string(body), nil
}
