package qualitypanel

import (
	"strconv"
	"strings"
	"time"
)

const (
	churnMinAge    = 14 * 24 * time.Hour
	churnMaxAge    = 30 * 24 * time.Hour
	churnResamples = 1000
	churnSeed      = 1
)

type ChurnInput struct {
	Repo, Window, Source string
	Now                  time.Time
	Cycle                int
}

type ChurnResult struct {
	Added, Survived int
	OQ2             Reading
}

func MeasureChurn(in ChurnInput) (ChurnResult, error) {
	return measureChurn(in, newGitReader(in.Repo))
}

func measureChurn(in ChurnInput, g *gitReader) (ChurnResult, error) {
	var res ChurnResult
	var rewritten, added []float64
	for _, c := range g.commitsBetween(in.Now.Add(-churnMaxAge), in.Now.Add(-churnMinAge)) {
		a, s := g.churnOfCommit(c)
		if a == 0 {
			continue
		}
		res.Added, res.Survived = res.Added+a, res.Survived+s
		rewritten, added = append(rewritten, float64(a-s)), append(added, float64(a))
	}
	if g.err != nil {
		return ChurnResult{}, g.err
	}
	res.OQ2 = Reading{
		Index: "OQ2", Interval: ClusterBootstrapRatio(rewritten, added, churnResamples, churnSeed), Method: MethodBootstrap95,
		N: len(added), Window: in.Window, Source: in.Source, Cycle: in.Cycle, Origin: "qualitypanel.MeasureChurn",
	}
	return res, nil
}

func (g *gitReader) churnOfCommit(c gitCommit) (added, survived int) {
	snapshot := strings.TrimSpace(g.out("rev-list", "-1", "--before="+strconv.FormatInt(c.At.Add(churnMinAge).Unix(), 10), "HEAD"))
	for _, line := range strings.Split(g.out("show", "--numstat", "--no-renames", "--format=", c.Hash), "\n") {
		f := strings.Split(line, "\t")
		if len(f) != 3 || !isProductionGo(f[2]) {
			continue
		}
		n, _ := strconv.Atoi(f[0])
		added += n
		survived += g.survivors(snapshot, f[2], c.Hash)
	}
	return added, survived
}

func (g *gitReader) survivors(snapshot, path, hash string) int {
	if strings.TrimSpace(g.out("ls-tree", "--name-only", snapshot, "--", path)) == "" {
		return 0
	}
	return strings.Count("\n"+g.out("blame", "--line-porcelain", snapshot, "--", path), "\n"+hash+" ")
}
