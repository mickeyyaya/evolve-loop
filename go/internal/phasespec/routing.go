package phasespec

import (
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

// ApplyUserRouting splices valid user specs into cfg so the router can propose them; builtin enables the optional-built-in exemption (Catalog{} for none).
func ApplyUserRouting(cfg *config.RoutingConfig, specs []PhaseSpec, builtin Catalog) []string {
	var warnings []string
	var pending []PhaseSpec
	for _, s := range specs {
		// The safety floor is enforced at this wiring seam, so an invalid spec never becomes a routing candidate.
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
	// Placement is a fixpoint: discovery sorts alphabetically, so an anchor may be a later
	// batch-mate. Passes repeat while any spec is placed.
	for len(pending) > 0 {
		var deferred []PhaseSpec
		for _, s := range pending {
			// A name already in the order never defers or warns: nothing will move.
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
				// Held, not force-placed, so it still lands after its stuck anchor.
				if stuck[s.After] {
					held = append(held, s)
					continue
				}
				warnings = append(warnings, "phase "+s.Name+" anchor "+s.After+" not in routing order — placed before audit")
				cfg.Order = spliceAfter(cfg.Order, s.Name, "")
				placed = true
			}
			if !placed { // anchor deadlock (cycle, possibly with tails)
				// Force-place one anchor target and re-loop, so its dependents follow it in
				// declared order and only the broken link warns.
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

// spliceAfter inserts name after anchor, else before "audit", else at the end; a name already present is a no-op.
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
