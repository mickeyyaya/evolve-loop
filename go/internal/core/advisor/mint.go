package advisor

import (
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/phaseconfig"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

var reservedNames = map[string]struct{}{
	"router":                 {},
	"evolve-router":          {},
	"advisor":                {},
	"phase-advisor":          {},
	"failure-advisor":        {},
	"evolve-failure-advisor": {},
}

func ReservedMintReason(name string) string {
	if _, ok := reservedNames[strings.ToLower(strings.TrimSpace(name))]; ok {
		return fmt.Sprintf("recursion guard: a minted phase may not assume the control-plane router/advisor identity %q", name)
	}
	return ""
}

func MintConfigsFrom(entries []router.PhasePlanEntry) ([]phaseconfig.PhaseConfig, []RejectedMint) {
	var out []phaseconfig.PhaseConfig
	var rejected []RejectedMint
	for _, e := range entries {
		if e.Mint == nil {
			continue
		}
		if reason := ReservedMintReason(e.Phase); reason != "" {
			rejected = append(rejected, RejectedMint{Phase: e.Phase, Reason: reason})
			continue
		}
		writesSource := true
		if e.Mint.WritesSource != nil {
			writesSource = *e.Mint.WritesSource
		}
		out = append(out, phaseconfig.PhaseConfig{
			PhaseSpec: phasespec.PhaseSpec{
				Name:         e.Phase,
				WritesSource: writesSource,
				Description:  e.Mint.Description,
				WhenToUse:    e.Mint.WhenToUse,
			},
			Dispatch: phaseconfig.Dispatch{CLI: e.Mint.CLI, ModelTierDefault: e.Mint.Tier},
			Prompt:   e.Mint.Prompt,
		})
	}
	return out, rejected
}
