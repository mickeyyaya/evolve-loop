package inboxcurator

import "github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"

type Tree struct {
	HasTest    func(string) bool
	PathExists func(string) bool
}

func (t Tree) ProbesPass(it inboxbatch.Item) bool {
	if len(it.Probes) == 0 {
		return false
	}
	for _, p := range it.Probes {
		if !t.probePasses(p) {
			return false
		}
	}
	return true
}

func (t Tree) probePasses(p inboxbatch.Probe) bool {
	switch p.Kind {
	case inboxbatch.ProbeTestExists:
		return t.HasTest(p.Arg)
	case inboxbatch.ProbePathExists:
		return t.PathExists(p.Arg)
	case inboxbatch.ProbePathAbsent:
		return !t.PathExists(p.Arg)
	}
	return false
}
