// Package failuregrade grades a gate or cycle failure into an enforcement tier.
// See docs/architecture/packages/internal-failuregrade.md.
package failuregrade

import "strings"

type Tier int

const (
	TierAbort Tier = iota
	TierCorrect
	TierQuarantine
	TierRepair
)

func (t Tier) String() string {
	switch t {
	case TierCorrect:
		return "correct"
	case TierQuarantine:
		return "quarantine"
	case TierRepair:
		return "repair"
	default:
		return "abort"
	}
}

type Evidence struct {
	ChurnIsBenign   bool
	RebuildVerified bool
}

const (
	sigMissingChallengeToken = "missing_challenge_token"
	sigSelfSHATampered       = "SELF_SHA_TAMPERED"
	sigTreeDiffGuard         = "tree-diff guard"
)

func Grade(reason string, ev Evidence) Tier {
	switch {
	case strings.Contains(reason, sigMissingChallengeToken):
		return TierCorrect
	case strings.Contains(reason, sigTreeDiffGuard):
		if ev.ChurnIsBenign {
			return TierQuarantine
		}
		return TierAbort
	case strings.Contains(reason, sigSelfSHATampered):
		if ev.RebuildVerified {
			return TierRepair
		}
		return TierAbort
	default:
		return TierAbort
	}
}
