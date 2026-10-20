package phasespec

import (
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

func ApplyUserRouting(cfg *config.RoutingConfig, specs []PhaseSpec, builtin Catalog) []string {
	var warnings []string
	pending, known := []PhaseSpec(nil), slices.Clone(cfg.Order)
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
	for len(pending) > 0 {
		var deferred []PhaseSpec
		for _, s := range pending {
			if indexOfStr(cfg.Order, s.Name) < 0 && s.After != "" && indexOfStr(cfg.Order, s.After) < 0 {
				deferred = append(deferred, s)
				continue
			}
			cfg.Order = spliceAfter(cfg.Order, s.Name, s.After)
		}
		if len(deferred) == len(pending) {
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
			if !placed {
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
	return append(warnings, refusePreBuildAfterBuild(cfg, specs, known)...)
}

func refusePreBuildAfterBuild(cfg *config.RoutingConfig, specs []PhaseSpec, known []string) []string {
	var warnings []string
	for _, s := range specs {
		if slices.Contains(known, s.Name) {
			continue
		}
		build := indexOfStr(cfg.Order, "build")
		at := indexOfStr(cfg.Order, s.Name)
		if s.Stage != StagePreBuild || build < 0 || at < build {
			continue
		}
		cfg.Order = append(cfg.Order[:at:at], cfg.Order[at+1:]...)
		delete(cfg.PhaseEnable, s.Name)
		delete(cfg.Triggers, s.Name)
		warnings = append(warnings, "phase "+s.Name+" not routed: stage pre-build must precede build, but its slot (after "+anchorOrDefault(s.After)+") is after build")
	}
	return warnings
}

func anchorOrDefault(anchor string) string {
	if anchor == "" {
		return "the pre-audit default"
	}
	return anchor
}

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
