// Package scopedelta adjudicates the work a phase agent produced outside its declared scope, by what the change means.
// See docs/architecture/packages/internal-scopedelta.md.
package scopedelta

import (
	"fmt"
	"path"
	"regexp"
	"strings"
)

type Class string

const (
	ClassClosure       Class = "closure"
	ClassDiscovered    Class = "discovered"
	ClassOpportunistic Class = "opportunistic"
	ClassMisunderstood Class = "misunderstood"
	ClassBoundary      Class = "boundary"
	ClassCrossLane     Class = "cross-lane"
	ClassInScope       Class = "in-scope"
)

type Disposition string

const (
	DispositionKeep   Disposition = "keep"
	DispositionCarve  Disposition = "carve"
	DispositionRefuse Disposition = "refuse"
)

type Scope struct {
	Cycle      int
	Declared   []string
	Protected  []string
	LaneOthers []string
}

type Declaration struct {
	Class         Class
	Justification string
}

type Entry struct {
	Path          string        `json:"path"`
	Class         Class         `json:"class"`
	Justification string        `json:"justification,omitempty"`
	Disposition   Disposition   `json:"disposition"`
	Reason        string        `json:"reason"`
	PatchRef      string        `json:"patch_ref,omitempty"`
	Preserve      bool          `json:"preserve,omitempty"`
	Effect        Effect        `json:"effect,omitempty"`
	Corroboration Corroboration `json:"corroboration,omitempty"`
}

type ClosureRule interface {
	Name() string
	Covers(p string, in Scope) bool
}

func DefaultClosureRules() []ClosureRule {
	return []ClosureRule{
		samePackageTestRule{},
		cycleArtifactRule{},
		goBuildMetadataRule{},
	}
}

type samePackageTestRule struct{}

func (samePackageTestRule) Name() string { return "same-package-test" }

func (samePackageTestRule) Covers(p string, in Scope) bool {
	if !strings.HasSuffix(p, "_test.go") {
		return false
	}
	dir := path.Dir(p)
	for _, d := range in.Declared {
		if path.Dir(d) == dir {
			return true
		}
	}
	return false
}

type cycleArtifactRule struct{}

func (cycleArtifactRule) Name() string { return "cycle-artifact" }

func (cycleArtifactRule) Covers(p string, in Scope) bool {
	if in.Cycle <= 0 {
		return false
	}
	c := fmt.Sprintf("%d", in.Cycle)
	return strings.HasPrefix(p, "go/acs/cycle"+c+"/") ||
		strings.HasPrefix(p, ".evolve/runs/cycle-"+c+"/")
}

type goBuildMetadataRule struct{}

func (goBuildMetadataRule) Name() string { return "go-build-metadata" }

func (goBuildMetadataRule) Covers(p string, in Scope) bool {
	switch p {
	case "go/go.mod", "go/go.sum", "go/.apicover-enforce":
	default:
		return false
	}
	return in.touchesGo()
}

func (s Scope) touchesGo() bool {
	for _, d := range s.Declared {
		if strings.HasSuffix(d, ".go") {
			return true
		}
	}
	return false
}

func (s Scope) InScope(p string) bool { return isAtOrUnderAny(p, s.Declared) }

func (s Scope) isProtected(p string) bool { return isAtOrUnderAny(p, s.Protected) }

func (s Scope) belongsToSiblingLane(p string) bool { return isAtOrUnderAny(p, s.LaneOthers) }

func isAtOrUnderAny(p string, roots []string) bool {
	for _, root := range roots {
		if root == "" {
			continue
		}
		root = strings.TrimSuffix(root, "/")
		if p == root {
			return true
		}
		if strings.HasPrefix(p, root+"/") {
			return true
		}
	}
	return false
}

func Classify(p string, d Declaration, in Scope, rules []ClosureRule) Entry {
	switch {
	case in.InScope(p):
		return Entry{
			Path: p, Class: ClassInScope, Justification: d.Justification,
			Disposition: DispositionKeep,
			Reason:      "declared in this cycle's scope",
		}
	case in.isProtected(p):
		return Entry{
			Path: p, Class: ClassBoundary, Justification: d.Justification,
			Disposition: DispositionRefuse, Preserve: true,
			Reason: "protected operator-owned surface — not adjudicable by the phase that touched it (ADR-0074); the finding is preserved for console review",
		}
	case coveredByRule(p, in, rules) != "":
		return Entry{
			Path: p, Class: ClassClosure, Justification: d.Justification,
			Disposition: DispositionKeep,
			Reason:      "necessary closure of the in-scope change (" + coveredByRule(p, in, rules) + ") — omitting it yields an incorrect tree",
		}
	case in.belongsToSiblingLane(p):
		return Entry{
			Path: p, Class: ClassCrossLane, Justification: d.Justification,
			Disposition: DispositionCarve, Preserve: true,
			Reason: "belongs to a sibling lane's scope; handed over with the work attached rather than raced",
		}
	}

	class := d.Class
	if class == ClassClosure || class == "" {
		class = ClassOpportunistic
	}
	return Entry{
		Path: p, Class: class, Justification: d.Justification,
		Disposition: DispositionCarve, Preserve: true,
		Reason: "outside the declared scope and unconfirmed by any closure rule; preserved as queued work rather than shipped unreviewed",
	}
}

