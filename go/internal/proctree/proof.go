package proctree

import (
	"slices"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

type Proof func(Process) bool

type Identity struct {
	Pid     int
	Started time.Time
}

func (p Process) Identity() Identity { return Identity{Pid: p.Pid, Started: p.Started} }

func Select(table []Process, proof Proof) []Process {
	var out []Process
	for _, p := range table {
		if proof(p) {
			out = append(out, p)
		}
	}
	return out
}

func TaggedWith(id string) Proof {
	return func(p Process) bool {
		tag, ok := p.Env[ipcenv.DispatchIDKey]
		return ok && id != "" && tag == id
	}
}

func Recorded(ids []Identity) Proof {
	set := make(map[Identity]bool, len(ids))
	for _, id := range ids {
		set[Identity{Pid: id.Pid, Started: id.Started.UTC()}] = true
	}
	return func(p Process) bool {
		return set[Identity{Pid: p.Pid, Started: p.Started.UTC()}]
	}
}

func AnyOf(proofs ...Proof) Proof {
	return func(p Process) bool {
		return slices.ContainsFunc(proofs, func(proof Proof) bool { return proof(p) })
	}
}

func Descendants(table []Process, roots []int) []Process {
	inTree := make(map[int]bool, len(roots))
	for _, r := range roots {
		if r > 1 {
			inTree[r] = true
		}
	}
	for grew := true; grew; {
		grew = false
		for _, p := range table {
			if !inTree[p.Pid] && inTree[p.Ppid] {
				inTree[p.Pid], grew = true, true
			}
		}
	}
	return Select(table, func(p Process) bool { return inTree[p.Pid] })
}

func Identities(procs []Process) []Identity {
	out := make([]Identity, 0, len(procs))
	for _, p := range procs {
		out = append(out, p.Identity())
	}
	return out
}
