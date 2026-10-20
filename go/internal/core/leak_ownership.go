package core

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/acssuite"
	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/mintregistry"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/scopedelta"
)

const evalsDir = ".evolve/evals/"

func EvalFilePath(root, slug string) string {
	return filepath.Join(root, filepath.FromSlash(evalsDir), slug+".md")
}

type ownerKind int

const (
	itemOwner ownerKind = iota + 1
	cycleOwner
	mintRegistryOwner
	dossierOwner
)

type ownerKey struct {
	kind  ownerKind
	item  string
	cycle int
}

func (k ownerKey) String() string {
	switch k.kind {
	case itemOwner:
		return "item " + k.item
	case cycleOwner:
		return "cycle " + strconv.Itoa(k.cycle)
	case dossierOwner:
		return "the dossier of cycle " + strconv.Itoa(k.cycle)
	}
	return "the mint registry"
}

type mainTreeOwnership struct {
	cycle     int
	items     map[string]bool
	mints     map[string]bool
	protected func(string) bool
}

type ownerRule func(o mainTreeOwnership, p string) (ownerKey, bool)

var ownerRules = []ownerRule{evalOwner, predicatePackageOwner, changeRecordOwner, mintOwner, dossierFileOwner}

func laneOwnership(cs CycleState, mints map[string]bool) mainTreeOwnership {
	return mainTreeOwnership{cycle: cs.CycleID, items: laneItems(cs.WorkspacePath), mints: mints}
}

func laneItems(workspace string) map[string]bool {
	committed, decided := committedset.Committed(workspace)
	materialized := materializedEvalSlugs(workspace, os.Stderr)
	if !decided && len(materialized) == 0 {
		return nil
	}
	items := make(map[string]bool, len(committed)+len(materialized))
	for _, id := range committed {
		items[id] = true
	}
	for _, slug := range materialized {
		items[slug] = true
	}
	return items
}

func materializedEvalSlugs(workspace string, warn io.Writer) []string {
	if workspace == "" {
		return nil
	}
	entries, err := os.ReadDir(filepath.Join(workspace, filepath.FromSlash(evalsDir)))
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(warn, "[orchestrator] WARN lane ownership: workspace eval home unreadable (%v) — owned slugs fall back to the committed set\n", err)
		}
		return nil
	}
	var slugs []string
	for _, e := range entries {
		if slug, ok := strings.CutSuffix(e.Name(), ".md"); ok && !e.IsDir() {
			slugs = append(slugs, slug)
		}
	}
	return slugs
}

func (o mainTreeOwnership) ownerOf(p string) (ownerKey, bool) {
	for _, rule := range ownerRules {
		if key, keyed := rule(o, p); keyed {
			return key, true
		}
	}
	return ownerKey{}, false
}

func (o mainTreeOwnership) holds(key ownerKey) bool {
	switch key.kind {
	case itemOwner:
		return o.items == nil || o.items[key.item]
	case cycleOwner, dossierOwner:
		return o.cycle <= 0 || o.cycle == key.cycle
	}
	return false
}

func (o mainTreeOwnership) foreignOwner(p string) (string, bool) {
	key, keyed := o.ownerOf(p)
	if !keyed || o.holds(key) {
		return "", false
	}
	return key.String(), true
}

func (o mainTreeOwnership) ownsEval(p string) bool {
	slug, ok := evalSlug(p)
	return ok && o.holds(ownerKey{kind: itemOwner, item: slug})
}

func evalSlug(p string) (string, bool) {
	name, ok := strings.CutPrefix(p, evalsDir)
	if !ok || strings.Contains(name, "/") {
		return "", false
	}
	return strings.CutSuffix(name, ".md")
}

func evalOwner(_ mainTreeOwnership, p string) (ownerKey, bool) {
	slug, ok := evalSlug(p)
	return ownerKey{kind: itemOwner, item: slug}, ok
}

func predicatePackageOwner(o mainTreeOwnership, p string) (ownerKey, bool) {
	packages := acssuite.AncestorCyclePackages([]string{p}, o.cycle)
	if len(packages) == 0 {
		return ownerKey{}, false
	}
	n, err := strconv.Atoi(strings.TrimPrefix(packages[0], "go/acs/cycle"))
	return ownerKey{kind: cycleOwner, cycle: n}, err == nil
}

func changeRecordOwner(_ mainTreeOwnership, p string) (ownerKey, bool) {
	if !explanationdocs.IsCycleChangeRecord(p) {
		return ownerKey{}, false
	}
	number, _, _ := strings.Cut(strings.TrimPrefix(path.Base(p), "cycle-"), "-")
	n, err := strconv.Atoi(number)
	return ownerKey{kind: cycleOwner, cycle: n}, err == nil
}

func mintOwner(o mainTreeOwnership, p string) (ownerKey, bool) {
	return ownerKey{kind: mintRegistryOwner}, isActiveMintPhasePath(o.mints, p)
}

var dossierFile = regexp.MustCompile(`^knowledge-base/cycles/cycle-(\d+)\.(json|md)$`)

