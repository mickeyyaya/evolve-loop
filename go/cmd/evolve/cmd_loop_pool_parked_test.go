package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/mainstop"
)

var parkedKey = baseattr.Key{Package: "example.com/m/dup", Test: "TestDup"}

type parkedPool struct {
	t         *testing.T
	repo      *gittest.Repo
	evolveDir string
}

func newParkedPool(t *testing.T) parkedPool {
	t.Helper()
	repo := gittest.Fixture(t)
	p := parkedPool{t: t, repo: repo, evolveDir: filepath.Join(repo.Dir, ".evolve")}
	p.commit("first")
	seedPoolInbox(t, p.evolveDir, nil)
	return p
}

func (p parkedPool) commit(body string) string {
	p.t.Helper()
	p.repo.Write("count", body)
	p.repo.Git("add", "count")
	p.repo.Git("commit", "-q", "-m", body)
	return p.repo.Git("rev-parse", "HEAD")
}

func (p parkedPool) park(cycle int, reason, item string) {
	p.t.Helper()
	seedPausedRun(p.t, p.repo.Dir, cycle, 0, 3, item)
	path := filepath.Join(p.evolveDir, "runs", "cycle-"+strconv.Itoa(cycle), core.CycleStateFile)
	raw, err := os.ReadFile(path)
	if err != nil {
		p.t.Fatal(err)
	}
	writeE2File(p.t, path, strings.Replace(string(raw), `"reason":"quota-likely"`, `"reason":"`+reason+`"`, 1))
	writeE2File(p.t, filepath.Join(p.evolveDir, "inbox", "2026-11-10T00-00-00Z-"+item+".json"), `{"id":"`+item+`","weight":0.1,"files":["go/`+item+`.go"]}`)
	claimFor(p.t, p.evolveDir, cycle, item)
}

func (p parkedPool) hold(mainSHA string) mainstop.Stop {
	p.t.Helper()
	stop, err := mainstop.Update(p.evolveDir, func(s mainstop.Stop, _ bool) (mainstop.Stop, error) {
		return s.Opened(baseattr.Verdict{MainRed: []baseattr.Key{parkedKey}, Main: mainSHA}, mainstop.FixItemID(mainSHA), time.Now()), nil
	})
	if err != nil {
		p.t.Fatal(err)
	}
	return stop
}

func (p parkedPool) release() {
	p.t.Helper()
	if _, err := mainstop.Update(p.evolveDir, func(s mainstop.Stop, _ bool) (mainstop.Stop, error) {
		return s.Released([]baseattr.Key{parkedKey}, "cycle 9 fix", time.Now()), nil
	}); err != nil {
		p.t.Fatal(err)
	}
}

func leadIDs(todos []fleet.Todo, n int) []string {
	var ids []string
	for _, td := range todos[:min(n, len(todos))] {
		ids = append(ids, td.ID)
	}
	return ids
}

func todoIDs(todos []fleet.Todo) []string { return leadIDs(todos, len(todos)) }

