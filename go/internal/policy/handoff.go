package policy

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

var knownHandoffPairs = []string{
	"build-audit",
	"intent-scout",
	"scout-triage",
	"tdd-build",
	"triage-tdd",
}

var judgedHandoffPairs = []string{"triage-tdd"}

func JudgedHandoffPairs() []string { return slices.Clone(judgedHandoffPairs) }

type HandoffPairConfig struct {
	Stage     string
	JudgeTier string
}

type HandoffConfig map[string]HandoffPairConfig

type handoffPairJSON struct {
	Stage     *string `json:"stage"`
	JudgeTier *string `json:"judge_tier"`
}

type HandoffPolicy struct {
	pairs map[string]HandoffPairConfig
}

func (h *HandoffPolicy) UnmarshalJSON(raw []byte) error {
	var rawPairs map[string]json.RawMessage
	if err := decodeStrict(raw, &rawPairs); err != nil {
		return fmt.Errorf("handoff: %w", err)
	}

	knownMap := make(map[string]bool, len(knownHandoffPairs))
	for _, name := range knownHandoffPairs {
		knownMap[name] = true
	}

	pairs := make(map[string]HandoffPairConfig, len(rawPairs))
	for pairName, rawPair := range rawPairs {
		if !knownMap[pairName] {
			return fmt.Errorf("handoff: unknown pair %q; known pairs are %s", pairName, strings.Join(knownHandoffPairs, ", "))
		}

		var decoded handoffPairJSON
		if err := decodeStrict(rawPair, &decoded); err != nil {
			return fmt.Errorf("handoff: pair %q: %w", pairName, err)
		}

		stage := "shadow"
		if decoded.Stage != nil {
			if *decoded.Stage != "shadow" && *decoded.Stage != "enforce" {
				return fmt.Errorf("handoff: pair %q: stage %q must be %q or %q", pairName, *decoded.Stage, "shadow", "enforce")
			}
			stage = *decoded.Stage
		}

		judgeTier := ""
		if decoded.JudgeTier != nil {
			judgeTier = *decoded.JudgeTier
		}
		if err := checkJudgeTier(pairName, judgeTier); err != nil {
			return err
		}

		pairs[pairName] = HandoffPairConfig{
			Stage:     stage,
			JudgeTier: judgeTier,
		}
	}

	h.pairs = pairs
	return nil
}

func checkJudgeTier(pair, tier string) error {
	judgeTierNames := slices.DeleteFunc(TierNames(), func(name string) bool { return name == "fast" })
	switch {
	case tier == "":
		return nil
	case !slices.Contains(judgedHandoffPairs, pair):
		return fmt.Errorf("handoff: pair %q: judge_tier is set but the pair has no judge that the reviewer checks; judged pairs are %s", pair, strings.Join(judgedHandoffPairs, ", "))
	case !slices.Contains(judgeTierNames, tier):
		return fmt.Errorf("handoff: pair %q: judge_tier %q must be one of %s", pair, tier, strings.Join(judgeTierNames, ", "))
	}
	return nil
}

func (p Policy) HandoffConfig() HandoffConfig {
	cfg := make(HandoffConfig, len(knownHandoffPairs))
	for _, name := range knownHandoffPairs {
		cfg[name] = HandoffPairConfig{Stage: "shadow"}
	}
	if p.Handoff == nil || p.Handoff.pairs == nil {
		return cfg
	}
	for name, pair := range p.Handoff.pairs {
		cfg[name] = pair
	}
	return cfg
}
