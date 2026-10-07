package core

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/acssuite"
	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/mintregistry"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
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
	}
	return "the mint registry"
}

type mainTreeOwnership struct {
	cycle int
	items map[string]bool
	mints map[string]bool
}

type ownerRule func(o mainTreeOwnership, p string) (ownerKey, bool)

var ownerRules = []ownerRule{evalOwner, predicatePackageOwner, changeRecordOwner, mintOwner}

func laneOwnership(cs CycleState, mints map[string]bool) mainTreeOwnership {
	return mainTreeOwnership{cycle: cs.CycleID, items: laneItems(cs.WorkspacePath), mints: mints}
}

func laneItems(workspace string) map[string]bool {
	committed, decided := committedset.Committed(workspace)
	materialized := materializedEvalSlugs(workspace)
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

func materializedEvalSlugs(workspace string) []string {
	if workspace == "" {
		return nil
	}
	entries, err := os.ReadDir(filepath.Join(workspace, filepath.FromSlash(evalsDir)))
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN lane ownership: workspace eval home unreadable (%v) — owned slugs fall back to the committed set\n", err)
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
	case cycleOwner:
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

type liveSiblings struct {
	projectRoot string
	runs        map[string]bool
	items       map[string]bool
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
	}
	return siblings
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
	}
	return key.kind == mintRegistryOwner
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
	ownership := laneOwnership(cr.cs, activeVerifiedMints(cr.req.ProjectRoot))
	return ownership.exemptions(cr.req.ProjectRoot, cr.cs.WorkspacePath, cr.consoleLeased)
}

func liveHolderCheck(projectRoot, ownWorkspace string, ownership mainTreeOwnership) func(string) bool {
	var siblings *liveSiblings
	return func(p string) bool {
		if _, foreign := ownership.foreignOwner(p); !foreign {
			return false
		}
		if siblings == nil {
			loaded := liveSiblingsOf(projectRoot, ownWorkspace, time.Now())
			siblings = &loaded
		}
		return ownership.heldBySibling(p, *siblings)
	}
}

func activeVerifiedMints(projectRoot string) map[string]bool {
	registry := mintregistry.Path(projectRoot)
	mints, err := mintregistry.ActiveNames(registry, time.Now())
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] ABNORMAL mint registry unreadable (%v); mint exemption disabled for this check\n", err)
		if _, qErr := mintregistry.QuarantineCorrupt(registry); qErr != nil {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN mint registry quarantine failed: %v\n", qErr)
		}
		return nil
	}
	return verifiedActiveMints(projectRoot, mints)
}