func TestPool_ParkedLanesResumeTogetherWhenTheStopReleases(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		arrange  func(p parkedPool, tip string)
		wantLead []string
		wantWarn string
	}{
		{name: "the stop is released", arrange: func(p parkedPool, tip string) { p.hold(tip); p.release() }, wantLead: []string{"quota-lane", "parked-a", "parked-b"}},
		{name: "main moved off the commit of the stop", arrange: func(p parkedPool, tip string) { p.hold(tip); p.commit("second") }, wantLead: []string{"quota-lane", "parked-a", "parked-b"}},
		{name: "no record", arrange: func(parkedPool, string) {}, wantLead: []string{"quota-lane", "parked-a", "parked-b"}},
		{name: "the record does not parse", arrange: func(p parkedPool, _ string) { writeE2File(p.t, mainstop.Path(p.evolveDir), "{") },
			wantLead: []string{"quota-lane", "parked-a", "parked-b"}, wantWarn: "so the parked lanes resume"},
		{name: "the stop holds on the main tip", arrange: func(p parkedPool, tip string) { p.hold(tip) }, wantLead: []string{"quota-lane"}},
		{name: "the stop holds and main does not resolve", arrange: func(p parkedPool, tip string) { p.hold(tip); p.repo.Git("branch", "-m", "main", "trunk") },
			wantLead: []string{"quota-lane"}, wantWarn: "so the parked lanes stay parked"},
		{name: "the stop is released and main does not resolve", arrange: func(p parkedPool, tip string) { p.hold(tip); p.release(); p.repo.Git("branch", "-m", "main", "trunk") },
			wantLead: []string{"quota-lane", "parked-a", "parked-b"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := newParkedPool(t)
			p.park(2240, "quota-likely", "quota-lane")
			p.park(2242, core.MainRedReason, "parked-a")
			p.park(2249, core.MainRedReason, "parked-b")
			tc.arrange(p, p.repo.Git("rev-parse", "HEAD"))
			var stderr bytes.Buffer

			todos := poolPlanFor(t, p.repo.Dir, p.evolveDir, &stderr)

			if got := leadIDs(todos, len(tc.wantLead)); !slices.Equal(got, tc.wantLead) {
				t.Errorf("lead = %v, want %v (todos %v, stderr %q)", got, tc.wantLead, todoIDs(todos), stderr.String())
			}
			if parked := slices.Contains(todoIDs(todos), "parked-a"); parked != (len(tc.wantLead) == 3) {
				t.Errorf("todos = %v: a parked lane is in the plan only when the stop lets it resume", todoIDs(todos))
			}
			if warned := strings.Contains(stderr.String(), "[pool] WARN main-red stop"); warned != (tc.wantWarn != "") || !strings.Contains(stderr.String(), tc.wantWarn) {
				t.Errorf("stderr = %q, want the main-red warning %q and no other", stderr.String(), tc.wantWarn)
			}
		})
	}
}

func TestPool_APlanWithNoParkedLaneReadsNoStop(t *testing.T) {
	t.Parallel()
	p := newParkedPool(t)
	p.park(2240, "quota-likely", "quota-lane")
	writeE2File(t, mainstop.Path(p.evolveDir), "{")
	var stderr bytes.Buffer

	todos := poolPlanFor(t, p.repo.Dir, p.evolveDir, &stderr)

	if got := leadIDs(todos, 1); !slices.Equal(got, []string{"quota-lane"}) || strings.Contains(stderr.String(), "[pool] WARN main-red stop") {
		t.Errorf("lead = %v stderr %q, want the quota lane first and no read of the stop record", got, stderr.String())
	}
}

func TestPausedPlan_AParkedScopeWaitsUntilThePlanLetsParkedLanesResume(t *testing.T) {
	t.Parallel()
	located := func(core.PausedScope) ([]string, error) { return nil, nil }
	cases := []struct {
		name     string
		attempts int
		resume   bool
		wantLead int
	}{
		{name: "the plan holds parked lanes", resume: false, wantLead: 0},
		{name: "the plan lets parked lanes resume", resume: true, wantLead: 1},
		{name: "a parked lane with no resume budget left stays out", attempts: 3, resume: true, wantLead: 0},
	}
	for _, tc := range cases {
		paused := []core.PausedScope{{Cycle: 2242, Scope: []string{"parked-a"}, AutoAttempts: tc.attempts, AutoMaxAttempts: 3, Reason: core.MainRedReason}}
		got := pausedPlan{paused: paused, now: time.Now(), adoptable: adoptAll, itemFiles: located, parkedResume: tc.resume}.order(nil)
		if len(got) != tc.wantLead {
			raw, _ := json.Marshal(got)
			t.Errorf("%s: backlog = %s, want %d parked scope in the lead", tc.name, raw, tc.wantLead)
		}
	}
}
