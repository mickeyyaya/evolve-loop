// Package scopedelta adjudicates the work a phase agent produced OUTSIDE its
// declared scope — on what the change means, not on the fact that it is
// outside.
//
// PROBLEM. Scope is a proxy for "how much unreviewed surface is entering the
// tree", and the pipeline has been treating the proxy as a verdict: in-scope
// passes, out-of-scope dies. That is wrong in both directions. An in-scope
// change can be terrible; an out-of-scope change is often the thing that makes
// the in-scope change correct — the call site its signature broke, the covering
// test its new export requires, the doc it falsified. And the disposal is
// SILENT: ship stages by declared manifest, so an unlisted path never reaches
// the commit and nobody ever decided to lose it. This repo has the scar —
// a complete, audited salvage implementation sat "built, green, and stranded in
// ten-plus continuation worktrees" until it was recovered by hand.
//
// APPROACH. Two halves, and the second is what makes the first safe:
//
//  1. CLASSIFY BY MEANING. "Out of scope" is six different things (Class), and
//     they want opposite dispositions — necessary closure must be KEPT (the
//     alternative is a broken tree), a discovered defect wants CARVE (preserve
//     the work, land it separately), a boundary violation wants REFUSE (policy,
//     not merit) but still PRESERVED.
//  2. ACCOUNT FOR EVERY PATH. Unaccounted is the whole point of the package:
//     a changed path that is neither in scope, nor computed closure, nor
//     adjudicated, is a PIPELINE DEFECT — the caller blocks on it. Silent
//     disposal stops being expressible.
//
// The anti-laundering hinge is that closure is COMPUTED (ClosureRule), never
// accepted on the agent's word: "necessary collateral" is exactly the label a
// producer would reach for to smuggle anything, so a claim no rule confirms is
// downgraded rather than honoured.
//
// Pure: no I/O, no clock, no globals. Every decision here is a function of its
// arguments, which is what lets the gate test its own contract exhaustively.

package scopedelta

import (
	"fmt"
	"path"
	"regexp"
	"strings"
)

// Class names WHY a path sits outside the declared scope. The taxonomy is the
// point: a single boolean cannot carry a disposition, and every one of these
// wants a different answer.
type Class string

const (
	// ClassClosure — the in-scope change is not correct without it (call sites
	// of a changed signature, a covering test for a new export, a doc the
	// change falsified). Computed, never declared. Kept.
	ClassClosure Class = "closure"
	// ClassDiscovered — a real, unrelated defect the agent found while working.
	// The FINDING is valuable; the FIX is unreviewed risk. Carve.
	ClassDiscovered Class = "discovered"
	// ClassOpportunistic — a refactor or cleanup the agent thought was nice.
	ClassOpportunistic Class = "opportunistic"
	// ClassMisunderstood — the agent genuinely believed this was the task.
	// Evidence about the ITEM's wording, not about the agent.
	ClassMisunderstood Class = "misunderstood"
	// ClassBoundary — a protected, operator-owned surface (ADR-0074). Policy,
	// not merit: refused whatever the justification, and preserved anyway.
	ClassBoundary Class = "boundary"
	// ClassCrossLane — the change belongs to a sibling lane's scope.
	ClassCrossLane Class = "cross-lane"
	// ClassInScope — not a delta at all. Present so Classify is TOTAL: a
	// wiring author who passes the full changed set must not have the cycle's
	// own declared work carved away (review MEDIUM).
	ClassInScope Class = "in-scope"
)

// Disposition is what HAPPENS to the path. Carve is the capability the pipeline
// lacked: without it the only choices were smuggle-it-in or lose-it, so agents
// rationally did the former and gates rationally killed the lane.
type Disposition string

const (
	DispositionKeep   Disposition = "keep"   // ships with this cycle
	DispositionCarve  Disposition = "carve"  // removed from the ship, preserved as a queued item
	DispositionRefuse Disposition = "refuse" // not admitted; preserved for the operator
)

// Scope is what this cycle was licensed to touch.
type Scope struct {
	Cycle int
	// Declared paths the cycle's contract and lane scope named.
	Declared []string
	// Protected surfaces are operator-owned; a lane may not edit them
	// regardless of merit (ADR-0074). Directory prefixes end in "/".
	Protected []string
	// LaneOthers are the scopes of sibling lanes in this wave.
	LaneOthers []string
}

