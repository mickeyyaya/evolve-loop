package fleet

import (
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

func PlanCycles(todos []Todo, count int) (specs []CycleSpec, deferred []Todo) {
	buckets, deferred := Partition(todos, count)
	for _, b := range buckets {
		if len(b) == 0 {
			continue
		}
		ids := make([]string, len(b))
		for i, td := range b {
			ids[i] = td.ID
		}
		specs = append(specs, CycleSpec{
			Scope:          ids,
			OutputContract: combinedContract(b),
			Env:            map[string]string{ipcenv.FleetScopeKey: strings.Join(ids, ",")},
		})
	}
	return specs, deferred
}

type Todo struct {
	ID             string   `json:"id"`
	Files          []string `json:"files"`
	DependsOn      []string `json:"depends_on,omitempty"`
	Priority       int      `json:"priority,omitempty"`
	Optional       bool     `json:"optional,omitempty"`
	OutputContract string   `json:"output_contract,omitempty"`
	ToolScope      []string `json:"tool_scope,omitempty"`
}

func Partition(todos []Todo, n int) (buckets [][]Todo, deferred []Todo) {
	if n < 1 {
		n = 1
	}
	buckets = make([][]Todo, n)
	owner := map[string]int{}
	for _, td := range todos {
		files := normalizeFiles(td.Files)
		owning := owningBuckets(owner, files)
		switch len(owning) {
		case 0:
			place(buckets, owner, leastLoaded(buckets), td, files)
		case 1:
			place(buckets, owner, only(owning), td, files)
		default:
			deferred = append(deferred, td)
		}
	}
	return buckets, deferred
}

func owningBuckets(owner map[string]int, files map[string]bool) map[int]bool {
	out := map[int]bool{}
	for f := range files {
		if b, ok := owner[f]; ok {
			out[b] = true
		}
	}
	return out
}

func place(buckets [][]Todo, owner map[string]int, b int, td Todo, files map[string]bool) {
	buckets[b] = append(buckets[b], td)
	for f := range files {
		owner[f] = b
	}
}

func leastLoaded(buckets [][]Todo) int {
	best, bestN := 0, -1
	for i, b := range buckets {
		if bestN == -1 || len(b) < bestN {
			best, bestN = i, len(b)
		}
	}
	return best
}

func only(set map[int]bool) int {
	for k := range set {
		return k
	}
	panic("fleet.only: called on empty set")
}

func normalizeFiles(files []string) map[string]bool {
	out := make(map[string]bool, len(files))
	for _, f := range files {
		out[filepath.Clean(f)] = true
	}
	return out
}
