package phasespec

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

// userSpecFile is the per-phase definition filename an operator drops under
// <phasesDir>/<name>/phase.json.
const userSpecFile = "phase.json"

// DiscoverUserSpecs reads operator-authored phase definitions from
// <phasesDir>/<name>/phase.json. Missing dir → no specs (fail-open: user
// phases are opt-in). An unreadable or malformed phase.json is skipped with a
// warning rather than failing the whole load — one broken brick must not break
// the catalog. Specs are returned sorted by directory name for determinism.
func DiscoverUserSpecs(phasesDir string) (specs []PhaseSpec, warnings []string) {
	entries, err := os.ReadDir(phasesDir)
	if err != nil {
		return nil, nil // no .evolve/phases/ → nothing to discover
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
			continue // dir without a phase.json is not a phase definition
		}
		var s PhaseSpec
		if err := json.Unmarshal(raw, &s); err != nil {
			warnings = append(warnings, "skipped "+path+": malformed JSON ("+err.Error()+")")
			continue
		}
		if s.Name == "" {
			// Default the name to the directory — but the dir name is untrusted
			// filesystem input, so guard the kebab-case floor here rather than
			// admitting a malformed name into the catalog.
			if !nameRE.MatchString(dir) {
				warnings = append(warnings, "skipped "+path+": directory name "+dir+" is not valid kebab-case and phase.json has no name")
				continue
			}
			s.Name = dir
		}
		specs = append(specs, s)
	}
	return specs, warnings
}

// Merge returns a new Catalog with the user specs layered over the built-in
// catalog. A user spec whose name clashes with a built-in is DROPPED with a
// warning (built-ins win — an operator cannot silently redefine a spine phase).
// The receiver is not mutated. User specs are appended in input-slice order, so
// callers wanting deterministic listing should pass sorted specs (as
// DiscoverUserSpecs does).
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

// IsUser reports whether name was contributed by an operator overlay (vs a
// built-in registry entry).
func (c Catalog) IsUser(name string) bool { return c.userNames[name] }