// Declaration is the producing agent's own account of a path: the cheap half of
// the contract, because the agent already knows why it touched the file — it
// has simply never been asked.
type Declaration struct {
	Class         Class
	Justification string
}

// Entry is one adjudicated path: the classification, the decision, and the
// reason the decision is defensible.
type Entry struct {
	Path          string      `json:"path"`
	Class         Class       `json:"class"`
	Justification string      `json:"justification,omitempty"`
	Disposition   Disposition `json:"disposition"`
	Reason        string      `json:"reason"`
	// PatchRef points at the preserved hunks for a carve or a refusal. Its
	// absence on a carve is what turns "carve" back into silent disposal.
	PatchRef string `json:"patch_ref,omitempty"`
	// Preserve records that the work survives even though it is not admitted.
	Preserve bool `json:"preserve,omitempty"`
	// Effect is the direction this change moves the bar, when the path is part
	// of the judging apparatus (D3, gaming.go). Unknown is honest: it is not
	// always mechanically decidable.
	Effect Effect `json:"effect,omitempty"`
	// Corroboration is the evidence from outside the author that a KEEP rests
	// on (D1/D4, gaming.go).
	Corroboration Corroboration `json:"corroboration,omitempty"`
}

// ClosureRule decides whether an out-of-scope path is NECESSARY closure of the
// in-scope change. Strategy, one rule per invariant, so the set can grow from
// observed firings rather than speculation — and so "is this closure?" always
// has a mechanical answer instead of a persuasive one.
type ClosureRule interface {
	// Name identifies the rule in the record, so a KEEP can always be traced
	// to the invariant that licensed it.
	Name() string
	// Covers reports whether p is closure of the given scope.
	Covers(p string, in Scope) bool
}

// DefaultClosureRules is the live rule set. Deliberately small: every rule here
// is one an unconfirmed claim would otherwise have to be trusted for, and a
// rule that cannot be decided from the paths alone does not belong in this
// list — it belongs in adjudication.
func DefaultClosureRules() []ClosureRule {
	return []ClosureRule{
		samePackageTestRule{},
		cycleArtifactRule{},
		goBuildMetadataRule{},
	}
}

// samePackageTestRule: a Go test file in a package the cycle already touches.
// Refusing this class produces the one outcome nobody wants — a change whose
// covering test was dropped for being "out of scope".
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

// cycleArtifactRule: this cycle's own predicate package and workspace. They are
// minted per cycle and are how the cycle proves itself; treating them as
// foreign is a bookkeeping error, not a scope decision.
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

// goBuildMetadataRule: the module and coverage-enrollment files a Go change
// mechanically requires. This change is its own witness — a new internal
// package must be enrolled in .apicover-enforce or the coverage gate reds, so
// carving that one line lands the package with its own gate disabled. Scoped
// to cycles that actually touched Go, or every cycle would silently license
// edits to the module graph.
type goBuildMetadataRule struct{}

func (goBuildMetadataRule) Name() string { return "go-build-metadata" }

func (goBuildMetadataRule) Covers(p string, in Scope) bool {
	switch p {
	case "go/go.mod", "go/go.sum", "go/.apicover-enforce":
	default:
		return false
	}
	for _, d := range in.Declared {
		if strings.HasSuffix(d, ".go") {
			return true
		}
	}
	return false
}

// InScope reports whether p was licensed directly (not by closure).
func (s Scope) InScope(p string) bool { return matchesAny(p, s.Declared) }

// isProtected reports whether p sits on an operator-owned surface.
func (s Scope) isProtected(p string) bool { return matchesAny(p, s.Protected) }

// belongsToSiblingLane reports whether p is another lane's business.
func (s Scope) belongsToSiblingLane(p string) bool { return matchesAny(p, s.LaneOthers) }

// matchesAny reports whether p is the entry itself or sits UNDER it.
//
// The trailing "/" is optional by design (review MEDIUM). The convention used
// to be load-bearing: an operator writing Protected: ["go/internal/phases/ship"]
// got an exact-path rule that no file ever matches, so the entire protected
// surface fell through to carve — fail-OPEN on the one class that is policy
// rather than merit. Matching entry+"/" as a prefix removes the trap without
// loosening anything: the separator is still required, so "docs/research" never
// matches "docs/researchers/x.md".
func matchesAny(p string, set []string) bool {
	for _, s := range set {
		if s == "" {
			continue
		}
		if p == strings.TrimSuffix(s, "/") {
			return true
		}
		if strings.HasPrefix(p, strings.TrimSuffix(s, "/")+"/") {
			return true
		}
	}
	return false
}

