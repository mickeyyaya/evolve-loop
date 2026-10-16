package dossier

import (
	"context"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

const dossierDir = "knowledge-base/cycles"

var orphanPair = regexp.MustCompile(`(^|/)cycle-(\d+)\.(json|md)$`)

type SweepResult struct {
	Recommitted []int
	Skipped     []int
	Failed      map[int]error
}

type OrphanPlan struct {
	Complete []int
	Half     []int
}

func PlanOrphans(g gitexec.Git) (OrphanPlan, error) {
	plan, _, err := planOrphans(g)
	return plan, err
}

func planOrphans(g gitexec.Git) (OrphanPlan, map[int]*orphanPairPaths, error) {
	var plan OrphanPlan
	ctx := context.Background()
	dirty, err := g.DirtyPaths(ctx)
	if err != nil {
		return plan, nil, fmt.Errorf("dossier: sweep: enumerate tree: %w", err)
	}
	pairs := groupOrphanPairs(dirty)
	cycles := make([]int, 0, len(pairs))
	for n := range pairs {
		cycles = append(cycles, n)
	}
	sort.Ints(cycles)
	for _, n := range cycles {
		pp := pairs[n]
		switch {
		case anyTracked(ctx, g, pp.json, pp.md):
		case pp.json == "" || pp.md == "":
			plan.Half = append(plan.Half, n)
		default:
			plan.Complete = append(plan.Complete, n)
		}
	}
	return plan, pairs, nil
}

func SweepOrphans(g gitexec.Git, logw io.Writer) (SweepResult, error) {
	res := SweepResult{Failed: map[int]error{}}
	plan, pairs, err := planOrphans(g)
	if err != nil {
		return res, err
	}
	res.Skipped = plan.Half
	for _, n := range plan.Complete {
		pp := pairs[n]
		base := strings.TrimSuffix(pp.json, ".json")
		if err := commitPairGit(g, base); err != nil {
			res.Failed[n] = err
			fmt.Fprintf(logw, "[dossier-sweep] ERROR cycle %d (%s): recommit failed: %v\n", n, path.Dir(pp.json), err)
			continue
		}
		res.Recommitted = append(res.Recommitted, n)
	}
	return res, nil
}

func anyTracked(ctx context.Context, g gitexec.Git, files ...string) bool {
	var nonZero []string
	for _, f := range files {
		if f != "" {
			nonZero = append(nonZero, f)
		}
	}
	if len(nonZero) == 0 {
		return false
	}
	args := append([]string{"ls-files", "--"}, nonZero...)
	out, err := g.Output(ctx, args...)
	return err == nil && strings.TrimSpace(out) != ""
}

type orphanPairPaths struct{ json, md string }

func groupOrphanPairs(dirty []string) map[int]*orphanPairPaths {
	pairs := map[int]*orphanPairPaths{}
	for _, p := range dirty {
		slash := filepath.ToSlash(p)
		if path.Dir(slash) != dossierDir {
			continue
		}
		m := orphanPair.FindStringSubmatch(slash)
		if m == nil {
			continue
		}
		n, convErr := strconv.Atoi(m[2])
		if convErr != nil {
			continue
		}
		pp := pairs[n]
		if pp == nil {
			pp = &orphanPairPaths{}
			pairs[n] = pp
		}
		if m[3] == "json" {
			pp.json = slash
		} else {
			pp.md = slash
		}
	}
	return pairs
}
