package qualitypanel

import (
	"regexp"
	"time"
)

const revertWindow = 30 * 24 * time.Hour

var (
	revertSubject = regexp.MustCompile(`^Revert\b`)
	revertBody    = regexp.MustCompile(`(?m)^This reverts commit [0-9a-f]{7,40}`)
)

type RevertRateInput struct {
	Repo, Window, Source string
	Now                  time.Time
	Cycle                int
}

type RevertRateResult struct {
	Reverts, Commits int
	OQ3              Reading
}

func MeasureRevertRate(in RevertRateInput) (RevertRateResult, error) {
	return measureRevertRate(in, newGitReader(in.Repo))
}

func measureRevertRate(in RevertRateInput, g *gitReader) (RevertRateResult, error) {
	commits := g.commitsBetween(in.Now.Add(-revertWindow), in.Now)
	if g.err != nil {
		return RevertRateResult{}, g.err
	}
	res := RevertRateResult{Commits: len(commits)}
	for _, c := range commits {
		if revertSubject.MatchString(c.Subject) || revertBody.MatchString(c.Body) {
			res.Reverts++
		}
	}
	res.OQ3 = Reading{
		Index: "OQ3", Interval: Wilson(res.Reverts, res.Commits), Method: MethodWilson95,
		N: res.Commits, Window: in.Window, Source: in.Source, Cycle: in.Cycle, Origin: "qualitypanel.MeasureRevertRate",
	}
	return res, nil
}
