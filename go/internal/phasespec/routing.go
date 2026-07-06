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
	for _, s := range specs {
		if v := ValidateUserSpecWithCatalog(s, builtin); len(v) > 0 {
			warnings = append(warnings, "phase "+s.Name+" not routed (invalid): "+strings.Join(v, "; "))
			continue
		}
		cfg.Order = spliceAfter(cfg.Order, s.Name, s.After)
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
