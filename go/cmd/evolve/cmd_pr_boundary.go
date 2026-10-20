package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/ciwatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/mergewindow"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

const (
	prBoundaryUsage  = "usage: evolve pr boundary [--json] [--project-root P]"
	prBoundaryPrefix = "evolve pr boundary: "
	prListLimit      = "200"
	prMergeable      = "MERGEABLE"
	reasonNoLoop     = "no early boundary: no loop is running, so the plane is at a boundary"
)

var errPoolUnknown = errors.New("no pool signal for the live wave, so the pool width is unknown; read evolve wave status")

type boundaryPlane struct {
	gh       func(ctx context.Context, args ...string) ([]byte, error)
	liveRuns func() ([]runlease.LiveRun, error)
	poolLive func() (int, bool, error)
	now      func() time.Time
}

type prBoundaryReport struct {
	LoopLive  bool   `json:"loop_live"`
	Due       bool   `json:"due"`
	PR        string `json:"pr,omitempty"`
	WaitS     int    `json:"wait_s"`
	BoundS    int    `json:"bound_s"`
	PoolLive  int    `json:"pool_live"`
	PoolWidth int    `json:"pool_width"`
	Line      string `json:"line"`
}

func defaultBoundaryPlane(root string) boundaryPlane {
	evolveDir := paths.EvolveDirOf(root)
	return boundaryPlane{
		gh:       ghIn(root),
		liveRuns: func() ([]runlease.LiveRun, error) { return planeLiveRuns(root, time.Now()) },
		poolLive: func() (int, bool, error) {
			records, err := wave.NewStore(evolveDir).Records()
			if err != nil || len(records) == 0 {
				return 0, false, err
			}
			loop, warnings := wave.ReadLoopSignals(root)
			live, ok := wave.PoolLive(records[len(records)-1], nil, loop)
			if !ok && len(warnings) > 0 {
				return 0, false, fmt.Errorf("read the loop signals: %s", strings.Join(warnings, "; "))
			}
			return live, ok, nil
		},
		now: time.Now,
	}
}

func runPRBoundary(args []string, stdout, stderr io.Writer) int {
	return runPRBoundaryWith(defaultBoundaryPlane, args, stdout, stderr)
}

func runPRBoundaryWith(planeOf func(string) boundaryPlane, args []string, stdout, stderr io.Writer) int {
	var a struct {
		projectRoot string
		asJSON      bool
	}
	operands, err := cliFlags{bools: map[string]*bool{"--json": &a.asJSON}, values: map[string]*string{"--project-root": &a.projectRoot}}.parse(args)
	if err == nil && len(operands) > 0 {
		err = fmt.Errorf("unexpected operand %q", operands[0])
	}
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n%s\n", prBoundaryPrefix, err, prBoundaryUsage)
		return exitUsage
	}
	rep, err := boundaryReport(a.projectRoot, planeOf, stderr)
	if err == nil {
		err = writePRBoundary(stdout, rep, a.asJSON)
	}
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n", prBoundaryPrefix, err)
		return exitIO
	}
	if rep.Due {
		return 0
	}
	return exitBoundary
}

func boundaryReport(projectRoot string, planeOf func(string) boundaryPlane, stderr io.Writer) (prBoundaryReport, error) {
	root, err := loopStopRoot(projectRoot, stderr)
	if err != nil {
		return prBoundaryReport{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), prWindowTimeout)
	defer cancel()
	return decidePRBoundary(ctx, root, planeOf(root))
}

func decidePRBoundary(ctx context.Context, root string, p boundaryPlane) (prBoundaryReport, error) {
	pol, err := policy.Load(paths.PolicyPath(paths.EvolveDirOf(root)))
	if err != nil {
		return prBoundaryReport{}, err
	}
	bound, err := pol.PREarlyBoundaryAfter()
	if err != nil {
		return prBoundaryReport{}, err
	}
	live, err := p.liveRuns()
	if err != nil {
		return prBoundaryReport{}, err
	}
	if len(live) == 0 {
		return prBoundaryReport{Line: reasonNoLoop}, nil
	}
	lanes, known, err := p.poolLive()
	if err == nil && !known {
		err = errPoolUnknown
	}
	if err != nil {
		return prBoundaryReport{}, err
	}
	waiting, err := waitingBoundaryPRs(ctx, root, p)
	if err != nil {
		return prBoundaryReport{}, err
	}
	pool := mergewindow.Pool{Live: lanes, Width: pol.FleetConfig().Count}
	e := mergewindow.DecideEarlyBoundary(waiting, pool, bound)
	return prBoundaryReport{
		LoopLive: true, Due: e.Due, PR: e.PR, WaitS: int(e.Wait / time.Second), BoundS: int(bound / time.Second),
		PoolLive: pool.Live, PoolWidth: pool.Width, Line: e.Line(),
	}, nil
}

type openPRDoc struct {
	Number     int    `json:"number"`
	IsDraft    bool   `json:"isDraft"`
	Mergeable  string `json:"mergeable"`
	HeadRefOid string `json:"headRefOid"`
}

func waitingBoundaryPRs(ctx context.Context, root string, p boundaryPlane) ([]mergewindow.WaitingPR, error) {
	var prs []openPRDoc
	if err := decodeGH(ctx, p.gh, &prs, "pr", "list", "--state", "open", "--base", "main", "--json", "number,isDraft,mergeable,headRefOid", "--limit", prListLimit); err != nil {
		return nil, err
	}
	var waiting []mergewindow.WaitingPR
	for _, pr := range prs {
		w, waits, err := waitOf(ctx, root, p, pr)
		if err != nil {
			return nil, err
		}
		if waits {
			waiting = append(waiting, w)
		}
	}
	return waiting, nil
}

func waitOf(ctx context.Context, root string, p boundaryPlane, pr openPRDoc) (mergewindow.WaitingPR, bool, error) {
	if pr.IsDraft || pr.Mergeable != prMergeable {
		return mergewindow.WaitingPR{}, false, nil
	}
	since, green, err := greenSince(ctx, p.gh, pr.HeadRefOid)
	if err != nil || !green {
		return mergewindow.WaitingPR{}, false, err
	}
	n := strconv.Itoa(pr.Number)
	d, err := decidePRWindow(ctx, root, prWindowArgs{pr: n}, p.gh)
	if err != nil || d.Verdict == mergewindow.Window {
		return mergewindow.WaitingPR{}, false, err
	}
	return mergewindow.WaitingPR{Number: n, Wait: p.now().Sub(since)}, true, nil
}

func greenSince(ctx context.Context, gh func(context.Context, ...string) ([]byte, error), head string) (time.Time, bool, error) {
	var runs []struct {
		Status     string    `json:"status"`
		Conclusion string    `json:"conclusion"`
		UpdatedAt  time.Time `json:"updatedAt"`
	}
	if err := decodeGH(ctx, gh, &runs, "run", "list", "--workflow", ciparity.RequiredWorkflow, "--commit", head, "--limit", "1", "--json", "status,conclusion,updatedAt"); err != nil {
		return time.Time{}, false, err
	}
	if len(runs) == 0 || runs[0].Status != ciwatch.StatusCompleted || runs[0].Conclusion != ciwatch.ConclusionSuccess {
		return time.Time{}, false, nil
	}
	return runs[0].UpdatedAt, true, nil
}

func writePRBoundary(w io.Writer, rep prBoundaryReport, asJSON bool) error {
	if asJSON {
		return json.NewEncoder(w).Encode(rep)
	}
	_, err := fmt.Fprintln(w, rep.Line)
	return err
}
