package gc

import (
	"reflect"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/proctree"
)

const gcRoot = "/hub/runtime"

var gcNow = time.Date(2026, 10, 8, 15, 0, 0, 0, time.UTC)

func gcProc(pid, ppid int, comm string, env map[string]string, args ...string) proctree.Process {
	return proctree.Process{Pid: pid, Ppid: ppid, Pgid: pid, Started: gcNow.Add(-48 * time.Hour), Comm: comm, Args: args, Env: env}
}

func dispatchEnv(id string) map[string]string {
	return map[string]string{ipcenv.DispatchIDKey: id, ProjectRootEnvKey: gcRoot}
}

func staleOptions() DispatchProcessOptions {
	return DispatchProcessOptions{
		ProjectRoot: gcRoot,
		PidAlive:    func(pid int) bool { return pid == 4242 || pid == 5000 },
		CycleClosed: func(cycle int) bool { return cycle == 1800 },
		RecordedTree: func(id string) []proctree.Identity {
			if id == "01R/1800/build/p4242n1" || id == "01R/1835/build/p999n1" {
				return []proctree.Identity{{Pid: 30, Started: gcNow.Add(-48 * time.Hour)}, {Pid: 31, Started: gcNow.Add(-48 * time.Hour)}}
			}
			return nil
		},
		Now:     gcNow,
		TailTTL: 24 * time.Hour,
	}
}

func TestStaleDispatch_RefusesEveryProcessWithoutTheProof(t *testing.T) {
	t.Parallel()
	proof := StaleDispatch(staleOptions())
	cases := []struct {
		name string
		p    proctree.Process
		stop bool
	}{
		{"an MCP server whose bridge is dead", gcProc(10, 1, "node", dispatchEnv("01R/1835/build/p999n1")), true},
		{"a child of a dead bridge that still has a parent", gcProc(11, 300, "node", dispatchEnv("01R/1835/build/p999n1")), true},
		{"an orphan of a closed cycle whose bridge pid is alive, outside the recorded tree", gcProc(12, 1, "node", dispatchEnv("01R/1800/build/p4242n1")), false},
		{"an orphan of a closed cycle whose bridge pid is alive, in the recorded tree", gcProc(30, 1, "node", dispatchEnv("01R/1800/build/p4242n1")), true},
		{"a live dispatch: bridge alive, cycle open", gcProc(13, 1, "node", dispatchEnv("01R/1835/build/p4242n1")), false},
		{"an attached process of a closed cycle whose bridge is alive", gcProc(14, 300, "node", dispatchEnv("01R/1800/build/p4242n1")), false},
		{"an operator shell with the project root and no tag", gcProc(15, 1, "zsh", map[string]string{ProjectRootEnvKey: gcRoot}), false},
		{"a tagged process of a different project", gcProc(16, 1, "node", map[string]string{ipcenv.DispatchIDKey: "01R/1835/build/p999n1", ProjectRootEnvKey: "/other"}), false},
		{"a tagged process with no project root", gcProc(17, 1, "node", map[string]string{ipcenv.DispatchIDKey: "01R/1835/build/p999n1"}), false},
		{"a malformed tag", gcProc(18, 1, "node", dispatchEnv("garbage")), false},
		{"Chrome", gcProc(19, 1, "Google Chrome Helper", map[string]string{}), false},
		{"an interactive claude session", gcProc(20, 900, "claude", map[string]string{ProjectRootEnvKey: gcRoot}), false},
		{"a process whose environment is hidden", gcProc(21, 1, "/bin/zsh", nil), false},
		{"the tag only in the arguments", gcProc(22, 1, "grep", map[string]string{ProjectRootEnvKey: gcRoot}, "grep", ipcenv.DispatchIDKey+"=01R/1835/build/p999n1"), false},
	}
	for _, tc := range cases {
		if got := proof(tc.p); got != tc.stop {
			t.Errorf("%s: stop=%v, want %v", tc.name, got, tc.stop)
		}
	}
}

