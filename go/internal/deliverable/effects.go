package deliverable

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// effectCheck appends violations for one declared effect, or returns an
// error when it cannot decide (the caller then fails open).
type effectCheck func(res *Result, roots phasecontract.Roots) error

// effectPerform is the host's performance of an effect before the review;
// nil leaves the effect to the agent.
type effectPerform func(h *HostEffects, roots phasecontract.Roots) error

type effect struct {
	check   effectCheck
	perform effectPerform
}

var effects = map[string]effect{
	phasecontract.EffectInboxClaim: {check: checkInboxClaim, perform: claimCommitted},
}

func verifyEffects(res *Result, c phasecontract.Contract, roots phasecontract.Roots) error {
	for _, name := range c.Effects {
		res.Effects = append(res.Effects, name)
		e, bound := effects[name]
		if !bound {
			res.add(CodeUnboundEffect, fmt.Sprintf("declared effect %q has no deterministic check — a phase-registry defect, not something this agent can correct", name))
			continue
		}
		if err := e.check(res, roots); err != nil {
			return err
		}
	}
	return nil
}

func inboxDir(roots phasecontract.Roots) string {
	return filepath.Join(roots.EvolveDir, "inbox")
}

// checkInboxClaim owes a claim under this cycle's processing/ dir for every committed id that is an inbox item;
// an id with no inbox file, or an empty commitment, owes nothing. The message is the correction directive.
func checkInboxClaim(res *Result, roots phasecontract.Roots) error {
	if roots.Cycle == 0 || roots.EvolveDir == "" {
		return errors.New("deliverable: effect inbox-claim needs Roots.Cycle and Roots.EvolveDir to locate processing/cycle-N/")
	}
	committed, recorded := committedset.Committed(roots.Workspace)
	if !recorded {
		return nil
	}
	for _, id := range committed {
		loc, err := inboxmover.Locate(inboxDir(roots), id)
		if errors.Is(err, inboxmover.ErrNotFound) {
			continue
		}
		if err != nil {
			return fmt.Errorf("deliverable: locate inbox item %q: %w", id, err)
		}
		switch loc.Cycle {
		case roots.Cycle:
			// claimed by this cycle — the effect happened
		case 0:
			res.add(CodeMissingEffect, fmt.Sprintf("declared effect inbox-claim not performed for committed item %q — the host's claim did not land and it is still pending at %s; `evolve inbox-mover claim %q %d` shows why (exit 3 = console-routed): defer or drop it", id, loc.Path, id, roots.Cycle))
		default:
			res.add(CodeMissingEffect, fmt.Sprintf("declared effect inbox-claim not performed for committed item %q — it is held by cycle-%d (%s); a committed item must be claimed by THIS cycle or not committed", id, loc.Cycle, loc.Path))
		}
	}
	return nil
}
