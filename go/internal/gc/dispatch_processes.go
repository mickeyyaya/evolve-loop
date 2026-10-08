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
	Now          time.Time
	TailTTL      time.Duration
}

type DispatchProcessItem struct {
	Process proctree.Process
	Rule    string
}

func StaleDispatch(o DispatchProcessOptions) proctree.Proof {
	root := filepath.Clean(o.ProjectRoot)
	return func(p proctree.Process) bool {
		tag, tagged := p.Env[ipcenv.DispatchIDKey]
		id, ok := proctree.ParseDispatchID(tag)
		if !tagged || !ok || !filepath.IsAbs(root) || p.Env[ProjectRootEnvKey] == "" || filepath.Clean(p.Env[ProjectRootEnvKey]) != root {
			return false
		}
		tree := o.RecordedTree(tag)
		if !o.PidAlive(id.Owner) {
			return proctree.OwnedByDispatch(tag, tree, nil)(p)
		}
		return p.Ppid == 1 && o.CycleClosed(id.Cycle) && proctree.Recorded(tree)(p) && !proctree.SharedHelper(p)
	}
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