func TestOrphanLogTail_Table(t *testing.T) {
	t.Parallel()
	proof := OrphanLogTail(staleOptions())
	young := gcProc(40, 1, "/usr/bin/tail", nil, "tail", "-F", gcRoot+"/.evolve/boundary-loop.log")
	young.Started = gcNow.Add(-time.Hour)
	cases := []struct {
		name string
		p    proctree.Process
		stop bool
	}{
		{"an old console monitor", gcProc(30, 1, "/usr/bin/tail", nil, "/usr/bin/tail", "-F", gcRoot+"/.evolve/boundary-loop.log"), true},
		{"two files and a line count", gcProc(31, 1, "tail", nil, "tail", "-n", "50", "-F", gcRoot+"/.evolve/a.log", gcRoot+"/.evolve/runs/cycle-1/b.log"), true},
		{"an attached tail", gcProc(32, 700, "/usr/bin/tail", nil, "tail", "-F", gcRoot+"/.evolve/a.log"), false},
		{"a young tail", young, false},
		{"a tail of a file outside .evolve", gcProc(33, 1, "/usr/bin/tail", nil, "tail", "-F", gcRoot+"/.evolve/a.log", "/var/log/system.log"), false},
		{"a relative path", gcProc(34, 1, "/usr/bin/tail", nil, "tail", "-F", ".evolve/a.log"), false},
		{"a path that climbs out", gcProc(35, 1, "/usr/bin/tail", nil, "tail", "-F", gcRoot+"/.evolve/../secrets.log"), false},
		{"a sibling with the same prefix", gcProc(36, 1, "/usr/bin/tail", nil, "tail", "-F", gcRoot+"/.evolve-old/a.log"), false},
		{"no file argument", gcProc(37, 1, "/usr/bin/tail", nil, "tail", "-F"), false},
		{"unreadable arguments", gcProc(38, 1, "/usr/bin/tail", nil), false},
		{"a different program", gcProc(39, 1, "/usr/bin/less", nil, "less", gcRoot+"/.evolve/a.log"), false},
		{"a tail whose argv[0] is not tail", gcProc(41, 1, "/usr/bin/tail", nil, "node", gcRoot+"/.evolve/a.log"), false},
		{"the old plus syntax", gcProc(42, 1, "/usr/bin/tail", nil, "tail", "+5", gcRoot+"/.evolve/a.log"), false},
	}
	for _, tc := range cases {
		if got := proof(tc.p); got != tc.stop {
			t.Errorf("%s: stop=%v, want %v", tc.name, got, tc.stop)
		}
	}
}

func TestOrphanLogTail_AZeroTTLTurnsTheRuleOff(t *testing.T) {
	t.Parallel()
	o := staleOptions()
	o.TailTTL = 0

	if OrphanLogTail(o)(gcProc(30, 1, "/usr/bin/tail", nil, "tail", "-F", gcRoot+"/.evolve/a.log")) {
		t.Errorf("a zero gc.temp_ttl_hours must turn the tail rule off")
	}
}

func TestPlanDispatchProcesses_LabelsEachItemWithItsRule(t *testing.T) {
	t.Parallel()
	table := []proctree.Process{
		gcProc(10, 1, "node", dispatchEnv("01R/1835/build/p999n1")),
		gcProc(30, 1, "/usr/bin/tail", nil, "tail", "-F", gcRoot+"/.evolve/a.log"),
		gcProc(50, 1, "Google Chrome", nil),
	}

	got := PlanDispatchProcesses(table, staleOptions())

	want := []DispatchProcessItem{{Process: table[0], Rule: RuleStaleDispatch}, {Process: table[1], Rule: RuleLogTail}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("plan = %+v, want %+v", got, want)
	}
}

func TestOrphanLogTail_ARelativeRootMatchesNothing(t *testing.T) {
	t.Parallel()
	o := staleOptions()
	o.ProjectRoot = "."

	if OrphanLogTail(o)(gcProc(30, 1, "/usr/bin/tail", nil, "tail", "-F", ".evolve/a.log")) {
		t.Errorf("a relative root matched a relative tail path: the cwd of the tail is unknown")
	}
}

func TestStaleDispatch_NeverStopsATaggedSharedHelperOutsideTheRecordedTree(t *testing.T) {
	t.Parallel()
	proof := StaleDispatch(staleOptions())
	dead := dispatchEnv("01R/1835/build/p999n1")
	cases := []struct {
		name string
		p    proctree.Process
		stop bool
	}{
		{"a Claude Code daemon", gcProc(60, 1, "claude", dead, "claude", "daemon", "run"), false},
		{"a Chrome for a browser MCP", gcProc(61, 1, "Google Chrome", dead), false},
		{"a tmux server", gcProc(62, 1, "/opt/homebrew/bin/tmux", dead, "tmux", "-L", "evolve-bridge", "new-session"), false},
		{"a Claude Code daemon in the persisted tree", gcProc(31, 1, "claude", dead, "claude", "daemon", "run"), false},
		{"a tmux server in the persisted tree of a closed cycle", gcProc(30, 1, "tmux", dispatchEnv("01R/1800/build/p4242n1"), "tmux", "new-session"), false},
		{"an MCP server outside the tree", gcProc(63, 1, "node", dead, "node", "mcp.js"), true},
	}
	for _, tc := range cases {
		if got := proof(tc.p); got != tc.stop {
			t.Errorf("%s: stop=%v, want %v", tc.name, got, tc.stop)
		}
	}
}

func TestStaleDispatch_ARelativeRootMatchesNothing(t *testing.T) {
	t.Parallel()
	o := staleOptions()
	o.ProjectRoot = "hub/runtime"
	env := map[string]string{ipcenv.DispatchIDKey: "01R/1835/build/p999n1", ProjectRootEnvKey: "hub/runtime"}

	if StaleDispatch(o)(gcProc(10, 1, "node", env)) {
		t.Errorf("a relative project root matched: the root of the gc run must be absolute")
	}
}