func coveredByRule(p string, in Scope, rules []ClosureRule) string {
	for _, r := range rules {
		if r.Covers(p, in) {
			return r.Name()
		}
	}
	return ""
}

func Unaccounted(changed []string, in Scope, rules []ClosureRule, adjudicated []Entry) []string {
	decided := make(map[string]bool, len(adjudicated))
	for _, e := range adjudicated {
		decided[e.Path] = true
	}
	var out []string
	for _, p := range changed {
		if in.InScope(p) || coveredByRule(p, in, rules) != "" || decided[p] {
			continue
		}
		out = append(out, p)
	}
	return out
}

var categoryVocabulary = map[string]bool{
	"": true, "a": true, "an": true, "the": true, "this": true, "that": true, "it": true, "its": true,
	"is": true, "was": true, "are": true, "be": true, "for": true, "of": true, "in": true, "to": true,
	"and": true, "or": true, "not": true, "no": true, "nothing": true, "more": true, "say": true,
	"out": true, "outside": true, "beyond": true, "within": true, "inside": true,
	"scope": true, "scopes": true, "scoped": true, "oos": true, "unrelated": true,
	"cycle": true, "cycles": true, "lane": true, "lanes": true, "item": true, "batch": true,
	"change": true, "changes": true, "different": true, "wrong": true, "violation": true,
	"n": true, "a/n": true, "na": true, "declared": true, "here": true, "s": true,
}

var reasonWordRE = regexp.MustCompile(`[A-Za-z0-9_]+`)

func saysMoreThanTheCategory(reason string) bool {
	for _, w := range reasonWordRE.FindAllString(strings.ToLower(reason), -1) {
		if !categoryVocabulary[w] {
			return true
		}
	}
	return false
}

func (e Entry) Validate() error {
	if e.Path == "" {
		return fmt.Errorf("scopedelta: entry has no path")
	}
	if strings.TrimSpace(e.Reason) == "" {
		return fmt.Errorf("scopedelta: %s: a %s decision must carry a reason", e.Path, e.Disposition)
	}
	if !saysMoreThanTheCategory(e.Reason) {
		return fmt.Errorf("scopedelta: %s: %q restates the category instead of naming a risk — say what admitting this change would cost, or what it is worth (unreviewed surface, protected boundary, cross-lane collision, a real adjacent defect)", e.Path, e.Reason)
	}
	if e.Class == ClassBoundary && e.Disposition != DispositionRefuse {
		return fmt.Errorf("scopedelta: %s: a boundary path may only be REFUSED (got %s) — a protected operator-owned surface is policy, not merit, and is not adjudicable by the phase that touched it", e.Path, e.Disposition)
	}
	if e.Disposition == DispositionCarve && strings.TrimSpace(e.PatchRef) == "" {
		return fmt.Errorf("scopedelta: %s: a carve must name the patch preserving the work, or it is a silent drop", e.Path)
	}
	return nil
}

type Summary struct {
	ByClass              map[Class]int `json:"by_class"`
	Kept                 int           `json:"kept"`
	Carved               int           `json:"carved"`
	Refused              int           `json:"refused"`
	TaskStatementSuspect bool          `json:"task_statement_suspect,omitempty"`
}

func Summarize(entries []Entry) Summary {
	s := Summary{ByClass: map[Class]int{}}
	for _, e := range entries {
		s.ByClass[e.Class]++
		switch e.Disposition {
		case DispositionKeep:
			s.Kept++
		case DispositionCarve:
			s.Carved++
		case DispositionRefuse:
			s.Refused++
		}
	}
	if isMajority(s.ByClass[ClassMisunderstood], len(entries)) {
		s.TaskStatementSuspect = true
	}
	return s
}

func isMajority(part, total int) bool { return part*2 > total }

type AccountResult struct {
	Unaccounted []string
	Invalid     []error
}

func (r AccountResult) OK() bool { return len(r.Unaccounted) == 0 && len(r.Invalid) == 0 }

func Account(changed []string, in Scope, rules []ClosureRule, adjudicated []Entry) AccountResult {
	res := AccountResult{}
	seen := make(map[string]Disposition, len(adjudicated))
	for _, e := range adjudicated {
		if prev, dup := seen[e.Path]; dup && prev != e.Disposition {
			res.Invalid = append(res.Invalid, fmt.Errorf("scopedelta: %s: two contradictory decisions (%s and %s) — one path carries one decision", e.Path, prev, e.Disposition))
			continue
		}
		seen[e.Path] = e.Disposition
		if e.Class == ClassClosure && coveredByRule(e.Path, in, rules) == "" {
			res.Invalid = append(res.Invalid, fmt.Errorf("scopedelta: %s: declared CLOSURE but no closure rule covers it — closure is computed, never claimed; if the change is genuinely necessary, name the counterfactual instead", e.Path))
			continue
		}
		if e.Class == ClassInScope && !in.InScope(e.Path) {
			res.Invalid = append(res.Invalid, fmt.Errorf("scopedelta: %s: declared IN-SCOPE but the cycle's scope does not name it", e.Path))
			continue
		}
		if err := e.Validate(); err != nil {
			res.Invalid = append(res.Invalid, err)
			continue
		}
		if err := Admissible(e); err != nil {
			res.Invalid = append(res.Invalid, err)
		}
	}
	res.Unaccounted = Unaccounted(changed, in, rules, adjudicated)
	return res
}
