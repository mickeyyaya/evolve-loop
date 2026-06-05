// Package phaseinventory builds .evolve/phase-inventory.json — the structured
// index of every phase the pipeline can run: built-in registry entries plus
// user/plugin phases from all discovery roots (ADR-0038). It is the phase
// counterpart of skillinventory and follows the same cache semantics (mtime
// freshness vs TTL; --force bypass).
//
// The inventory is ADVISORY-ONLY: the orchestrator's composition root always
// re-reads the live merged catalog for routing and execution, so a stale
// inventory can weaken an advisor SELECT hint but can never break a cycle.
// Consumers: the advisor's catalog cards, `evolve phases list`, and any LLM
// reading the index offline to learn what phases exist before minting one.
package phaseinventory

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// DefaultTTL matches skillinventory's cadence.
const DefaultTTL = time.Hour

// OutputFile is the inventory's project-relative location.
const OutputFile = ".evolve/phase-inventory.json"

// Inventory is the on-disk JSON shape — the public contract for advisor and
// LLM consumers.
type Inventory struct {
	GeneratedAt   time.Time           `json:"generated_at"`
	ProjectRoot   string              `json:"project_root"`
	PhaseCount    int                 `json:"phase_count"`
	Phases        []PhaseEntry        `json:"phases"`
	CategoryIndex map[string][]string `json:"category_index"` // goal type → []phase name
}

// PhaseEntry is one phase's index line: spec metadata plus the
// contract-derived deliverable facts (what the runtime will actually verify).
type PhaseEntry struct {
	Name             string   `json:"name"`
	Source           string   `json:"source"`         // "builtin" | "user"
	Root             string   `json:"root,omitempty"` // discovery root for user phases (project-relative when inside)
	Archetype        string   `json:"archetype"`
	Kind             string   `json:"kind"`
	Optional         bool     `json:"optional,omitempty"`
	WritesSource     bool     `json:"writes_source,omitempty"`
	Tier             string   `json:"tier"`
	Agent            string   `json:"agent"`
	Description      string   `json:"description,omitempty"`
	WhenToUse        string   `json:"when_to_use,omitempty"`
	Categories       []string `json:"categories,omitempty"`
	Artifact         string   `json:"artifact"`
	RequiredSections []string `json:"required_sections,omitempty"`
	EmitsVerdict     bool     `json:"emits_verdict,omitempty"`
	After            string   `json:"after,omitempty"`
}

// Options configures Build. ProjectRoot is required. RegistryPath defaults to
// docs/architecture/phase-registry.json under ProjectRoot; Roots defaults to
// [ProjectRoot/.evolve/phases] (callers pass the EVOLVE_PHASE_ROOTS expansion).
type Options struct {
	ProjectRoot  string
	RegistryPath string
	Roots        []string
	TTL          time.Duration
	NowFn        func() time.Time
	Force        bool
}

// Result captures a Build outcome. Warnings carry fail-open scan findings
// (missing registry, malformed phase.json, shadowed duplicates).
type Result struct {
	OutputPath string
	CacheHit   bool
	PhaseCount int
	Warnings   []string
}

// Build produces the inventory at <project_root>/.evolve/phase-inventory.json.
func Build(opts Options) (Result, error) {
	if opts.ProjectRoot == "" {
		return Result{}, fmt.Errorf("phaseinventory: ProjectRoot required")
	}
	ttl := opts.TTL
	if ttl == 0 {
		ttl = DefaultTTL
	}
	nowFn := opts.NowFn
	if nowFn == nil {
		nowFn = time.Now
	}
	outPath := filepath.Join(opts.ProjectRoot, filepath.FromSlash(OutputFile))

	if !opts.Force {
		if info, err := os.Stat(outPath); err == nil && nowFn().Sub(info.ModTime()) < ttl {
			return Result{OutputPath: outPath, CacheHit: true}, nil
		}
	}

	inv, warnings := scan(opts, nowFn())
	if err := atomicwrite.JSON(outPath, inv); err != nil {
		return Result{}, fmt.Errorf("phaseinventory: write: %w", err)
	}
	return Result{OutputPath: outPath, PhaseCount: inv.PhaseCount, Warnings: warnings}, nil
}

