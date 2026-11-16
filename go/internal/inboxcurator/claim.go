package inboxcurator

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/landedclaim"
)

func CheckedClaims(open []inboxbatch.Item, check landedclaim.Check) []Proposal {
	if check == nil {
		return nil
	}
	var out []Proposal
	for _, it := range open {
		sha := it.LandedSHAText()
		if sha == "" || !check(sha, it.ClaimPaths()) {
			continue
		}
		out = append(out, Proposal{
			Kind: ProposeConsume, ID: it.ID, Path: it.Path, Trigger: TriggerLanding, Deterministic: true,
			Reason:   "a lane's triage claimed the item as landed, and the commit is on main and changes a declared path",
			Evidence: []string{"landed_sha " + sha},
		})
	}
	return out
}
