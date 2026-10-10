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

const (
	TierHaiku  = "haiku"
	TierSonnet = "sonnet"
	TierOpus   = "opus"
)

type ResolveModelTierRequest struct {
	ProfilePath string
	Cycle       int

	ModelTierHint          string
	AuditorTierOverride    string
	DiffComplexityDisabled bool
	WorktreePath           string

	ProjectRoot string
}

type ResolveModelTierOptions struct {
	ReadProfile    func(path string) (string, error)
	ReadState      func(projectRoot string) (string, error)
	DiffComplexity func(worktree string) (string, error)
}

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
		if req.AuditorTierOverride != "" {
			return req.AuditorTierOverride, nil
		}
		streak := readConsecutiveSuccesses(opts.ReadState, req.ProjectRoot)
		if streak < 1 {
			return TierOpus, nil
		}
		if !req.DiffComplexityDisabled {
			tier, _ := opts.DiffComplexity(req.WorktreePath)
			if tier == "trivial" {
				return TierSonnet, nil
			}
		}
	}

	defaultTier := matchField(profileBody, reFieldTierDefault)
	if defaultTier == "" {
		return "", fmt.Errorf("subagent/modeltier: profile %s missing model_tier_default", req.ProfilePath)
	}
	return applyModelTierOverride(defaultTier, profileBody, req), nil
}

func applyModelTierOverride(base, profileBody string, req ResolveModelTierRequest) string {
	var p profiles.Profile
	if err := json.Unmarshal([]byte(profileBody), &p); err != nil {
		return base
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
	if p.ModelTierEnvelope != nil && p.ModelTierEnvelope.Max != "" {
		if maxRank := policy.TierRank(p.ModelTierEnvelope.Max); maxRank > 0 &&
			policy.TierRank(override) > maxRank {
			override = p.ModelTierEnvelope.Max
		}
	}
	if policy.TierRank(override) > policy.TierRank(base) {
		return override
	}
	return base
}

func activeSituation(req ResolveModelTierRequest) string {
	if req.Cycle <= 1 {
		return "cycle_1_or_low_goal"
	}
	return ""
}

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
