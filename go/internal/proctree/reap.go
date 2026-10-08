package proctree

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"syscall"
	"time"
)

type Lister func(context.Context) ([]Process, error)

type Signaler func(pid int, sig syscall.Signal) error

type Reaper struct {
	List      Lister
	Signal    Signaler
	Sleep     func(time.Duration)
	Grace     time.Duration
	Protected []int
	Self      int
}

type Report struct {
	Terminated []Process
	Killed     []Process
	Survivors  []Process
	Errors     []string
}

const killSettle = 200 * time.Millisecond

func (r Reaper) Reap(ctx context.Context, proof Proof) Report {
	var rep Report
	table, err := r.List(ctx)
	if err != nil {
		rep.Errors = append(rep.Errors, fmt.Sprintf("list processes: %v", err))
		return rep
	}
	if r.Self != 0 && !slices.ContainsFunc(table, func(p Process) bool { return p.Pid == r.Self }) {
		rep.Errors = append(rep.Errors, fmt.Sprintf("refuse to reap: the reaper pid %d is not in the process table, so its ancestors are unknown", r.Self))
		return rep
	}
	targets := Select(table, proof)
	if len(targets) == 0 {
		return rep
	}
	protected := r.protected(table)
	rep.Terminated = r.send(targets, syscall.SIGTERM, protected, &rep)
	r.Sleep(r.Grace)
	left := r.stillMatching(ctx, rep.Terminated, proof, &rep)
	if len(left) == 0 {
		return rep
	}
	rep.Killed = r.send(left, syscall.SIGKILL, protected, &rep)
	r.Sleep(killSettle)
	rep.Survivors = r.stillMatching(ctx, rep.Killed, proof, &rep)
	return rep
}

func (r Reaper) protected(table []Process) []int {
	prot := slices.Clone(r.Protected)
	byPid := make(map[int]int, len(table))
	for _, p := range table {
		byPid[p.Pid] = p.Ppid
	}
	for pid, seen := r.Self, map[int]bool{}; pid > 1 && !seen[pid]; pid = byPid[pid] {
		seen[pid] = true
		prot = append(prot, pid)
	}
	return prot
}

func (r Reaper) send(targets []Process, sig syscall.Signal, protected []int, rep *Report) []Process {
	var done []Process
	for _, p := range targets {
		if p.Pid <= 1 || slices.Contains(protected, p.Pid) {
			rep.Errors = append(rep.Errors, fmt.Sprintf("refuse to signal protected pid %d (%s)", p.Pid, p.Comm))
			continue
		}
		if err := r.Signal(p.Pid, sig); err != nil && !errors.Is(err, syscall.ESRCH) {
			rep.Errors = append(rep.Errors, fmt.Sprintf("signal %v to pid %d (%s): %v", sig, p.Pid, p.Comm, err))
			continue
		}
		done = append(done, p)
	}
	return done
}

func (r Reaper) stillMatching(ctx context.Context, sent []Process, proof Proof, rep *Report) []Process {
	table, err := r.List(ctx)
	if err != nil {
		rep.Errors = append(rep.Errors, fmt.Sprintf("list processes again: %v", err))
		return nil
	}
	return Select(table, Recorded(Identities(sent)).and(proof))
}

func (p Proof) and(other Proof) Proof {
	return func(x Process) bool { return p(x) && other(x) }
}
