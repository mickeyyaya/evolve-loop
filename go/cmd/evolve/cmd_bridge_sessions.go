package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/panewatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/swarm"
)

const bridgeSessionsUsage = "Usage: evolve bridge sessions [--project-root=DIR] [--json]"

const bridgeSessionsTmuxTimeout = 5 * time.Second

type bridgeSessionRow struct {
	Cycle        int    `json:"cycle"`
	Phase        string `json:"phase"`
	CLI          string `json:"cli"`
	Model        string `json:"model"`
	State        string `json:"state"`
	ProgressAgeS int64  `json:"progress_age_s"`
	DrawnAgeS    int64  `json:"drawn_age_s"`
	TokenLine    string `json:"token_line"`
	Session      string `json:"session"`
	Socket       string `json:"socket"`
}

var bridgeSessionsTmux = func(ctx context.Context, socket string) (string, error) {
	cctx, cancel := context.WithTimeout(ctx, bridgeSessionsTmuxTimeout)
	defer cancel()
	out, err := exec.CommandContext(cctx, "tmux", "-L", socket, "list-sessions", "-F", "#{session_name}\t#{window_activity}").Output()
	return string(out), err
}

var bridgeLiveSockets = liveLoopSockets

var bridgeSessionsNow = time.Now

func liveLoopSockets() []string {
	names, err := swarm.ExecListBridgeSockets()
	if err != nil {
		return nil
	}
	var live []string
	for _, name := range names {
		pid, err := strconv.Atoi(strings.TrimPrefix(name, "evolve-bridge-p"))
		if err == nil && pid > 0 && swarm.ExecPidAlive(pid) {
			live = append(live, name)
		}
	}
	return live
}

func runBridgeSessions(args []string, stdout, stderr io.Writer) int {
	root, asJSON := envOrCwd("EVOLVE_PROJECT_ROOT"), false
	for _, a := range args {
		switch {
		case strings.HasPrefix(a, "--project-root="):
			root = strings.TrimPrefix(a, "--project-root=")
		case a == "--json":
			asJSON = true
		case a == "--help" || a == "-h":
			fmt.Fprintln(stdout, bridgeSessionsUsage)
			return 0
		default:
			fmt.Fprintf(stderr, "evolve bridge sessions: unknown argument %q\n%s\n", a, bridgeSessionsUsage)
			return 10
		}
	}
	rows := collectBridgeSessions(root, bridgeSessionsNow(), stderr)
	if asJSON {
		return writeSessionsJSON(rows, stdout, stderr)
	}
	writeSessionsTable(rows, stdout)
	return 0
}

func collectBridgeSessions(root string, now time.Time, stderr io.Writer) []bridgeSessionRow {
	snaps := liveSnapshots(root, now, stderr)
	activity := socketActivity(socketsOf(snaps), stderr)
	rows := make([]bridgeSessionRow, 0, len(snaps))
	watched := map[string]bool{}
	for _, s := range snaps {
		watched[s.Session] = true
		rows = append(rows, snapshotRow(s, activity, now))
	}
	return append(rows, unwatchedRows(activity, watched, now)...)
}

func liveSnapshots(root string, now time.Time, stderr io.Writer) []panewatch.Snapshot {
	var snaps []panewatch.Snapshot
	for _, run := range runlease.LiveRuns(filepath.Join(root, ".evolve", "runs"), now) {
		found, err := panewatch.ReadAll(run.Dir)
		if err != nil {
			fmt.Fprintf(stderr, "evolve bridge sessions: WARN %s: %v\n", run.Dir, err)
		}
		snaps = append(snaps, liveWriters(found)...)
	}
	sort.SliceStable(snaps, func(i, j int) bool { return snaps[i].Cycle < snaps[j].Cycle })
	return snaps
}

func liveWriters(snaps []panewatch.Snapshot) []panewatch.Snapshot {
	var live []panewatch.Snapshot
	for _, s := range snaps {
		if s.WriterAlive(runlease.PIDAlive) {
			live = append(live, s)
		}
	}
	return live
}

