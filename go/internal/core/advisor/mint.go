package advisor

import (
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/phaseconfig"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// reservedNames are the control-plane identities a minted phase may never
// assume — the WS1-S2 recursion guard (PRIMARY). The advisor composes the
// executed spine; minting a router/advisor would let a brain schedule a brain,
// breaking the compose-vs-execute layering (ADR-0052 D1). The AgentLabel values
// the advisors dispatch under ("router"/"failure-advisor") are the canonical
// members; aliases cover the persona slug and bare role words.
var reservedNames = map[string]struct{}{
	"router":                 {},
	"evolve-router":          {},
	"advisor":                {},
	"phase-advisor":          {},
	"failure-advisor":        {},
	"evolve-failure-advisor": {},
}

// ReservedMintReason returns a non-empty reason when name is a reserved
// control-plane identity (so a dropped mint is observable), or "" when the name
// is free to mint. Matched case-insensitively after trimming.
func ReservedMintReason(name string) string {
	if _, ok := reservedNames[strings.ToLower(strings.TrimSpace(name))]; ok {
		return fmt.Sprintf("recursion guard: a minted phase may not assume the control-plane router/advisor identity %q", name)
	}
	return ""
}

// MintConfigsFrom reconstructs a phaseconfig.PhaseConfig for every entry that
// carries a Mint block. The entry's Phase becomes the phase name (and default
// agent/profile key); the MintSpec supplies the persona + dispatch knobs. The
// registrar later forces Optional + clamps the tier/cli, so this mapping does
// the minimum: name + inline prompt + tier + cli + writes_source.
//
// A mint entry is collected regardless of its Run flag: REGISTRATION (wiring the
// phase into runners/catalog/routing) is distinct from DISPATCH (whether it runs
// this cycle, which the entry's Run flag governs via the routing loop). A
// run:false mint thus reserves the phase without executing it. Returns nil (the
// common no-op path) when no entry mints.
//
// WS1-S2: a mint that would assume a reserved control-plane identity is dropped
// (recursion guard) — the advisor proposes spine phases, never a router. The
// drops are returned as DATA; the entry point reports each once with its cycle
// stamp (ADVISOR_MINT_REJECTED) instead of every re-parse re-printing it.
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
