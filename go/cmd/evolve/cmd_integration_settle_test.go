package main

import (
	"context"
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
)

const (
	hostOpen   = "OPEN"
	hostClosed = "CLOSED"
)

type hostPR struct {
	State string
	Head  string
}

type settleJourney struct {
	*trainJourney
	mu   sync.Mutex
	prs  map[int]hostPR
	runs int
}

func (s *settleJourney) run(args ...string) (int, string, string) {
	s.runs++
	s.env.Wave = "s" + strconv.Itoa(s.runs)
	return s.trainJourney.run(args...)
}

func newSettleJourney(t *testing.T) *settleJourney {
	t.Helper()
	s := &settleJourney{trainJourney: newTrainJourney(t), prs: map[int]hostPR{}}
	s.env.Gh = s.route
	return s
}

func (s *settleJourney) host(n int, state, head string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prs[n] = hostPR{State: state, Head: head}
}

func (s *settleJourney) entry(n int) map[string]any {
	p := s.prs[n]
	m := map[string]any{"number": n, "state": p.State, "closed": p.State != hostOpen, "headRefName": p.Head, "mergedAt": nil}
	if s.gh.pullHead != nil {
		if oid := s.gh.pullHead(strconv.Itoa(n)); oid != "" {
			m["headRefOid"] = oid
		}
	}
	return m
}

func (s *settleJourney) openList() string {
	var open []map[string]any
	for n, p := range s.prs {
		if p.State == hostOpen {
			open = append(open, s.entry(n))
		}
	}
	return hostJSON(open)
}

func hostJSON(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(raw) + "\n"
}

func (s *settleJourney) route(ctx context.Context, dir string, args ...string) (string, int, error) {
	s.mu.Lock()
	joined := strings.Join(args, " ")
	switch {
	case strings.HasPrefix(joined, "pr view "):
		n, _ := strconv.Atoi(args[2])
		out := hostJSON(s.entry(n))
		s.mu.Unlock()
		return out, 0, nil
	case strings.HasPrefix(joined, "pr list") && (slices.Contains(args, "closed") || slices.Contains(args, "merged") || slices.Contains(args, "all")):
		var listed []map[string]any
		for n, p := range s.prs {
			if p.State != hostOpen || slices.Contains(args, "all") {
				listed = append(listed, s.entry(n))
			}
		}
		s.mu.Unlock()
		return hostJSON(listed), 0, nil
	}
	s.publishOpenList()
	s.mu.Unlock()
	return s.trainJourney.gh.run(ctx, dir, args...)
}

func (s *settleJourney) publishOpenList() { s.trainJourney.gh.ready = s.openList() }

func (s *settleJourney) batchPR(t *testing.T, n int, head string, members ...int) {
	t.Helper()
	branch := "batch-" + strconv.Itoa(n)
	s.work.Git("checkout", "-q", "-b", branch, "main")
	for _, m := range members {
		ms := strconv.Itoa(m)
		file := "batch-member-" + ms + ".md"
		s.work.Write(file, ms+"\n")
		s.work.Git("add", file)
		s.work.Git("commit", "-q", "-m", "evolve-pr "+ms+": integrate\n\nSource-PR: #"+ms)
	}
	s.work.Git("push", "-q", "origin", branch+":refs/pull/"+strconv.Itoa(n)+"/head")
	s.work.Git("checkout", "-q", "main")
	s.host(n, hostOpen, head)
}

func (s *settleJourney) recorded(t *testing.T, pr int) {
	t.Helper()
	s.host(pr, hostOpen, "feat/pr-"+strconv.Itoa(pr))
	if rc, _, errb := s.run("record-pr", "--pr", strconv.Itoa(pr), "--class", "docs-review"); rc != 0 {
		t.Fatalf("record-pr %d: rc=%d %s", pr, rc, errb)
	}
}

func (s *settleJourney) landWithoutSettling(t *testing.T, pr int) {
	t.Helper()
	before := s.originMain()
	s.gh.failClose = true
	rc, _, _ := s.run("batch", strconv.Itoa(pr))
	s.gh.failClose = false
	if rc != exitIO || s.originMain() == before {
		t.Fatalf("batch %d: rc=%d moved=%v, want main moved and the close refused (an interrupted settle)", pr, rc, s.originMain() != before)
	}
	if _, ok := integration.FindPRRecord(s.env.Root, pr); !ok {
		t.Fatalf("the record of PR %d must stay after the refused close", pr)
	}
	if len(s.gh.closed) != 0 {
		t.Fatalf("a refused close must record no close: %v", s.gh.closed)
	}
}

func settledLine(out string, pr int) (string, bool) {
	needle := "PR #" + strconv.Itoa(pr)
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, needle) && strings.Contains(line, "spent") {
			return line, true
		}
	}
	return "", false
}

