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

type parseFailure struct {
	cause string
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

func DecodeProposal(raw []byte) (*router.Proposal, error) {
	if raw == nil {
		return nil, &parseFailure{cause: causeNoJSON, err: fmt.Errorf("no JSON object in proposer output")}
	}
	var prop router.Proposal
	if err := json.Unmarshal(raw, &prop); err != nil {
		return nil, &parseFailure{cause: causeInvalidJSON, err: fmt.Errorf("parse proposal: %w", err)}
	}
	if prop.NextPhase == "" && len(prop.InsertPhases) == 0 &&
		prop.RecoveryAction == "" && prop.LearningRichness == "" {
		return nil, &parseFailure{cause: causeEmpty, err: fmt.Errorf("empty proposal")}
	}
	return &prop, nil
}

type RejectedMint struct {
	Phase  string
	Reason string
}

type ParsedPlan struct {
	Plan          *router.PhasePlan
	RejectedMints []RejectedMint
}

func DecodePhasePlan(raw []byte) (ParsedPlan, error) {
	if raw == nil {
		return ParsedPlan{}, &parseFailure{cause: causeNoJSON, err: fmt.Errorf("no JSON array in plan output")}
	}
	var entries []router.PhasePlanEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
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

func SanitizeTier(tier string) string {
	if slices.Contains(modelcatalog.CanonicalTiers, tier) {
		return tier
	}
	return ""
}

func ReplayPlanFromResponse(raw string, in router.RouteInput, floor []string) (*router.PhasePlan, []router.Clamp, error) {
	parsed, err := ParsePhasePlan(raw)
	if err != nil {
		return nil, nil, err
	}
	clamped, clamps := router.ClampPlanToFloorWith(in, parsed.Plan, floor, in.IntentRequired)
	return clamped, clamps, nil
}
