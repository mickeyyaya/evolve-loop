package gc

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
)

type Process struct {
	Pid  int    `json:"pid"`
	Ppid int    `json:"ppid"`
	Cwd  string `json:"cwd"`
}

type ProcessReapReport struct {
	Reaped []Process `json:"reaped"`
	Errors []string  `json:"errors,omitempty"`
}

func ReapFinishedCycleOrphans(ctx context.Context, o WorktreeOptions, kill func(pid int) error) ProcessReapReport {
	var rep ProcessReapReport
	procs, err := o.listProcesses(ctx)
	if err != nil {
		rep.Errors = append(rep.Errors, err.Error())
		return rep
	}
	for _, p := range o.FinishedCycleOrphans(procs) {
		if err := kill(p.Pid); err != nil {
			rep.Errors = append(rep.Errors, fmt.Sprintf("signal pid %d (cwd %s): %v", p.Pid, p.Cwd, err))
			continue
		}
		rep.Reaped = append(rep.Reaped, p)
	}
	return rep
}

func (o WorktreeOptions) FinishedCycleOrphans(procs []Process) []Process {
	var out []Process
	for _, p := range procs {
		if p.Ppid == 1 && o.finishedCycleTree(p.Cwd) {
			out = append(out, p)
		}
	}
	return out
}

func (o WorktreeOptions) finishedCycleTree(cwd string) bool {
	leaf, ok := leafUnder(o.WorktreeBase, cwd)
	if !ok {
		return false
	}
	n, ok := leafCycleNumber(leaf)
	if !strings.HasPrefix(leaf, "cycle-") || !ok {
		return false
	}
	if _, err := os.Stat(filepath.Join(o.WorktreeBase, leaf)); os.IsNotExist(err) {
		return true
	}
	return dossier.ClosedOut(o.ProjectRoot, n)
}

func leafUnder(base, p string) (string, bool) {
	for _, b := range []string{filepath.Clean(base), resolvePath(base)} {
		rel, err := filepath.Rel(b, p)
		if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		return strings.SplitN(rel, string(filepath.Separator), 2)[0], true
	}
	return "", false
}

func (o WorktreeOptions) listProcesses(ctx context.Context) ([]Process, error) {
	var out strings.Builder
	code, err := o.Exec(ctx, "lsof", o.ProjectRoot, []string{"-a", "-d", "cwd", "-u", uidArg(), "-R", "-F", "pRn"}, nil, nil, &out, nil)
	if err != nil {
		return nil, fmt.Errorf("gc: list process cwds: %w", err)
	}
	if code != 0 && out.Len() == 0 {
		return nil, fmt.Errorf("gc: list process cwds: lsof exit %d with no output", code)
	}
	return parseCwdListing(out.String()), nil
}

func uidArg() string { return strconv.Itoa(os.Getuid()) }

func parseCwdListing(s string) []Process {
	var out []Process
	var cur Process
	flush := func() {
		if cur.Pid > 0 && cur.Cwd != "" {
			out = append(out, cur)
		}
		cur = Process{}
	}
	sc := bufio.NewScanner(strings.NewReader(s))
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		switch line[0] {
		case 'p':
			flush()
			cur.Pid, _ = strconv.Atoi(line[1:])
		case 'R':
			cur.Ppid, _ = strconv.Atoi(line[1:])
		case 'n':
			cur.Cwd = line[1:]
		}
	}
	flush()
	return out
}
