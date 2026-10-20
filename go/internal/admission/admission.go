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

type leafGraph interface {
	IsLeaf(pkg string) bool
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

type ShadowAdmission struct {
	ID     string
	Rank   int
	Cells  []string
	HeldBy string
	Reason string
}

type Admitter struct {
	Graph        Graph
	Text         func(id string) string
	Optimistic   bool
	OnShadow     func(ShadowAdmission)
	OnOptimistic func(ShadowAdmission)
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
		if blocker, busy := a.firstConflict(holders, fp); busy {
			d.BlockedBy = blocker.lane
			a.recordShadow(holders, fp, d, blocker)
			waiting = append(waiting, d)
			continue
		}
		a.recordOptimistic(holders, fp, d)
		holders = append(holders, holder{lane: td.ID, fp: fp})
		admitted = append(admitted, d)
	}
	return admitted, waiting
}

func (a Admitter) firstConflict(holders []holder, fp footprint) (holder, bool) {
	rule := conflict
	if a.Optimistic {
		rule = sharedEdit
	}
	for _, h := range holders {
		if rule(h.fp, fp) {
			return h, true
		}
	}
	return holder{}, false
}

func (a Admitter) recordShadow(holders []holder, fp footprint, d Decision, blocker holder) {
	if a.OnShadow == nil || a.Optimistic {
		return
	}
	for _, h := range holders {
		if sharedEdit(h.fp, fp) {
			return
		}
	}
	a.OnShadow(ShadowAdmission{ID: d.ID, Rank: d.Rank, Cells: d.Cells, HeldBy: d.BlockedBy, Reason: "reach meets " + reachMet(blocker.fp, fp)})
}

func (a Admitter) recordOptimistic(holders []holder, fp footprint, d Decision) {
	if a.OnOptimistic == nil || !a.Optimistic {
		return
	}
	for _, h := range holders {
		if conflict(h.fp, fp) {
			a.OnOptimistic(ShadowAdmission{ID: d.ID, Rank: d.Rank, Cells: d.Cells, HeldBy: h.lane, Reason: "reach meets " + reachMet(h.fp, fp)})
			return
		}
	}
}

func reachMet(x, y footprint) string {
	var met []string
	for _, pair := range [][2]map[string]bool{{x.edits, y.reach}, {y.edits, x.reach}} {
		for p := range pair[0] {
			if pair[1][p] {
				met = append(met, strings.TrimPrefix(p, ModulePath+"/"))
			}
		}
	}
	return slices.Min(met)
}

func conflict(x, y footprint) bool {
	return sharedEdit(x, y) || meets(x.edits, y.reach) || meets(y.edits, x.reach)
}

func sharedEdit(x, y footprint) bool {
	return (x.hub && y.hub) || meets(x.edits, y.edits)
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
	}
	if a.allLeaves(pkgs) {
		return fp
	}
	for _, p := range pkgs {
		for _, d := range a.Graph.Deps(p) {
			if !hubs[d] {
				fp.reach[d] = true
			}
		}
	}
	return fp
}

func (a Admitter) allLeaves(pkgs []string) bool {
	lg, ok := a.Graph.(leafGraph)
	if !ok || len(pkgs) == 0 {
		return false
	}
	return !slices.ContainsFunc(pkgs, func(p string) bool { return !lg.IsLeaf(p) })
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