// Classify decides one path's class and its default disposition.
//
// Precedence is deliberate and not negotiable by the producer:
//
//	protected surface  → boundary, refused, preserved   (policy, not merit)
//	computed closure   → closure, kept                  (a rule confirmed it)
//	sibling lane       → cross-lane, carved             (hand it over)
//	otherwise          → the agent's declared class, defaulting to
//	                     opportunistic, and NEVER closure — an unconfirmed
//	                     closure claim is the laundering label for anything.
//
// The returned Entry is a PROPOSAL: an adjudicator may move a discovered defect
// from carve to keep with a reason. It may never move a boundary refusal.
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
		// Downgraded, loudly: the rules did not confirm it, and honouring the
		// claim would make "necessary collateral" a universal passphrase.
		class = ClassOpportunistic
	}
	return Entry{
		Path: p, Class: class, Justification: d.Justification,
		Disposition: DispositionCarve, Preserve: true,
		Reason: "outside the declared scope and unconfirmed by any closure rule; preserved as queued work rather than shipped unreviewed",
	}
}

// coveredByRule returns the name of the first rule covering p, or "".
func coveredByRule(p string, in Scope, rules []ClosureRule) string {
	for _, r := range rules {
		if r.Covers(p, in) {
			return r.Name()
		}
	}
	return ""
}

// Unaccounted returns the changed paths that were neither in scope, nor
// computed closure, nor adjudicated.
//
// This is the never-drop invariant, and the reason the package exists. A
// non-empty result is a PIPELINE defect, not a task failure: work is about to
// vanish with no decision attached to it. The caller blocks.
//
// Precedent for the shape: an unaccounted defect disposition already blocks a
// ship the same way. This is that mechanism pointed at scope.
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

// categoryVocabulary is the language a reason can be built from while still
// saying nothing: the category itself, its negations, and filler.
//
// A DENYLIST of set phrases was the first attempt and it failed the way
// denylists do — the reason is LLM-authored, so paraphrase is the normal case,
// not the adversarial one, and "not this cycle's scope" / "different scope" /
// "unrelated" all sailed through (review MEDIUM). The floor is inverted
// instead: strip the vocabulary and require that something SUBSTANTIVE remains.
// A reason that survives is naming something the category does not.
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

// reasonWordRE splits a reason into comparable word tokens.
// Hyphens and slashes are SEPARATORS, not word characters: "out-of-scope" and
// "n/a" must tokenise to their parts, or the compound spelling walks straight
// past a vocabulary built from the simple one.
var reasonWordRE = regexp.MustCompile(`[A-Za-z0-9_]+`)

// saysMoreThanTheCategory reports whether the reason names anything beyond the
// scope category itself.
func saysMoreThanTheCategory(reason string) bool {
	for _, w := range reasonWordRE.FindAllString(strings.ToLower(reason), -1) {
		if !categoryVocabulary[w] {
			return true
		}
	}
	return false
}

// Validate reports whether an adjudication is defensible on its face.
//
// The rule that carries the design: a refusal must name a RISK. "Out of scope"
// restates the category and engages with nothing, and a reviewer permitted to
// stop there never considers what the change MEANS — which is the entire defect
// this package addresses. KEEP carries the same obligation in the other
// direction, because it admits unreviewed surface. CARVE must name where the
// work went, or it is silent disposal wearing a better word.
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
	// Policy is not reversible by the record (review HIGH). Classify refuses a
	// protected surface whatever the justification; without this an adjudicator
	// could agree the finding is real and simply KEEP an operator-owned edit,
	// and both Validate and the accounting would have approved it.
	if e.Class == ClassBoundary && e.Disposition != DispositionRefuse {
		return fmt.Errorf("scopedelta: %s: a boundary path may only be REFUSED (got %s) — a protected operator-owned surface is policy, not merit, and is not adjudicable by the phase that touched it", e.Path, e.Disposition)
	}
	if e.Disposition == DispositionCarve && strings.TrimSpace(e.PatchRef) == "" {
		return fmt.Errorf("scopedelta: %s: a carve must name the patch preserving the work, or it is a silent drop", e.Path)
	}
	return nil
}

