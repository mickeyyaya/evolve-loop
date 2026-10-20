package cliroute

import (
	"fmt"
	"hash/fnv"
	"math"
	"slices"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
)

const (
	shareBuckets = 100
	maxSharePct  = shareBuckets - 1
	weekWindow   = 7 * 24 * time.Hour
)

type tierShare struct {
	family string
	pct    int
}

func (c *compiler) compileShares(ceilings map[string][]string) map[string]tierShare {
	out := map[string]tierShare{}
	for _, tier := range sortedKeys(c.block.Tiers) {
		pct := c.block.Tiers[tier].SharePct
		key := "cli_routing.tiers." + tier + ".share_pct"
		switch fams := ceilings[tier]; {
		case pct == 0:
		case pct < 1 || pct > maxSharePct:
			c.add(SeverityError, key, "share_pct %d is outside 1 to %d", pct, maxSharePct)
		case len(c.block.Tiers[tier].CLIs) == 0:
			c.add(SeverityError, key, "share_pct needs clis: the first family of the tier chain takes the share")
		case len(fams) < 2:
			c.add(SeverityError, key, "share_pct needs a second family in clis to take the other seats")
		default:
			out[tier] = tierShare{family: fams[0], pct: pct}
		}
	}
	return out
}

func shareTier(v resolver, res resolution) (resolution, error) {
	tier := dispatchTier(res.plan)
	share, ok := v.table.shares[tier]
	if !ok || len(res.plan.Candidates) == 0 || familyOf(res.plan.Candidates[0]) != share.family {
		return res, nil
	}
	bucket := shareBucket(res.req)
	paced, pace := v.pace(res.req.ProjectRoot, share.family, tier)
	res = res.noted("share %d%%: bucket %d of %d; %s", share.pct, bucket, shareBuckets, pace)
	if !paced && bucket < share.pct {
		return res.noted("share keeps the seat on %s", share.family), nil
	}
	res.plan.Candidates = lastFamily(res.plan.Candidates, share.family)
	return res.noted("share gives the seat to %s", familyOf(res.plan.Candidates[0])), nil
}

func dispatchTier(plan llmroute.Plan) string {
	if len(plan.Tiers) > 0 {
		return plan.Tiers[0]
	}
	return plan.Model
}

func shareBucket(req Request) int {
	h := fnv.New32a()
	fmt.Fprintf(h, "%d|%s|%s", req.Cycle, req.Agent, req.Phase)
	return int(h.Sum32() % shareBuckets)
}

func lastFamily(chain []string, family string) []string {
	out := slices.Clone(chain)
	slices.SortStableFunc(out, func(a, b string) int {
		return rankOf(familyOf(a) == family) - rankOf(familyOf(b) == family)
	})
	return out
}

func (v resolver) pace(root, family, tier string) (bool, string) {
	if v.host.TierWeek == nil {
		return false, "no usage reader, so the pace does not apply"
	}
	w, found, err := v.host.TierWeek(root, family, tier)
	switch {
	case err != nil:
		return false, fmt.Sprintf("the usage read failed (%v), so the pace does not apply", err)
	case !found:
		return false, fmt.Sprintf("no week window of %s at %s, so the pace does not apply", family, tier)
	case w.ResetsAt == nil:
		return false, fmt.Sprintf("the week window of %s has no reset time, so the pace does not apply", family)
	}
	gone := weekGone(v.now(), *w.ResetsAt)
	label := windowLabel(w)
	if w.PercentUsed > gone {
		return true, fmt.Sprintf("pace: %s used %.0f%% of its week with %.0f%% of the week gone, so it is paced", label, w.PercentUsed, gone)
	}
	return false, fmt.Sprintf("pace: %s used %.0f%% of its week with %.0f%% of the week gone, so it is on pace", label, w.PercentUsed, gone)
}

func weekGone(now, resetsAt time.Time) float64 {
	left := resetsAt.Sub(now)
	return 100 * math.Min(1, math.Max(0, 1-float64(left)/float64(weekWindow)))
}

func windowLabel(w quotastate.UsageWindow) string {
	if w.Model == "" {
		return w.Family
	}
	return w.Family + " " + w.Model
}

func (v resolver) now() time.Time {
	if v.host.Now == nil {
		return time.Now()
	}
	return v.host.Now()
}

func (r *Router) ShareNotes(projectRoot string, chain []string) []string {
	v := resolver{table: r.tables.Load().declared, host: r.host}
	fams := familiesOf(chain)
	var notes []string
	for _, tier := range sortedKeys(v.table.shares) {
		share := v.table.shares[tier]
		if !slices.Contains(fams, share.family) {
			continue
		}
		_, pace := v.pace(projectRoot, share.family, tier)
		notes = append(notes, fmt.Sprintf("share: when the agent dispatches at %s, %s takes %d%% of the seats; %s", tier, share.family, share.pct, pace))
	}
	return notes
}