// scan assembles the Inventory from the built-in registry + all user roots.
// Everything is fail-open: a missing registry or broken overlay degrades to a
// warning, never an error — an index must describe what IS loadable.
func scan(opts Options, now time.Time) (Inventory, []string) {
	var warnings []string

	registryPath := opts.RegistryPath
	if registryPath == "" {
		registryPath = filepath.Join(opts.ProjectRoot, "docs", "architecture", "phase-registry.json")
	}
	builtins, err := phasespec.Load(registryPath)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("registry unreadable (%v) — inventorying user roots only", err))
		builtins = phasespec.Catalog{}
	}

	roots := opts.Roots
	if len(roots) == 0 {
		roots = []string{filepath.Join(opts.ProjectRoot, ".evolve", "phases")}
	}
	userSpecs, sources, discWarns := phasespec.DiscoverUserSpecsFromRoots(roots)
	warnings = append(warnings, discWarns...)

	merged, mergeWarns := builtins.Merge(userSpecs)
	warnings = append(warnings, mergeWarns...)

	inv := Inventory{
		GeneratedAt:   now,
		ProjectRoot:   opts.ProjectRoot,
		Phases:        make([]PhaseEntry, 0, len(merged.All())),
		CategoryIndex: map[string][]string{},
	}
	for _, spec := range merged.All() {
		entry := entryFromSpec(spec, merged.IsUser(spec.Name), sources[spec.Name], opts.ProjectRoot)
		cats := entry.Categories
		if len(cats) == 0 {
			cats = []string{"uncategorized"}
		}
		for _, c := range cats {
			inv.CategoryIndex[c] = append(inv.CategoryIndex[c], entry.Name)
		}
		inv.Phases = append(inv.Phases, entry)
	}
	inv.PhaseCount = len(inv.Phases)

	// Stable ordering for deterministic diffs (the catalog keeps pipeline
	// order; the index is a lookup surface, so name order reads better).
	sort.Slice(inv.Phases, func(i, j int) bool { return inv.Phases[i].Name < inv.Phases[j].Name })
	for _, list := range inv.CategoryIndex {
		sort.Strings(list)
	}
	return inv, warnings
}

// entryFromSpec projects one PhaseSpec (+ its contract derivation) into an
// index entry. root is the discovery root for user phases ("" for built-ins),
// rewritten project-relative when it sits inside the project.
func entryFromSpec(spec phasespec.PhaseSpec, isUser bool, root, projectRoot string) PhaseEntry {
	source := "builtin"
	if isUser {
		source = "user"
	} else {
		root = ""
	}
	if root != "" {
		if rel, err := filepath.Rel(projectRoot, root); err == nil && !strings.HasPrefix(rel, "..") {
			root = filepath.ToSlash(rel)
		}
	}

	contract := phasecontract.FromSpec(spec)
	sections := make([]string, 0, len(contract.Sections))
	for _, s := range contract.Sections {
		sections = append(sections, s.Canonical)
	}

	return PhaseEntry{
		Name:             spec.Name,
		Source:           source,
		Root:             root,
		Archetype:        string(spec.RoleOrDefault()),
		Kind:             spec.KindOrDefault(),
		Optional:         spec.Optional,
		WritesSource:     spec.WritesSource,
		Tier:             spec.ModelOrDefault(),
		Agent:            spec.AgentName(),
		Description:      spec.Description,
		WhenToUse:        spec.WhenToUse,
		Categories:       spec.Categories,
		Artifact:         contract.ArtifactName,
		RequiredSections: sections,
		EmitsVerdict:     len(contract.Verdicts) > 0,
		After:            spec.After,
	}
}
