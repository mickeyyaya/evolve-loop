// Package admission decides which todos can run at once by their package cells, with one hub cell of capacity one.
// See docs/architecture/packages/internal-admission.md.
package admission

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
)

const HubCell = "hub"

type Graph interface {
	Packages(files []string) []string
	Deps(pkg string) []string
	Hubs() map[string]bool
}

type Holding struct {
	Lane string
	Todo fleet.Todo
}

type Decision struct {
	ID        string
	Rank      int
	Cells     []string
	BlockedBy string
}

type Admitter struct {
	Graph Graph
	Text  func(id string) string
}

type footprint struct {
	edits map[string]bool
	reach map[string]bool
	hub   bool
}

type holder struct {
	lane string
	fp   footprint
}

var namedPackagePath = regexp.MustCompile(`go/internal/[A-Za-z0-9_./-]+`)

func (a Admitter) Admit(inFlight []Holding, candidates []fleet.Todo, scanK int) (admitted, waiting []Decision) {
	holders := make([]holder, 0, len(inFlight)+len(candidates))
	for _, h := range inFlight {
		holders = append(holders, holder{lane: h.Lane, fp: a.footprintOf(h.Todo)})
	}
	for rank, td := range candidates {
		if len(admitted) == scanK {
			break
		}
		fp := a.footprintOf(td)
		d := Decision{ID: td.ID, Rank: rank, Cells: fp.cells()}
		if blocker, busy := firstConflict(holders, fp); busy {
			d.BlockedBy = blocker
			waiting = append(waiting, d)
			continue
		}
		holders = append(holders, holder{lane: td.ID, fp: fp})
		admitted = append(admitted, d)
	}
	return admitted, waiting
}

func firstConflict(holders []holder, fp footprint) (string, bool) {
	for _, h := range holders {
		if conflict(h.fp, fp) {
			return h.lane, true
		}
	}
	return "", false
}

func conflict(x, y footprint) bool {
	return (x.hub && y.hub) || meets(x.edits, y.edits) || meets(x.edits, y.reach) || meets(y.edits, x.reach)
}

func meets(x, y map[string]bool) bool {
	for k := range x {
		if y[k] {
			return true
		}
	}
	return false
}

func (a Admitter) footprintOf(td fleet.Todo) footprint {
	pkgs, hub := a.edited(td)
	hubs := a.Graph.Hubs()
	fp := footprint{edits: map[string]bool{}, reach: map[string]bool{}, hub: hub}
	for _, p := range pkgs {
		fp.edits[p] = true
		fp.hub = fp.hub || hubs[p]
		for _, d := range a.Graph.Deps(p) {
			if !hubs[d] {
				fp.reach[d] = true
			}
		}
	}
	return fp
}

func (a Admitter) edited(td fleet.Todo) (pkgs []string, hub bool) {
	files := td.Files
	if len(files) == 1 && files[0] == td.ID {
		files = nil
	}
	if len(files) > 0 {
		return a.Graph.Packages(files), slices.ContainsFunc(files, globalZone)
	}
	pkgs = a.Graph.Packages(namedPaths(a.text(td.ID)))
	return pkgs, len(pkgs) == 0
}

func namedPaths(text string) []string {
	found := namedPackagePath.FindAllString(text, -1)
	for i, p := range found {
		found[i] = strings.TrimRight(p, "./")
	}
	return found
}

func (a Admitter) text(id string) string {
	if a.Text == nil {
		return ""
	}
	return a.Text(id)
}

const moduleMakefile = "go/Makefile"

func globalZone(file string) bool {
	return fleet.IsGlobalZone(file) || fleet.IsGlobalZone(strings.TrimPrefix(file, "go/")) || filepath.Clean(file) == moduleMakefile
}

func (fp footprint) cells() []string {
	out := make([]string, 0, len(fp.edits)+1)
	for p := range fp.edits {
		out = append(out, strings.TrimPrefix(p, ModulePath+"/"))
	}
	slices.Sort(out)
	if fp.hub {
		out = append([]string{HubCell}, out...)
	}
	return out
}
