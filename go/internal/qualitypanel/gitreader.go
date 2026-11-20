package qualitypanel

import (
	"strconv"
	"strings"
	"time"
)

type gitRunner func(dir, stdin, name string, args ...string) (string, error)

type gitReader struct {
	repo string
	run  gitRunner
	err  error
}

type gitCommit struct {
	Hash, Subject, Body string
	At                  time.Time
}

func newGitReader(repo string) *gitReader { return &gitReader{repo: repo, run: runIn} }

func (g *gitReader) out(args ...string) string {
	if g.err != nil {
		return ""
	}
	out, err := g.run("", "", "git", append([]string{"-C", g.repo}, args...)...)
	g.err = err
	return out
}

func (g *gitReader) commitsBetween(from, to time.Time) []gitCommit {
	log := g.out("log", "--no-merges", "--since="+strconv.FormatInt(from.Unix(), 10), "--format=%x1e%H%x1f%ct%x1f%s%x1f%b")
	var out []gitCommit
	for _, rec := range strings.Split(log, "\x1e")[1:] {
		f := strings.SplitN(rec, "\x1f", 4)
		secs, _ := strconv.ParseInt(f[1], 10, 64)
		c := gitCommit{Hash: f[0], At: time.Unix(secs, 0), Subject: f[2], Body: f[3]}
		if !c.At.Before(from) && !c.At.After(to) {
			out = append(out, c)
		}
	}
	return out
}
