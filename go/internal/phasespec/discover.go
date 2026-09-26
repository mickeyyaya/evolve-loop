package phasespec

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

const userSpecFile = "phase.json"

// DiscoverUserSpecs reads <phasesDir>/<name>/phase.json specs sorted by directory; a missing dir or bad file is skipped, never fatal.
func DiscoverUserSpecs(phasesDir string) (specs []PhaseSpec, warnings []string) {
	entries, err := os.ReadDir(phasesDir)
	if err != nil {
		return nil, nil
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, dir := range names {
		path := filepath.Join(phasesDir, dir, userSpecFile)
		raw, err := os.ReadFile(path)
		if err != nil {
			continue // a dir without phase.json is not a phase
		}
		var s PhaseSpec
		if err := json.Unmarshal(raw, &s); err != nil {
			warnings = append(warnings, "skipped "+path+": malformed JSON ("+err.Error()+")")
			continue
		}
		if s.Name == "" {
			// The directory name is untrusted input, so it must pass the kebab-case floor before naming a phase.
			if !nameRE.MatchString(dir) {
				warnings = append(warnings, "skipped "+path+": directory name "+dir+" is not valid kebab-case and phase.json has no name")
				continue
			}
			s.Name = dir
		}
		ApplyArchetypeDefaults(&s)
		// Transition-activating fields are built-in only, so an overlay can never inject a branch
		// into the kernel; this is the one ingestion point where provenance is known.
		// See ADR-0058.
		if s.OnPass != "" || s.OnFail != "" || s.BranchingStrategy != "" {
			warnings = append(warnings, "user phase "+s.Name+" in "+path+" declares transition-activating fields (on_pass/on_fail/branching_strategy) — stripped; these are restricted to built-in phases (ADR-0058)")
			s.OnPass, s.OnFail, s.BranchingStrategy = "", "", ""
		}
		specs = append(specs, s)
	}
	return specs, warnings
}

// DiscoverUserSpecsFromRoots discovers specs across roots; the left-most root wins a name clash, and sources maps name to root.
func DiscoverUserSpecsFromRoots(roots []string) (specs []PhaseSpec, sources map[string]string, warnings []string) {
	sources = make(map[string]string)
	for _, root := range roots {
		rootSpecs, rootWarns := DiscoverUserSpecs(root)
		warnings = append(warnings, rootWarns...)
		for _, s := range rootSpecs {
			if prev, dup := sources[s.Name]; dup {
				warnings = append(warnings, "phase "+s.Name+" in "+root+" ignored — already loaded from "+prev+" (left-most root wins)")
				continue
			}
			specs = append(specs, s)
			sources[s.Name] = root
		}
	}
	return specs, sources, warnings
}

// Merge returns a new Catalog with user specs appended over the built-ins, which win every clash except an optional built-in's overlay.
func (c Catalog) Merge(user []PhaseSpec) (Catalog, []string) {
	merged := Catalog{
		order:     append([]string(nil), c.order...),
		byName:    make(map[string]PhaseSpec, len(c.byName)+len(user)),
		userNames: make(map[string]bool, len(user)),
	}
	for k, v := range c.byName {
		merged.byName[k] = v
	}
	var warnings []string
	for _, s := range user {
		if s.Name == "" {
			warnings = append(warnings, "skipped a user phase with an empty name")
			continue
		}
		if _, isBuiltin := c.byName[s.Name]; isBuiltin {
			// An optional built-in's overlay (memo) is adopted in place. The built-in's own Optional
			// flag, not the overlay's, keeps an operator from hijacking a mandatory spine phase.
			if isOptionalBuiltinName(s.Name, c) {
				merged.byName[s.Name] = s
				merged.userNames[s.Name] = true
				continue
			}
			warnings = append(warnings, "user phase "+s.Name+" clashes with a built-in — built-in kept, user definition ignored")
			continue
		}
		if _, dup := merged.byName[s.Name]; dup {
			warnings = append(warnings, "duplicate user phase "+s.Name+" — first kept")
			continue
		}
		merged.order = append(merged.order, s.Name)
		merged.byName[s.Name] = s
		merged.userNames[s.Name] = true
	}
	return merged, warnings
}

// IsUser reports whether name was contributed by an operator overlay.
func (c Catalog) IsUser(name string) bool { return c.userNames[name] }
