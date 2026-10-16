// Package phaseinventory builds the index of every phase the pipeline can run.
// It writes the built-in registry entries and the user and plugin phases of
// all discovery roots to .evolve/phase-inventory.json.
package phaseinventory

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

const DefaultTTL = time.Hour

const OutputFile = ".evolve/phase-inventory.json"

type Inventory struct {
	GeneratedAt   time.Time           `json:"generated_at"`
	ProjectRoot   string              `json:"project_root"`
	PhaseCount    int                 `json:"phase_count"`
	Phases        []PhaseEntry        `json:"phases"`
	CategoryIndex map[string][]string `json:"category_index"`
}

type PhaseEntry struct {
	Name             string   `json:"name"`
	Source           string   `json:"source"`
	Root             string   `json:"root,omitempty"`
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

type Options struct {
	ProjectRoot  string
	RegistryPath string
	Roots        []string
	TTL          time.Duration
	NowFn        func() time.Time
	Force        bool
}

type Result struct {
	OutputPath string
	CacheHit   bool
	PhaseCount int
	Warnings   []string
}

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

func scan(opts Options, now time.Time) (Inventory, []string) {
	var warnings []string

	registryPath := opts.RegistryPath
	if registryPath == "" {
		registryPath = config.RegistryPath(opts.ProjectRoot)
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

	sortByName(&inv)
	return inv, warnings
}

func sortByName(inv *Inventory) {
	sort.Slice(inv.Phases, func(i, j int) bool { return inv.Phases[i].Name < inv.Phases[j].Name })
	for _, list := range inv.CategoryIndex {
		sort.Strings(list)
	}
}

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

	var contract phasecontract.Contract
	var sections []string
	if phasecontract.SynthesizesContract(spec) {
		contract = phasecontract.FromSpec(spec)
		sections = make([]string, 0, len(contract.Sections))
		for _, s := range contract.Sections {
			sections = append(sections, s.Canonical)
		}
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
