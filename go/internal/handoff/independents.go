package handoff

import (
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const unattributedAuthor = "unattributed"

func Independents(author Author) func(cli, tier string) bool {
	authorRank := policy.TierRank(author.Tier)
	attributed := !slices.Contains([]string{"", unattributedAuthor}, author.CLI)
	return func(cli, tier string) bool {
		if !attributed {
			return false
		}
		if !llmroute.SameFamily(cli, author.CLI) {
			return true
		}
		return authorRank > 0 && policy.TierRank(tier) > authorRank
	}
}