func TestJourneySettleClosesAndSpendsALandedPRAtTheNextBatchStart(t *testing.T) {
	t.Parallel()
	s := newSettleJourney(t)
	s.pr(t, 1, "feat: a", "a.md", "a\n")
	s.pr(t, 2, "feat: b", "b.md", "b\n")
	s.recorded(t, 1)
	s.recorded(t, 2)
	s.landWithoutSettling(t, 1)

	if rc, _, errb := s.run("batch", "--dry-run", "2"); rc != 0 || len(s.gh.closed) != 0 {
		t.Fatalf("a dry run must settle nothing: rc=%d closed=%v err=%s", rc, s.gh.closed, errb)
	}
	if _, ok := integration.FindPRRecord(s.env.Root, 1); !ok {
		t.Fatal("a dry run must keep the record of the landed PR")
	}

	rc, out, errb := s.run("batch", "2")
	if rc != 0 {
		t.Fatalf("batch 2: rc=%d out=%s err=%s", rc, out, errb)
	}
	if body := s.gh.closed["1"]; !strings.Contains(body, "already on main") {
		t.Errorf("PR 1 close comment = %q, want the landed note", body)
	}
	if _, ok := integration.FindPRRecord(s.env.Root, 1); ok {
		t.Error("the record of the landed PR must be spent at the next batch start")
	}
	if line, ok := settledLine(out, 1); !ok {
		t.Errorf("no printed line names PR #1 as spent:\n%s", out)
	} else if strings.Contains(line, "step:") {
		t.Errorf("the settle line %q must not read as a batch step", line)
	}
}

func TestJourneySettleSpendsTheRecordOfAClosedPRWithAPrintedLine(t *testing.T) {
	t.Parallel()
	s := newSettleJourney(t)
	s.pr(t, 2, "feat: b", "b.md", "b\n")
	s.pr(t, 3, "feat: c", "c.md", "c\n")
	s.pr(t, 5, "feat: e", "e.md", "e\n")
	s.recorded(t, 2)
	s.recorded(t, 3)
	s.recorded(t, 5)
	s.host(5, hostClosed, "feat/pr-5")

	rc, out, errb := s.run("batch", "2")
	if rc != 0 {
		t.Fatalf("batch 2: rc=%d out=%s err=%s", rc, out, errb)
	}
	if _, ok := integration.FindPRRecord(s.env.Root, 5); ok {
		t.Error("the record of a closed PR must be spent")
	}
	if _, ok := settledLine(out, 5); !ok {
		t.Errorf("no printed line names PR #5 as spent:\n%s", out)
	}
	if body, closed := s.gh.closed["5"]; closed || len(s.gh.comments["5"]) != 0 {
		t.Errorf("the host closed PR 5 already: no second close %q and no comment %v", body, s.gh.comments["5"])
	}
	if _, ok := integration.FindPRRecord(s.env.Root, 3); !ok {
		t.Error("the record of an open PR that is not on main must stay")
	}
	if line, ok := settledLine(out, 3); ok {
		t.Errorf("PR #3 is open and not on main, yet a line settles it: %q", line)
	}
	if _, closed := s.gh.closed["3"]; closed {
		t.Error("PR 3 is open and not on main: it must not close")
	}
}

func TestJourneySettleClosesAnOpenBatchPRWhoseMembersAreLandedOrClosed(t *testing.T) {
	t.Parallel()
	s := newSettleJourney(t)
	s.pr(t, 1, "feat: a", "a.md", "a\n")
	s.pr(t, 2, "feat: b", "b.md", "b\n")
	s.pr(t, 3, "feat: c", "c.md", "c\n")
	s.pr(t, 5, "feat: e", "e.md", "e\n")
	for _, n := range []int{1, 2, 3, 5} {
		s.recorded(t, n)
	}
	s.landWithoutSettling(t, 1)
	s.host(5, hostClosed, "feat/pr-5")
	s.batchPR(t, 60, "integration/j1-b1", 1, 5)
	s.batchPR(t, 61, "integration/j1-b2", 3)

	rc, out, errb := s.run("batch", "2")
	if rc != 0 {
		t.Fatalf("batch 2: rc=%d out=%s err=%s", rc, out, errb)
	}
	if body, closed := s.gh.closed["60"]; !closed || body == "" {
		t.Errorf("the batch PR #60 holds only a landed PR and a closed PR: it must close with a comment (closed=%v)", s.gh.closed)
	}
	if _, closed := s.gh.closed["61"]; closed {
		t.Error("the batch PR #61 holds PR 3, which is open and not on main: it must stay open")
	}
	if _, ok := integration.FindPRRecord(s.env.Root, 3); !ok {
		t.Error("the record of PR 3 must stay")
	}
}
