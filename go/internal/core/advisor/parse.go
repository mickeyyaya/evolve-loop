package advisor

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// parseFailure carries fields.cause out of the parsers, which stay Center-free because resume,
// replay and the routing-eval corpus call them and must emit nothing.
type parseFailure struct {
	cause string // no_json | invalid_json | empty
	err   error
}

func (p *parseFailure) Error() string { return p.err.Error() }
func (p *parseFailure) Unwrap() error { return p.err }

func (a *Advisor) warnUnparseable(in router.RouteInput, d decision, err error, resp LaunchResponse) {
	cause := causeInvalidJSON
	var pf *parseFailure
	if errors.As(err, &pf) {
		cause = pf.cause
	}
	a.warn(in, d, CodeResponseUnparseable, err.Error(), map[string]string{
		"step": stepParse, "cause": cause, "stdout_bytes": strconv.Itoa(len(resp.Stdout)), "artifact": d.artifactFile(),
	})
}

// ParseProposal decodes the last balanced JSON object in the response; an absent, malformed or empty proposal is an error.
func ParseProposal(stdout string) (*router.Proposal, error) {
	start, end, ok := LastBalancedSpan(stdout, '{', '}')
	if !ok {
		return nil, &parseFailure{cause: causeNoJSON, err: fmt.Errorf("no JSON object in proposer output")}
	}
	var prop router.Proposal
	if err := json.Unmarshal([]byte(stdout[start:end+1]), &prop); err != nil {
		return nil, &parseFailure{cause: causeInvalidJSON, err: fmt.Errorf("parse proposal: %w", err)}
	}
	if prop.NextPhase == "" && len(prop.InsertPhases) == 0 &&
		prop.RecoveryAction == "" && prop.LearningRichness == "" {
		return nil, &parseFailure{cause: causeEmpty, err: fmt.Errorf("empty proposal")}
	}
	return &prop, nil
}

// RejectedMint is one plan entry the recursion guard dropped, with the reason.
type RejectedMint struct {
	Phase  string
	Reason string
}

// ParsedPlan is the parsed plan and the mints the recursion guard rejected.
type ParsedPlan struct {
	Plan          *router.PhasePlan
	RejectedMints []RejectedMint
}

// ParsePhasePlan decodes the last balanced JSON array in the response; an absent, malformed or empty plan is an error.
func ParsePhasePlan(stdout string) (ParsedPlan, error) {
	start, end, ok := LastBalancedSpan(stdout, '[', ']')
	if !ok {
		return ParsedPlan{}, &parseFailure{cause: causeNoJSON, err: fmt.Errorf("no JSON array in plan output")}
	}
	var entries []router.PhasePlanEntry
	if err := json.Unmarshal([]byte(stdout[start:end+1]), &entries); err != nil {
		return ParsedPlan{}, &parseFailure{cause: causeInvalidJSON, err: fmt.Errorf("parse phase plan: %w", err)}
	}
	if len(entries) == 0 {
		return ParsedPlan{}, &parseFailure{cause: causeEmpty, err: fmt.Errorf("empty phase plan")}
	}
	for i := range entries {
		entries[i].Tier = SanitizeTier(entries[i].Tier)
	}
	mints, rejected := MintConfigsFrom(entries)
	return ParsedPlan{Plan: &router.PhasePlan{Entries: entries, MintPhases: mints}, RejectedMints: rejected}, nil
}

// SanitizeTier keeps only a canonical tier and drops any alias or model name, untranslated;
// the downstream clamp trusts this instead of re-validating.
func SanitizeTier(tier string) string {
	if slices.Contains(modelcatalog.CanonicalTiers, tier) {
		return tier
	}
	return ""
}

// ReplayPlanFromResponse reparses a captured response through the live parse and floor clamp, returning the clamps that fired.
func ReplayPlanFromResponse(raw string, in router.RouteInput, floor []string) (*router.PhasePlan, []router.Clamp, error) {
	parsed, err := ParsePhasePlan(raw)
	if err != nil {
		return nil, nil, err
	}
	clamped, clamps := router.ClampPlanToFloorWith(in, parsed.Plan, floor, in.IntentRequired)
	return clamped, clamps, nil
}

// LastBalancedSpan returns the inclusive bounds of the last top-level open/close span in s, ignoring delimiters
// inside JSON strings. The last span wins because a reply follows any echoed example.
func LastBalancedSpan(s string, open, close byte) (start, end int, ok bool) {
	depth, spanStart := 0, -1
	inStr, esc := false, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inStr {
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case open:
			if depth == 0 {
				spanStart = i
			}
			depth++
		case close:
			if depth > 0 {
				depth--
				if depth == 0 && spanStart >= 0 {
					start, end, ok = spanStart, i, true
				}
			}
		}
	}
	return start, end, ok
}
