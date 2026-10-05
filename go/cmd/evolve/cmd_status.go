package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/ciwatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/dashboard"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

const (
	statusGHTimeout  = 10 * time.Second
	statusMainBranch = "main"
)

type statusStreak struct {
	ConsecutiveShipped int                    `json:"consecutive_shipped"`
	LastZeroShipRun    *dashboard.ZeroShipRun `json:"last_zero_ship_run"`
	Closed             int                    `json:"closed"`
	Shipped            int                    `json:"shipped"`
}

type statusRemote struct {
	Available bool   `json:"available"`
	Items     any    `json:"items,omitempty"`
	Error     string `json:"error,omitempty"`
}

type statusPR struct {
	Number        int      `json:"number"`
	Title         string   `json:"title"`
	HeadRefName   string   `json:"headRefName"`
	URL           string   `json:"url"`
	Checks        string   `json:"checks"`
	FailingChecks []string `json:"failing_checks,omitempty"`
}

type statusCIRun struct {
	Workflow    string   `json:"workflow"`
	Branch      string   `json:"branch"`
	Status      string   `json:"status"`
	Conclusion  string   `json:"conclusion,omitempty"`
	URL         string   `json:"url,omitempty"`
	FailingJobs []string `json:"failing_jobs,omitempty"`
}

