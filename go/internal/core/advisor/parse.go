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

// parseFailure classifies why no decision decoded (fields.cause on the
// ADVISOR_RESPONSE_UNPARSEABLE event) while its Error text stays the
// pre-extraction text the orchestrator prints. Unexported: the parsers stay
// pure and Center-free — resume, replay and the routing-eval corpus call
// them with no Center and must emit nothing.
type parseFailure struct {
	cause string // no_json | invalid_json | empty
	err   error
}

func (p *parseFailure) Error() string { return p.err.Error() }
func (p *parseFailure) Unwrap() error { return p.err }

// warnUnparseable reports one parse failure with its cause, the response
// size and the artifact the decision would have read.
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

// ParseProposal extracts the strict-JSON proposal from the response the
// bridge read back — since 2026-09-14 the routing-proposal.json artifact's
// content (a bare object), before that the REPL scrollback, which echoed the
// PROMPT and its JSON example. The LAST balanced object is taken either way
// (an answer is last; a prompt echo is not), tolerant of a ```json fence /
// surrounding prose. Empty/unparseable → error (caller degrades to static).
// PURE and Center-free.
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

// RejectedMint is one plan entry the recursion guard dropped: the minted
// name and the reason. Data, not a print — the entry point reports it once
// with its cycle stamp; resume and replay re-parse silently.
type RejectedMint struct {
	Phase  string
	Reason string
}

// ParsedPlan is ParsePhasePlan's Result Object: the plan and the mints the
// guard rejected.
type ParsedPlan struct {
	Plan          *router.PhasePlan
	RejectedMints []RejectedMint
}

// ParsePhasePlan extracts the strict-JSON whole-cycle plan from the LLM stdout.
// The wire format is a bare array of {phase, run, justification}; like
// ParseProposal it takes the LAST balanced array so the prompt's echoed JSON
// example (present in the captured scrollback under the ADR-0027 stdout
// contract) is not mistaken for the answer. An empty or unparseable body is an
// error (caller degrades to the deterministic static plan). PURE and
// Center-free.
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

// SanitizeTier confines the advisor's OWN emitted tier to the strict
// canonical vocabulary — modelcatalog.CanonicalTiers (fast/balanced/deep/top,
// "top" = the frontier tier), PROJECTED rather than copied: the catalog is
// the vocabulary's one home and the test pins the four — enforcing the
// driver_agnostic_model_routing invariant: the advisor proposes an ABSTRACT
// tier, never a raw or legacy model alias. Unlike policy.TierRank (which
// accepts "opus"/"sonnet"/"haiku" for an OPERATOR pin), an advisor-emitted
// alias or garbage value is dropped outright rather than translated — the
// clamp downstream trusts this invariant instead of re-validating it. Empty
// stays empty (the common no-op case).
func SanitizeTier(tier string) string {
	if slices.Contains(modelcatalog.CanonicalTiers, tier) {
		return tier
	}
	return ""
}

// ReplayPlanFromResponse reparses a captured advisor response (WS3-S1's
// advisor-response-<kind>.txt) through the SAME parse + integrity-floor clamp
// the live planning path runs (ParsePhasePlan → router.ClampPlanToFloorWith,
// the exact pair cyclerun.go uses), and returns the clamped plan + the clamps
// that fired. WS3-S5 replay uses it to prove a recorded response still
// reproduces the recorded phase-plan.json; WS4 builds its golden corpus on the
// same entry point, so a regression there is caught against the real floor —
// not a parallel reimplementation. An unparseable response is a loud error
// (detecting exactly that corruption is the point of replay). Rejected mints
// are dropped silently here — they were reported at decision time.
func ReplayPlanFromResponse(raw string, in router.RouteInput, floor []string) (*router.PhasePlan, []router.Clamp, error) {
	parsed, err := ParsePhasePlan(raw)
	if err != nil {
		return nil, nil, err
	}
	clamped, clamps := router.ClampPlanToFloorWith(in, parsed.Plan, floor, in.IntentRequired)
	return clamped, clamps, nil
}

// LastBalancedSpan finds the LAST top-level balanced span delimited by open/
// close in s, returning [start, end] inclusive indices. It forward-scans while
// tracking JSON string-literal context (with backslash escapes), so a literal
// delimiter inside a "justification" value (e.g. `}` or `]`) is not miscounted.
// It records every top-level span and returns the last, so the agent's reply is
// extracted even when the scrollback also contains an earlier (prompt-echoed)
// example of the same shape. Returns ok=false when no balanced span exists.
// The plan judge and the retry adjudicator read it through core's facade.
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
					start, end, ok = spanStart, i, true // keep scanning for a later span
				}
			}
		}
	}
	return start, end, ok
}