func dossierFileOwner(_ mainTreeOwnership, p string) (ownerKey, bool) {
	m := dossierFile.FindStringSubmatch(p)
	if m == nil {
		return ownerKey{}, false
	}
	n, err := strconv.Atoi(m[1])
	return ownerKey{kind: dossierOwner, cycle: n}, err == nil
}

type liveSiblings struct {
	projectRoot string
	runs        map[string]bool
	items       map[string]bool
	declared    []string
}

func liveSiblingsOf(projectRoot, ownWorkspace string, now time.Time) liveSiblings {
	siblings := liveSiblings{projectRoot: projectRoot, runs: map[string]bool{}, items: map[string]bool{}}
	for _, run := range runlease.LiveRuns(runWorkspacesRoot(projectRoot), now) {
		if sameDirectory(run.Dir, ownWorkspace) {
			continue
		}
		siblings.runs[filepath.Clean(run.Dir)] = true
		for id, held := range laneItems(run.Dir) {
			siblings.items[id] = held
		}
		if cycle, ok := inboxbatch.ParseProcessingCycle(filepath.Base(run.Dir)); ok {
			files, err := LaneDeclaredFiles(paths.EvolveDirOf(projectRoot), cycle)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[orchestrator] WARN lane ownership: sibling cycle %d has an unreadable scope (%v), so it holds no undeclared path\n", cycle, err)
			}
			siblings.declared = append(siblings.declared, files...)
		}
	}
	return siblings
}

func (s liveSiblings) declaresAgainst(p string, own []string) bool {
	scope := scopedelta.Scope{Declared: own, LaneOthers: s.declared}
	return scopedelta.Classify(p, scopedelta.Declaration{}, scope, nil).Class == scopedelta.ClassCrossLane
}

func runWorkspacesRoot(projectRoot string) string {
	return filepath.Dir(RunWorkspacePath(projectRoot, 1))
}

func (s liveSiblings) holds(key ownerKey) bool {
	switch key.kind {
	case itemOwner:
		return s.items[key.item]
	case cycleOwner:
		return s.runs[filepath.Clean(RunWorkspacePath(s.projectRoot, key.cycle))]
	case dossierOwner:
		return s.runs[filepath.Clean(RunWorkspacePath(s.projectRoot, key.cycle))] || s.cycleExists(key.cycle)
	}
	return key.kind == mintRegistryOwner
}

func (s liveSiblings) cycleExists(cycle int) bool {
	info, err := os.Stat(RunWorkspacePath(s.projectRoot, cycle))
	return err == nil && info.IsDir()
}

func (o mainTreeOwnership) heldBySibling(p string, siblings liveSiblings) bool {
	key, keyed := o.ownerOf(p)
	return keyed && !o.holds(key) && siblings.holds(key)
}

type leakExemptions struct {
	holder func(string) bool
	leased map[string]bool
}

func (e leakExemptions) heldElsewhere(p string) bool {
	return e.holder != nil && e.holder(p)
}

func (o mainTreeOwnership) exemptions(projectRoot, ownWorkspace string, leased map[string]bool) leakExemptions {
	return leakExemptions{holder: liveHolderCheck(projectRoot, ownWorkspace, o), leased: leased}
}

func (cr *cycleRun) leakExemptions() leakExemptions {
	ownership := laneOwnership(cr.cs, activeVerifiedMints(cr.req.ProjectRoot, cr.o.errOut()))
	ownership.protected = cr.o.protectedSurface
	return ownership.exemptions(cr.req.ProjectRoot, cr.cs.WorkspacePath, cr.consoleLeased)
}

func liveHolderCheck(projectRoot, ownWorkspace string, ownership mainTreeOwnership) func(string) bool {
	var siblings *liveSiblings
	var own *[]string
	load := func() liveSiblings {
		if siblings == nil {
			loaded := liveSiblingsOf(projectRoot, ownWorkspace, time.Now())
			siblings = &loaded
		}
		return *siblings
	}
	ownScope := func() []string {
		if own == nil {
			files, err := LaneDeclaredFiles(paths.EvolveDirOf(projectRoot), ownership.cycle)
			if err != nil {
				files = nil
			}
			own = &files
		}
		return *own
	}
	return func(p string) bool {
		if _, keyed := ownership.ownerOf(p); keyed {
			_, foreign := ownership.foreignOwner(p)
			return foreign && ownership.heldBySibling(p, load())
		}
		if ownership.protected == nil || ownership.protected(p) {
			return false
		}
		scope := ownScope()
		return len(scope) > 0 && load().declaresAgainst(p, scope)
	}
}

func activeVerifiedMints(projectRoot string, warn io.Writer) map[string]bool {
	registry := mintregistry.Path(projectRoot)
	mints, err := mintregistry.ActiveNames(registry, time.Now())
	if err != nil {
		fmt.Fprintf(warn, "[orchestrator] ABNORMAL mint registry unreadable (%v); mint exemption disabled for this check\n", err)
		if _, qErr := mintregistry.QuarantineCorrupt(registry); qErr != nil {
			fmt.Fprintf(warn, "[orchestrator] WARN mint registry quarantine failed: %v\n", qErr)
		}
		return nil
	}
	return verifiedActiveMints(projectRoot, mints)
}
