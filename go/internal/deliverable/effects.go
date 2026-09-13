package deliverable

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// effects.go — ADR-0100 slice 2: a declared EFFECT is verified at the phase
// boundary exactly as a declared output is.
//
// A phase's persona can be instructed to do something outside its workspace
// that later phases and sibling lanes depend on. Triage's inbox claim is the
// one that exists today: `evolve inbox-mover claim` moves the item into
// processing/cycle-N/ so no other lane's triage can select it. Batch cycles
// 1630 (claim refused by the sandbox) and 1631 (the worktree's tracked COPY
// of the inbox was claimed, the plane's item stayed dispatchable) showed the
// report and the decision looking complete while the effect had not happened,
// and nothing judged it — the spine ran on an unclaimed commitment.
//
// The registry declares effects by name; effectChecks binds each name to one
// deterministic check (a registry lookup, not a strategy hierarchy — one
// entry today). A declared name with no binding is a registry defect and is
// reported as such: re-dispatching an agent cannot bind a check.

// effectCheck appends violations for one declared effect, or returns an
// error when it cannot decide (infra ambiguity ⇒ the caller fails OPEN).
type effectCheck func(res *Result, roots phasecontract.Roots) error

var effectChecks = map[string]effectCheck{
	"inbox-claim": checkInboxClaim,
}

// verifyEffects runs the check bound to each declared effect.
func verifyEffects(res *Result, c phasecontract.Contract, roots phasecontract.Roots) error {
	for _, name := range c.Effects {
		check, bound := effectChecks[name]
		if !bound {
			res.add(CodeUnboundEffect, fmt.Sprintf("declared effect %q has no deterministic check — a phase-registry defect, not something this agent can correct", name))
			continue
		}
		if err := check(res, roots); err != nil {
			return err
		}
	}
	return nil
}

// checkInboxClaim owes a claim for every committed id that is an inbox item:
// the item's file must sit under this cycle's processing/ dir. An id with no
// inbox file (scout- or carryover-originated work) owes nothing; an empty or
// unrecorded commitment owes nothing. Each violation names the item and the
// exact command, because that message is the correction directive.
func checkInboxClaim(res *Result, roots phasecontract.Roots) error {
	if roots.Cycle == 0 || roots.EvolveDir == "" {
		return errors.New("deliverable: effect inbox-claim needs Roots.Cycle and Roots.EvolveDir to locate processing/cycle-N/")
	}
	committed, recorded := committedset.Committed(roots.Workspace)
	if !recorded {
		return nil
	}
	inboxDir := filepath.Join(roots.EvolveDir, "inbox")
	for _, id := range committed {
		loc, err := inboxmover.Locate(inboxDir, id)
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
			res.add(CodeMissingEffect, fmt.Sprintf("declared effect inbox-claim not performed for committed item %q — it is still pending at %s; run exactly: evolve inbox-mover claim %q %d", id, loc.Path, id, roots.Cycle))
		default:
			res.add(CodeMissingEffect, fmt.Sprintf("declared effect inbox-claim not performed for committed item %q — it is held by cycle-%d (%s); a committed item must be claimed by THIS cycle or not committed", id, loc.Cycle, loc.Path))
		}
	}
	return nil
}
