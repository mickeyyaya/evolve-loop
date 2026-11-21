//go:build integration

package proctree

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

type psRow struct {
	ppid, pgid int
	state      string
	started    time.Time
	comm       string
}

func psSnapshot(t *testing.T) map[int]psRow {
	t.Helper()
	cmd := exec.Command("ps", "-A", "-o", "pid=,ppid=,pgid=,uid=,stat=,lstart=,comm=")
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("ps: %v", err)
	}
	rows := map[int]psRow{}
	for _, line := range strings.Split(string(out), "\n") {
		if pid, row, ok := parsePSLine(line); ok {
			rows[pid] = row
		}
	}
	return rows
}

func parsePSLine(line string) (int, psRow, bool) {
	const numeric, stamp = 4, 5
	f := strings.Fields(line)
	if len(f) < numeric+1+stamp+1 {
		return 0, psRow{}, false
	}
	n := make([]int, numeric)
	for i := range n {
		v, err := strconv.Atoi(f[i])
		if err != nil {
			return 0, psRow{}, false
		}
		n[i] = v
	}
	started, err := time.ParseInLocation("Mon Jan _2 15:04:05 2006", strings.Join(f[numeric+1:numeric+1+stamp], " "), time.Local)
	if err != nil || n[3] != os.Getuid() {
		return 0, psRow{}, false
	}
	return n[0], psRow{ppid: n[1], pgid: n[2], state: f[numeric], started: started, comm: squeeze(strings.Join(f[numeric+1+stamp:], " "))}, true
}

func (r psRow) identity() psRow {
	r.state = ""
	return r
}

func squeeze(s string) string { return strings.Join(strings.Fields(s), " ") }

func stableRows(before, after map[int]psRow) map[int]psRow {
	stable := map[int]psRow{}
	for pid, row := range before {
		if again, ok := after[pid]; ok && again.identity() == row.identity() && !strings.HasPrefix(row.state, "Z") {
			stable[pid] = row
		}
	}
	return stable
}

func TestExecLister_MatchesPSForTheOwnUIDTable(t *testing.T) {
	child := exec.Command("/bin/sleep", "30")
	child.Args[0] = "renamedproc"
	if err := child.Start(); err != nil {
		t.Fatalf("start the renamed child: %v", err)
	}
	t.Cleanup(func() { _ = child.Process.Kill(); _ = child.Wait() })
	wantChildComm := map[string]string{"darwin": "renamedproc", "linux": "sleep"}[runtime.GOOS]

	before := psSnapshot(t)
	began := time.Now()
	table, err := ExecLister()(context.Background())
	elapsed := time.Since(began)
	stable := stableRows(before, psSnapshot(t))
	if err != nil {
		t.Fatalf("ExecLister: %v", err)
	}

	compared := 0
	for _, p := range table {
		want, ok := stable[p.Pid]
		if !ok {
			continue
		}
		compared++
		if p.Ppid != want.ppid || p.Pgid != want.pgid || squeeze(p.Comm) != squeeze(want.comm) || p.Started.Sub(want.started).Abs() > time.Second {
			t.Errorf("pid %d: kernel row %+v, ps row %+v, want the same ppid, pgid, comm and a start second within 1s", p.Pid, p, want)
		}
	}
	for _, pid := range []int{os.Getpid(), os.Getppid(), child.Process.Pid} {
		if _, ok := stable[pid]; !ok {
			t.Errorf("ps has no stable row for pid %d: the comparison would skip it", pid)
		}
	}
	if got, _ := rowOf(table, child.Process.Pid); got.Comm != wantChildComm {
		t.Errorf("renamed child Comm = %q, want %q", got.Comm, wantChildComm)
	}
	t.Logf("compared %d of %d kernel rows against %d stable ps rows; ExecLister took %v", compared, len(table), len(stable), elapsed)
}
