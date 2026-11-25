package gc

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/proctree"
)

const ProjectRootEnvKey = "EVOLVE_PROJECT_ROOT"

const (
	RuleStaleDispatch = "stale-dispatch"
	RuleLogTail       = "log-tail"
	tailProgram       = "tail"
)

var tailValueFlags = map[string]bool{"-n": true, "-c": true, "-b": true}

type DispatchProcessOptions struct {
	ProjectRoot  string
	PidAlive     func(int) bool
	CycleClosed  func(int) bool
	RecordedTree func(string) []proctree.Identity
	Killable     proctree.Proof
	Now          time.Time
	TailTTL      time.Duration
}

type DispatchProcessItem struct {
	Process proctree.Process
	Rule    string
}

func (o DispatchProcessOptions) tag(p proctree.Process) (string, proctree.DispatchID, bool) {
	root := filepath.Clean(o.ProjectRoot)
	tag, tagged := p.Env[ipcenv.DispatchIDKey]
	id, ok := proctree.ParseDispatchID(tag)
	if !tagged || !ok || !filepath.IsAbs(root) || p.Env[ProjectRootEnvKey] == "" || filepath.Clean(p.Env[ProjectRootEnvKey]) != root {
		return "", proctree.DispatchID{}, false
	}
	return tag, id, true
}

func StaleDispatch(o DispatchProcessOptions) proctree.Proof {
	return func(p proctree.Process) bool {
		tag, id, ok := o.tag(p)
		if !ok {
			return false
		}
		d := proctree.Dispatch{ID: tag, Tree: o.RecordedTree(tag), Killable: o.Killable}
		if !o.PidAlive(id.Owner) {
			return d.Owned()(p)
		}
		return p.Ppid == 1 && o.CycleClosed(id.Cycle) && proctree.Recorded(d.Tree)(p) && proctree.NotShared(p)
	}
}

const (
	RuleShared      = "shared"
	RuleNotKillable = "not-killable"
)

type UnreapedItem struct {
	Process proctree.Process
	Rule    string
}

func UnreapedTagged(table []proctree.Process, o DispatchProcessOptions) []UnreapedItem {
	reaped := StaleDispatch(o)
	var items []UnreapedItem
	for _, p := range table {
		_, id, ok := o.tag(p)
		if !ok || o.PidAlive(id.Owner) || reaped(p) {
			continue
		}
		rule := RuleNotKillable
		if proctree.SharedHelper(p) {
			rule = RuleShared
		}
		items = append(items, UnreapedItem{Process: p, Rule: rule})
	}
	return items
}

func OrphanLogTail(o DispatchProcessOptions) proctree.Proof {
	logDir := filepath.Join(filepath.Clean(o.ProjectRoot), ".evolve") + string(filepath.Separator)
	return func(p proctree.Process) bool {
		if o.TailTTL <= 0 || !filepath.IsAbs(o.ProjectRoot) || p.Ppid != 1 || filepath.Base(p.Comm) != tailProgram || len(p.Args) == 0 || filepath.Base(p.Args[0]) != tailProgram {
			return false
		}
		if o.Now.Sub(p.Started) <= o.TailTTL {
			return false
		}
		files := tailFiles(p.Args[1:])
		return len(files) > 0 && allUnder(files, logDir)
	}
}

func tailFiles(args []string) []string {
	var files []string
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--":
			return append(files, args[i+1:]...)
		case tailValueFlags[a]:
			i++
		case strings.HasPrefix(a, "-"):
		default:
			files = append(files, a)
		}
	}
	return files
}

func allUnder(files []string, dir string) bool {
	for _, f := range files {
		if !strings.HasPrefix(filepath.Clean(f), dir) {
			return false
		}
	}
	return true
}

func PlanDispatchProcesses(table []proctree.Process, o DispatchProcessOptions) []DispatchProcessItem {
	rules := []struct {
		name  string
		proof proctree.Proof
	}{{RuleStaleDispatch, StaleDispatch(o)}, {RuleLogTail, OrphanLogTail(o)}}
	var items []DispatchProcessItem
	for _, p := range table {
		for _, r := range rules {
			if r.proof(p) {
				items = append(items, DispatchProcessItem{Process: p, Rule: r.name})
				break
			}
		}
	}
	return items
}