func socketsOf(snaps []panewatch.Snapshot) []string {
	seen := map[string]bool{}
	var sockets []string
	for _, s := range append(snapshotSockets(snaps), bridgeLiveSockets()...) {
		if s != "" && !seen[s] {
			seen[s] = true
			sockets = append(sockets, s)
		}
	}
	return sockets
}

func snapshotSockets(snaps []panewatch.Snapshot) []string {
	out := make([]string, 0, len(snaps))
	for _, s := range snaps {
		out = append(out, s.Socket)
	}
	return out
}

type sessionActivity struct {
	drawn       map[string]time.Time
	socketOf    map[string]string
	unreachable map[string]bool
}

func socketActivity(sockets []string, stderr io.Writer) sessionActivity {
	act := sessionActivity{drawn: map[string]time.Time{}, socketOf: map[string]string{}, unreachable: map[string]bool{}}
	for _, socket := range sockets {
		out, err := bridgeSessionsTmux(context.Background(), socket)
		if err != nil {
			act.unreachable[socket] = true
			fmt.Fprintf(stderr, "evolve bridge sessions: WARN tmux socket %s unreachable: %v\n", socket, err)
			continue
		}
		for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
			name, ts, ok := strings.Cut(line, "\t")
			if secs, err := strconv.ParseInt(strings.TrimSpace(ts), 10, 64); ok && err == nil {
				act.drawn[name], act.socketOf[name] = time.Unix(secs, 0), socket
			}
		}
	}
	return act
}

func snapshotRow(s panewatch.Snapshot, act sessionActivity, now time.Time) bridgeSessionRow {
	row := bridgeSessionRow{
		Cycle: s.Cycle, Phase: s.Agent, CLI: s.CLI, Model: firstNonEmpty(s.ModelLabel, s.Model),
		State: busyWord(s.Busy), ProgressAgeS: ageSeconds(now, s.ProgressAt), DrawnAgeS: -1,
		TokenLine: s.TokenLine, Session: s.Session, Socket: s.Socket,
	}
	drawn, alive := act.drawn[s.Session]
	switch {
	case alive:
		row.DrawnAgeS = ageSeconds(now, drawn)
	case !act.unreachable[s.Socket]:
		row.State = "gone"
	}
	return row
}

func unwatchedRows(act sessionActivity, watched map[string]bool, now time.Time) []bridgeSessionRow {
	var names []string
	for name := range act.drawn {
		if !watched[name] {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	rows := make([]bridgeSessionRow, 0, len(names))
	for _, name := range names {
		rows = append(rows, bridgeSessionRow{Phase: "-", CLI: "-", Model: "-", State: "unwatched", ProgressAgeS: -1,
			DrawnAgeS: ageSeconds(now, act.drawn[name]), Session: name, Socket: act.socketOf[name]})
	}
	return rows
}

func busyWord(busy bool) string {
	if busy {
		return "busy"
	}
	return "idle"
}

func ageSeconds(now, then time.Time) int64 {
	if then.IsZero() {
		return -1
	}
	return int64(now.Sub(then).Round(time.Second) / time.Second)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func writeSessionsJSON(rows []bridgeSessionRow, stdout, stderr io.Writer) int {
	data, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "evolve bridge sessions: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, string(data))
	return 0
}

func writeSessionsTable(rows []bridgeSessionRow, stdout io.Writer) {
	if len(rows) == 0 {
		fmt.Fprintln(stdout, "no live bridge sessions")
		return
	}
	tw := tabwriter.NewWriter(stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "CYCLE\tPHASE\tCLI\tMODEL\tSTATE\tPROGRESS\tDRAWN\tTOKENS\tSESSION")
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", cycleCell(r.Cycle), r.Phase, r.CLI, r.Model, r.State,
			ageCell(r.ProgressAgeS), ageCell(r.DrawnAgeS), firstNonEmpty(r.TokenLine, "-"), r.Session)
	}
	_ = tw.Flush()
}

func cycleCell(cycle int) string {
	if cycle == 0 {
		return "-"
	}
	return strconv.Itoa(cycle)
}

func ageCell(seconds int64) string {
	if seconds < 0 {
		return "-"
	}
	return (time.Duration(seconds) * time.Second).String()
}
