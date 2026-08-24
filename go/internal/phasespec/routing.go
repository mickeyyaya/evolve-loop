package phasespec

import (
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

// ApplyUserRouting splices validated user phases into the routing config so the
// kernel router can PROPOSE them: each phase is positioned in cfg.Order (after
// its After anchor, or just before "audit" by default), its insert_when
// triggers are registered, and it is marked content-routed (EnableContent).
//
// Invalid specs (per ValidateUserSpecWithCatalog — e.g. not optional) are
// SKIPPED with a warning and never routed: the safety floor is enforced at the
// wiring seam, so a malformed user phase can never enter the kernel's candidate
// set. Returns the skip warnings. The builtin catalog exempts activation
// overlays for already-optional built-in phases from the single-word naming
// floor (see ValidateUserSpecWithCatalog) — pass an empty Catalog{} for no
// exemption.
func ApplyUserRouting(cfg *config.RoutingConfig, specs []PhaseSpec, builtin Catalog) []string {
	var warnings []string
	var pending []PhaseSpec
	for _, s := range specs {
		if v := ValidateUserSpecWithCatalog(s, builtin); len(v) > 0 {
			warnings = append(warnings, "phase "+s.Name+" not routed (invalid): "+strings.Join(v, "; "))
			continue
		}
		if s.Routing != nil {
			if cfg.Triggers == nil {
				cfg.Triggers = map[string]config.RoutingBlock{}
			}
			cfg.Triggers[s.Name] = *s.Routing
		}
		if cfg.PhaseEnable == nil {
			cfg.PhaseEnable = map[string]config.Enable{}
		}
		cfg.PhaseEnable[s.Name] = config.EnableContent
		pending = append(pending, s)
	}
	// Placement is a FIXPOINT over the batch: a spec with a non-empty anchor
	// waits for that anchor, which may itself be a later spec in the same batch
	// — DiscoverUserSpecs sorts alphabetically, so input order never guarantees
	// anchor-before-anchored (cycle-1550: bug-reproduction, anchored after
	// fault-localization, was spliced first, missed its anchor, and silently
	// took the before-audit fallback — executing a red-first Evaluate phase
	// POST-build). Passes repeat while progress is made. When a pass strands,
	// the stuck set splits: an anchor naming NO batch-mate will never appear —
	// force-place that spec at the fallback, LOUDLY — while a spec anchored to
	// a stuck batch-mate is only TRANSITIVELY blocked and is held, so the next
	// pass places it after its just-placed anchor (declared order honored even
	// through the escape). Only a pure anchor cycle, where no honorable order
	// exists, falls back wholesale.
	for len(pending) > 0 {
		var deferred []PhaseSpec
		for _, s := range pending {
			// A name already in the order (activation overlay of a placed
			// phase) never defers and never warns: spliceAfter is idempotent
			// and nothing will move regardless of the anchor's fate.
			if indexOfStr(cfg.Order, s.Name) < 0 && s.After != "" && indexOfStr(cfg.Order, s.After) < 0 {
				deferred = append(deferred, s)
				continue
			}
			cfg.Order = spliceAfter(cfg.Order, s.Name, s.After)
		}
		if len(deferred) == len(pending) { // no anchor resolved this pass
			stuck := map[string]bool{}
			for _, s := range deferred {
				stuck[s.Name] = true
			}
			var held []PhaseSpec
			placed := false
			for _, s := range deferred {
				if stuck[s.After] {
					held = append(held, s)
					continue
				}
				warnings = append(warnings, "phase "+s.Name+" anchor "+s.After+" not in routing order — placed before audit")
				cfg.Order = spliceAfter(cfg.Order, s.Name, "")
				placed = true
			}
			if !placed { // anchor deadlock (cycle, possibly with tails)
				// Force-place exactly ONE spec — the first that is itself an
				// anchor target of another stuck spec — then re-loop: its
				// dependents (cycle tails and remaining members alike) resolve
				// after it on later passes, honoring their declared order. Only
				// the broken link gets a warning; the specs downstream of it
				// are placed honorably and silently.
				isAnchor := map[string]bool{}
				for _, s := range held {
					isAnchor[s.After] = true
				}
				pick := 0
				for i, s := range held {
					if isAnchor[s.Name] {
						pick = i
						break
					}
				}
				s := held[pick]
				warnings = append(warnings, "phase "+s.Name+" anchor "+s.After+" unresolvable (anchor cycle) — placed before audit")
				cfg.Order = spliceAfter(cfg.Order, s.Name, "")
				pending = append(held[:pick:pick], held[pick+1:]...)
				continue
			}
			pending = held
			continue
		}
		pending = deferred
	}
	return warnings
}

// spliceAfter inserts name into order right after anchor. If anchor is empty or
// absent, it inserts just before "audit" (the canonical post-build check slot);
// if "audit" is absent too, it appends. A name already present is left alone.
func spliceAfter(order []string, name, anchor string) []string {
	if indexOfStr(order, name) >= 0 {
		return order
	}
	pos := -1
	if anchor != "" {
		if i := indexOfStr(order, anchor); i >= 0 {
			pos = i + 1
		}
	}
	if pos < 0 {
		if i := indexOfStr(order, "audit"); i >= 0 {
			pos = i
		} else {
			pos = len(order)
		}
	}
	out := make([]string, 0, len(order)+1)
	out = append(out, order[:pos]...)
	out = append(out, name)
	out = append(out, order[pos:]...)
	return out
}

func indexOfStr(xs []string, want string) int {
	for i, x := range xs {
		if x == want {
			return i
		}
	}
	return -1
}