type ghPRCheck struct {
	Name       string `json:"name"`
	Context    string `json:"context"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	State      string `json:"state"`
}

type ghPR struct {
	Number      int         `json:"number"`
	Title       string      `json:"title"`
	HeadRefName string      `json:"headRefName"`
	URL         string      `json:"url"`
	Checks      []ghPRCheck `json:"statusCheckRollup"`
}

type statusReport struct {
	Loop   dashboard.LoopStatus     `json:"loop"`
	Cycles []dashboard.CycleSummary `json:"cycles"`
	Streak statusStreak             `json:"streak"`
	PRs    statusRemote             `json:"prs"`
	CI     statusRemote             `json:"ci"`
}

var failingCheckStates = map[string]bool{
	"FAILURE": true, "ERROR": true, "TIMED_OUT": true, "CANCELLED": true, "ACTION_REQUIRED": true, "STARTUP_FAILURE": true,
}

func runStatus(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var projectRoot string
	var asJSON bool
	fs.StringVar(&projectRoot, "project-root", "", "project root (default: $EVOLVE_PROJECT_ROOT or cwd)")
	fs.BoolVar(&asJSON, "json", false, "emit one JSON object instead of the human report")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "evolve status: unexpected argument %q (usage: evolve status [--json] [--project-root P])\n", fs.Arg(0))
		return 1
	}
	root, err := loopStopRoot(projectRoot, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "evolve status: cwd: %v\n", err)
		return 1
	}
	if err := statusSnapshotReadable(root); err != nil {
		fmt.Fprintf(stderr, "evolve status: cannot read the snapshot: %v\n", err)
		return 2
	}
	report := buildStatusReport(root, time.Now())
	if asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(stderr, "evolve status: encode: %v\n", err)
			return 1
		}
		return 0
	}
	writeStatusText(stdout, report)
	return 0
}

func statusSnapshotReadable(root string) error {
	evolveDir := paths.EvolveDirOf(root)
	info, err := os.Stat(evolveDir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", evolveDir)
	}
	return nil
}

func buildStatusReport(root string, now time.Time) statusReport {
	snap := dashboard.Collect(root, now)
	cycles := snap.Cycles
	if cycles == nil {
		cycles = []dashboard.CycleSummary{}
	}
	ctx, cancel := context.WithTimeout(context.Background(), statusGHTimeout)
	defer cancel()
	return statusReport{
		Loop:   snap.Loop,
		Cycles: cycles,
		Streak: statusStreak{
			ConsecutiveShipped: snap.Trend.ShipStreak,
			LastZeroShipRun:    snap.Trend.LastZeroShipRun,
			Closed:             snap.Trend.Closed,
			Shipped:            snap.Trend.Shipped,
		},
		PRs: statusOpenPRs(ctx, root),
		CI:  statusMainCI(ctx, root),
	}
}

func statusOpenPRs(ctx context.Context, root string) statusRemote {
	cmd := exec.CommandContext(ctx, "gh", "pr", "list", "--state", "open", "--json", "number,title,headRefName,url,statusCheckRollup")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return statusRemote{Error: "gh pr list: " + err.Error()}
	}
	var prs []ghPR
	if err := json.Unmarshal(out, &prs); err != nil {
		return statusRemote{Error: "gh pr list: " + err.Error()}
	}
	items := make([]statusPR, 0, len(prs))
	for _, pr := range prs {
		state, failing := prCheckState(pr.Checks)
		items = append(items, statusPR{Number: pr.Number, Title: pr.Title, HeadRefName: pr.HeadRefName, URL: pr.URL, Checks: state, FailingChecks: failing})
	}
	return statusRemote{Available: true, Items: items}
}

func prCheckState(checks []ghPRCheck) (string, []string) {
	if len(checks) == 0 {
		return "none", nil
	}
	state := "passing"
	var failing []string
	for _, c := range checks {
		verdict := strings.ToUpper(c.Conclusion)
		if verdict == "" {
			verdict = strings.ToUpper(c.State)
		}
		switch {
		case failingCheckStates[verdict]:
			failing = append(failing, c.Name+c.Context)
		case verdict == "" || verdict == "PENDING" || verdict == "EXPECTED" || (c.Status != "" && !strings.EqualFold(c.Status, "COMPLETED")):
			state = "pending"
		}
	}
	if len(failing) > 0 {
		return "failing", failing
	}
	return state, nil
}

func statusMainCI(ctx context.Context, root string) statusRemote {
	st, err := ciwatch.LatestRequiredRunOnBranch(ctx, root, statusMainBranch)
	if err != nil {
		return statusRemote{Error: err.Error()}
	}
	return statusRemote{Available: true, Items: []statusCIRun{{
		Workflow: ciparity.RequiredWorkflow, Branch: statusMainBranch, Status: st.Status,
		Conclusion: st.Conclusion, URL: st.RunURL, FailingJobs: st.FailingJobs,
	}}}
}

func writeStatusLoopLine(w io.Writer, l dashboard.LoopStatus) {
	fmt.Fprintf(w, "loop:    running=%t brake=%t cycle=%d phase=%s\n", l.Running, l.BrakeEngaged, l.CycleID, l.Phase)
}

func writeStatusText(w io.Writer, r statusReport) {
	writeStatusLoopLine(w, r.Loop)
	fmt.Fprintf(w, "cycles:  %d rendered\n", len(r.Cycles))
	for _, c := range r.Cycles {
		fmt.Fprintf(w, "  cycle %d  %s\n", c.ID, c.StateName)
	}
	fmt.Fprintf(w, "streak:  %d consecutive shipped (shipped %d of %d closed)\n", r.Streak.ConsecutiveShipped, r.Streak.Shipped, r.Streak.Closed)
	if z := r.Streak.LastZeroShipRun; z != nil {
		fmt.Fprintf(w, "         last zero-ship run: cycles %d-%d (%d)\n", z.FirstCycle, z.LastCycle, z.Length)
	} else {
		fmt.Fprintln(w, "         last zero-ship run: none")
	}
	writeStatusPRs(w, r.PRs)
	writeStatusCI(w, r.CI)
}

func writeStatusPRs(w io.Writer, r statusRemote) {
	items, _ := r.Items.([]statusPR)
	if !r.Available {
		fmt.Fprintf(w, "prs:     unavailable (%s)\n", r.Error)
		return
	}
	fmt.Fprintf(w, "prs:     %d open\n", len(items))
	for _, pr := range items {
		fmt.Fprintf(w, "  #%d %s [%s] checks %s", pr.Number, pr.Title, pr.HeadRefName, pr.Checks)
		if len(pr.FailingChecks) > 0 {
			fmt.Fprintf(w, ": %s", strings.Join(pr.FailingChecks, ", "))
		}
		fmt.Fprintln(w)
	}
}

func writeStatusCI(w io.Writer, r statusRemote) {
	runs, _ := r.Items.([]statusCIRun)
	if !r.Available || len(runs) == 0 {
		fmt.Fprintf(w, "ci:      unavailable (%s)\n", r.Error)
		return
	}
	run := runs[0]
	fmt.Fprintf(w, "ci:      %s on %s: %s %s %s\n", run.Workflow, run.Branch, run.Status, run.Conclusion, run.URL)
	if len(run.FailingJobs) > 0 {
		fmt.Fprintf(w, "         failing jobs: %s\n", strings.Join(run.FailingJobs, ", "))
	}
}