// Summary is the per-cycle rollup — the feedback half. Enforcement alone
// repeats this repo's meta-defect (diagnosis that never becomes a queued fix),
// so the shape of a cycle's delta has to say something actionable about the
// SYSTEM, not just about this cycle.
type Summary struct {
	ByClass map[Class]int `json:"by_class"`
	Kept    int           `json:"kept"`
	Carved  int           `json:"carved"`
	Refused int           `json:"refused"`
	// TaskStatementSuspect is set when scope MISUNDERSTANDING dominates the
	// delta: that is evidence the item was ambiguous, so re-dispatching the
	// same agent against the same wording reproduces it. The fix belongs in
	// triage, not in the agent.
	TaskStatementSuspect bool `json:"task_statement_suspect,omitempty"`
}

// Summarize rolls up one cycle's adjudicated entries.
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
	// Dominant, not merely present: one misread path is noise; a majority of
	// them is a statement about the item's wording.
	if n := s.ByClass[ClassMisunderstood]; n > 0 && n*2 > len(entries) {
		s.TaskStatementSuspect = true
	}
	return s
}

// AccountResult is the ship-time answer: what was left undecided, and which
// decisions are not defensible on their face. Either being non-empty blocks.
type AccountResult struct {
	// Unaccounted paths changed but were neither in scope, nor computed
	// closure, nor adjudicated — work about to vanish with no decision
	// attached to it.
	Unaccounted []string
	// Invalid names the adjudications that do not hold up (a refusal that only
	// restates the category, a carve that names no patch, a boundary path the
	// record tried to keep).
	Invalid []error
}

// OK reports whether the cycle may ship on scope grounds.
func (r AccountResult) OK() bool { return len(r.Unaccounted) == 0 && len(r.Invalid) == 0 }

// Account is THE seam: it answers "is every changed path decided, and is every
// decision defensible" in one call.
//
// It exists because the two questions were separable and therefore separated —
// Unaccounted alone counted an entry as accounted merely for EXISTING, so a
// carve naming no patch satisfied the never-drop invariant it was meant to
// violate (review HIGH). The invariant then held only by caller discipline,
// which is exactly the kind of guarantee this package was written to stop
// relying on. Callers should use Account; Unaccounted remains exported for the
// narrower question and for tests that want the halves apart.
func Account(changed []string, in Scope, rules []ClosureRule, adjudicated []Entry) AccountResult {
	res := AccountResult{}
	seen := make(map[string]Disposition, len(adjudicated))
	for _, e := range adjudicated {
		// One path, one decision. Two contradictory entries both validated
		// before, and whichever the consumer read first decided what shipped
		// (review MEDIUM).
		if prev, dup := seen[e.Path]; dup && prev != e.Disposition {
			res.Invalid = append(res.Invalid, fmt.Errorf("scopedelta: %s: two contradictory decisions (%s and %s) — one path carries one decision", e.Path, prev, e.Disposition))
			continue
		}
		seen[e.Path] = e.Disposition
		// RE-DERIVE the mechanically-established classes instead of trusting
		// the record (review BLOCK). Admissible exempts closure and in-scope
		// from the evidence layer, so a producer that simply DECLARED either
		// would skip every check with one string field — the anti-gaming hinge
		// bypassed at the only seam that admits code.
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
		// Well-formed is not the same as supportable: Validate asks whether the
		// decision was stated properly, Admissible whether the EVIDENCE carries
		// it (gaming.go). Both block, because a keep that no counterfactual
		// backs is exactly as unshippable as one with no reason at all.
		if err := Admissible(e); err != nil {
			res.Invalid = append(res.Invalid, err)
		}
	}
	// A path carrying an entry is not UNACCOUNTED even when that entry is
	// indefensible — it was decided, badly, and saying "you never decided this"
	// would send the operator after the wrong defect. The ship still blocks:
	// OK() requires both lists empty, so an invalid decision is exactly as
	// blocking as a missing one, just diagnosed as itself.
	res.Unaccounted = Unaccounted(changed, in, rules, adjudicated)
	return res
}
