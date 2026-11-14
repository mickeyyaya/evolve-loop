package changedpkgs

import "strings"

const (
	acsPatternPrefix = "./acs/"
	enforceListPath  = "go/.apicover-enforce"
)

type Scope struct {
	Full     bool
	Packages []string
	Reason   string
}

func GateScope(repoRoot, baseRef string) Scope {
	files, ok := ChangedFilesChecked(repoRoot, baseRef)
	if !ok {
		return fullScope("the change against " + baseRef + " cannot be read")
	}
	var changed []string
	for _, f := range files {
		pattern, ok := PatternOf(f.Path)
		if !ok || f.Path == enforceListPath {
			continue
		}
		if pattern == moduleRootPattern {
			return fullScope("a module-root file changed: " + f.Path)
		}
		changed = append(changed, pattern)
	}
	closure, ok := ImporterClosureChecked(repoRoot, changed)
	if !ok {
		return fullScope("the import graph of the module cannot be read")
	}
	return Scope{Packages: withoutAcs(closure.Patterns)}
}

func (s Scope) Enforced(entries []string) []string {
	if s.Full {
		return entries
	}
	var kept []string
	for _, entry := range entries {
		if s.covers(entry) {
			kept = append(kept, entry)
		}
	}
	return kept
}

func (s Scope) covers(entry string) bool {
	for _, pattern := range s.Packages {
		dir := strings.TrimSuffix(pattern, "/...")
		if entry == dir || strings.HasPrefix(entry, dir+"/") {
			return true
		}
	}
	return false
}

func fullScope(reason string) Scope {
	return Scope{Full: true, Reason: reason}
}

func withoutAcs(patterns []string) []string {
	var kept []string
	for _, p := range patterns {
		if !strings.HasPrefix(p, acsPatternPrefix) {
			kept = append(kept, p)
		}
	}
	return kept
}
