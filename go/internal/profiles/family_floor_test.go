package profiles

import (
	"strings"
	"testing"
)

// claudeFamilyFloor lists the phases that must stay off the builder's CLI
// family, each with its reason. See ADR-0104.
var claudeFamilyFloor = map[string]string{
	"auditor":            "adversarial grading of build content — cross-family anti-gaming core",
	"adversarial-review": "adversarial grading of build content — cross-family anti-gaming core",
	"tdd-engineer":       "test author — anti-cooperative-bias family split from the builder",
	"spec-verifier":      "audit-side verification of build output",
	"spec-verify":        "audit-side verification of build output",
}

// family reduces a driver name to its CLI family. It mirrors policy.BaseCLI
// because importing internal/policy here would create an import cycle.
func family(cli string) string {
	if i := strings.IndexByte(cli, '-'); i > 0 {
		return cli[:i]
	}
	return cli
}

// It compares against the live builder's family, not a literal, so a builder
// flip cannot make the floor contradict itself.
func TestClaudeFamilyFloor(t *testing.T) {
	loader, names := RealTreeProfiles(t)
	builder, err := loader.Get("builder")
	if err != nil {
		t.Fatalf("Get(builder): %v", err)
	}
	builderFam := family(builder.CLI)

	claudeCount := 0
	for _, name := range names {
		p, err := loader.Get(name)
		if err != nil {
			t.Errorf("profile %s: Get failed (%v) — it cannot be checked, so it cannot be trusted", name, err)
			continue
		}
		if family(p.CLI) != "claude" {
			continue
		}
		claudeCount++
		if _, ok := claudeFamilyFloor[name]; !ok {
			t.Errorf("profile %s: claude family without a floor justification — claude is the quota-constrained family (2026-09-02 directive); either add it to claudeFamilyFloor WITH its load-bearing reason, or route it to codex", name)
		}
	}
	if claudeCount == 0 {
		t.Fatal("matched NO claude-family profiles — the selector is broken and this guard is vacuous")
	}

	for name, why := range claudeFamilyFloor {
		p, err := loader.Get(name)
		if err != nil {
			t.Errorf("floor entry %s (%s) does not resolve in the TRACKED tree: %v — a floor pinned on a missing/untracked profile is vacuous", name, why, err)
			continue
		}
		if family(p.CLI) == builderFam {
			t.Errorf("floor entry %s runs on the builder's own family %q — the floor (%s) is broken", name, p.CLI, why)
		}
		for _, fb := range p.CLIFallback {
			if family(fb) != family(p.CLI) {
				t.Errorf("floor entry %s: cli_fallback %q leaves the %s family — a floored phase fails LOUDLY on quota rather than silently handing %s to another family (see the floor doc)", name, fb, family(p.CLI), why)
			}
		}
		for _, allowed := range p.AllowedCLIs {
			if allowed == "all" || family(allowed) == builderFam {
				t.Errorf("floor entry %s: allowed_clis=%v admits the builder's family — a policy pin could breach the floor at dispatch while every profile guard stays green (mirror the tdd-engineer allowed_clis precedent)", name, p.AllowedCLIs)
				break
			}
		}
	}
}
