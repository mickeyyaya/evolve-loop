package handoff

import (
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func Independents(author Author, served func(cli, tier string) (model string, ok bool), vendors map[string][]string) func(cli, tier string) bool {
	authorModel, resolved := servedGroup(served, author.CLI, author.Tier)
	authorVendors := vendors[llmroute.Family(author.CLI)]
	attributed := !slices.Contains([]string{"", Unattributed}, author.CLI)
	return func(cli, tier string) bool {
		seatModel, ok := servedGroup(served, cli, tier)
		seatVendors := vendors[llmroute.Family(cli)]
		if !attributed || !resolved || !ok || len(authorVendors) == 0 || len(seatVendors) == 0 {
			return false
		}
		if !sharesVendor(authorVendors, seatVendors) {
			return true
		}
		return llmroute.SameFamily(cli, author.CLI) && seatModel != authorModel && policy.TierRank(tier) > policy.TierRank(author.Tier)
	}
}

func servedGroup(served func(cli, tier string) (model string, ok bool), cli, tier string) (string, bool) {
	model, ok := served(cli, tier)
	return clihealth.ModelGroup(model), ok && model != ""
}

func sharesVendor(a, b []string) bool {
	return slices.ContainsFunc(a, func(vendor string) bool { return slices.Contains(b, vendor) })
}

type Identity struct {
	Served  func(cli, tier string) (model string, ok bool)
	Vendors map[string][]string
}

func (id Identity) Basis(author Author, seat Seat, ceiling string) Basis {
	authorModel, _ := servedGroup(id.Served, author.CLI, author.Tier)
	seatModel, _ := servedGroup(id.Served, seat.CLI, seat.Tier)
	return Basis{
		AuthorModel:   authorModel,
		SeatModel:     seatModel,
		AuthorVendors: id.Vendors[llmroute.Family(author.CLI)],
		SeatVendors:   id.Vendors[llmroute.Family(seat.CLI)],
		TierCeiling:   ceiling,
	}
}
