package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/ciwatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/dashboard"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/qualitypanel"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
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
	ProjectRoot string                     `json:"project_root"`
	Loop        dashboard.LoopStatus       `json:"loop"`
	Quality     []dashboard.QualityReading `json:"quality,omitempty"`
	QualityMode string                     `json:"-"`
	Cycles      []dashboard.CycleSummary   `json:"cycles"`
	Streak      statusStreak               `json:"streak"`
	PRs         statusRemote               `json:"prs"`
	CI          statusRemote               `json:"ci"`
}

var statusCollect = dashboard.Collect

var failingCheckStates = map[string]bool{
	"FAILURE": true, "ERROR": true, "TIMED_OUT": true, "CANCELLED": true, "ACTION_REQUIRED": true, "STARTUP_FAILURE": true,
}

func runStatus(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var projectRoot, qualityMode string
	var asJSON bool
	fs.StringVar(&projectRoot, "project-root", "", "project root (default: $EVOLVE_PROJECT_ROOT or cwd)")
	fs.BoolVar(&asJSON, "json", false, "emit one JSON object instead of the human report")
	fs.StringVar(&qualityMode, "quality", "report", "quality panel stage filter: report or all")
	if err := fs.Parse(args); err != nil {
		return cliExitUsage
	}
	if qualityMode != "report" && qualityMode != "all" {
		fmt.Fprintf(stderr, "evolve status: unknown --quality %q (want report or all)\n", qualityMode)
		return cliExitUsage
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "evolve status: unexpected argument %q (usage: evolve status [--json] [--quality report|all] [--project-root P])\n", fs.Arg(0))
		return cliExitUsage
	}
	root, err := loopStopRoot(projectRoot, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "evolve status: cwd: %v\n", err)
		return 1
	}
	if rc := checkRootHasEvolve("status", root, stderr); rc != 0 {
		return rc
	}
	report := buildStatusReport(root, time.Now())
	report.QualityMode = qualityMode
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

type rootCheck struct{ op, root string }

func (c rootCheck) refuse(err error, stderr io.Writer) int {
	if err == nil {
		return 0
	}
	fmt.Fprintf(stderr, "evolve %s: cannot read the project root %s: %v; pass --project-root with the project to read\n", c.op, c.root, err)
	return exitIO
}

func checkRootDir(op, root string, stderr io.Writer) int {
	return rootCheck{op, root}.refuse(directoryReadable(root), stderr)
}

func checkRootHasEvolve(op, root string, stderr io.Writer) int {
	return rootCheck{op, root}.refuse(directoryReadable(paths.EvolveDirOf(root)), stderr)
}

func directoryReadable(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", dir)
	}
	return nil
}

func buildStatusReport(root string, now time.Time) statusReport {
	snap := statusCollect(root, now)
	cycles := snap.Cycles
	if cycles == nil {
		cycles = []dashboard.CycleSummary{}
	}
	ctx, cancel := context.WithTimeout(context.Background(), statusGHTimeout)
	defer cancel()
	return statusReport{
		ProjectRoot: root,
		Loop:        snap.Loop,
		Quality:     snap.Quality,
		QualityMode: "report",
		Cycles:      cycles,
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
	cmd := sysexec.Command(ctx, "gh", "pr", "list", "--state", "open", "--json", "number,title,headRefName,url,statusCheckRollup")
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
	writeStatusQuality(w, r.Quality, r.QualityMode)
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

func writeStatusQuality(w io.Writer, readings []dashboard.QualityReading, mode string) {
	if len(readings) == 0 {
		return
	}
	fmt.Fprintf(w, "Quality (%s stage; [95%% interval], n, window)\n", mode)
	for _, q := range readings {
		if mode != "all" && q.Stage == string(qualitypanel.StageShadow) {
			continue
		}
		fmt.Fprintln(w, formatQualityLine(q))
	}
}

func formatQualityLine(q dashboard.QualityReading) string {
	name := q.Index
	unit := ""
	if ix, ok := qualitypanel.Lookup(q.Index); ok {
		name = ix.Name
		unit = ix.Unit
	}
	if !q.HasReading {
		return fmt.Sprintf("  %-4s %-34s no reading yet", q.Index, name)
	}
	val := formatQualityValue(q, unit)
	mark := ""
	if q.Status == string(qualitypanel.StatusBreach) {
		mark = " !"
	}
	return fmt.Sprintf("  %-4s %-34s %-19s n=%-4d %-15s %s%s", q.Index, name, val, q.N, q.Window, q.Stage, mark)
}

func formatQualityValue(q dashboard.QualityReading, unit string) string {
	scale := 1.0
	suffix := ""
	if unit == "ratio" {
		scale = 100.0
		suffix = "%"
	}
	if q.Lo != nil && q.Hi != nil {
		return fmt.Sprintf("%.1f%s [%.1f, %.1f]", *q.Value*scale, suffix, *q.Lo*scale, *q.Hi*scale)
	}
	return fmt.Sprintf("%.1f%s", *q.Value*scale, suffix)
}
