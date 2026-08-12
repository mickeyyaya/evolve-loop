// gaming.go — telling MEANINGFUL out-of-scope work apart from work that is
// gaming its own evaluation.
//
// THE PREMISE. A justification is written by the party that benefits from it.
// Any criterion of the form "does the explanation sound good" is therefore
// defeated by a better explanation, and an agent optimising for a verdict will
// produce one. Every discriminator here is instead a relationship between the
// change and something the producing agent does NOT control:
//
//	D1 COUNTERFACTUAL NECESSITY — name what fails WITHOUT the change. An agent
//	   can only produce a real failing command by having done real work; the
//	   claim costs what the work costs, which is the whole point.
//	D2 SIGNAL vs SUBJECT — does the path edit the thing being judged, or the
//	   apparatus doing the judging? Editing the measurement is categorically
//	   different from editing the measured, and it is where every interesting
//	   cheat lives (weakened test, widened gate, forged evidence).
//	D3 DIRECTION OF EFFECT — does it LOOSEN the bar or tighten it? Nobody games
//	   by raising their own bar. A loosening of the apparatus that judges this
//	   very change is the strongest single signal available, and the one case
//	   where the justification is simply irrelevant: an agent that needs a
//	   weaker test has said what it could not do.
//	D4 CORROBORATION — does anything the agent did not author agree the work was
//	   needed? A pre-existing queued item, an executed counterfactual, a
//	   coverage line. Narrative alone is uncorroborated by construction.
//
// THE HONEST LIMIT, stated because a gate that overclaims is worse than one
// that does less: none of this decides whether an uncorroborated claim is TRUE.
// It decides what may SHIP on the strength of it. The default for the
// undecidable case is CARVE — preserve, do not ship — which makes both error
// directions recoverable: a missed cheat gains the agent nothing (the code does
// not land), and a missed genuine improvement loses nothing but time (the work
// is queued with its patch). Optimising a classifier is optional when the
// failure modes are symmetric and non-destructive; that property is the real
// safety argument, not the accuracy of the rules below.
//
// AND THE DESIGN GOAL THAT FOLLOWS: make the cheapest path to "accepted"
// indistinguishable from doing the work honestly. An agent that produces a real
// counterfactual, tightens rather than loosens, and points at a pre-existing
// item has — by construction — done the thing we wanted.

package scopedelta

import (
	"fmt"
	"path"
	"strings"
)

// Surface names what a path DOES in the pipeline (D2).
type Surface string

const (
	// SurfaceSubject — product code and prose: the thing being judged.
	SurfaceSubject Surface = "subject"
	// SurfaceSignal — tests, acceptance predicates, gates, personas, policy
	// dials, evidence sidecars and graded artifacts: the apparatus doing the
	// judging. An agent editing this is editing its own examination.
	SurfaceSignal Surface = "signal"
)

// Effect is the direction a change moves the bar (D3). Unknown is the honest
// default: it is not always mechanically decidable, and guessing would put a
// number on something the adjudicator should look at.
type Effect string

const (
	EffectUnknown  Effect = ""
	EffectTightens Effect = "tightens"
	EffectLoosens  Effect = "loosens"
)

// Corroboration is evidence from OUTSIDE the author (D1, D4). Every field is
// something a third party can re-run or look up; none of it is prose.
type Corroboration struct {
	// FailsWithout claims the executed negative control: revert the change and
	// something reds. Meaningless without Command — an assertion in the costume
	// of evidence.
	FailsWithout bool `json:"fails_without,omitempty"`
	// Command is what was run to establish FailsWithout, verbatim, so the
	// adjudicator can re-run it rather than believe it.
	Command string `json:"command,omitempty"`
	// QueuedItemID names a PRE-EXISTING inbox item describing this work. The
	// value is in the ordering: the finding existed before this agent claimed
	// it, so the claim is not self-minted.
	QueuedItemID string `json:"queued_item_id,omitempty"`
	// CoverageLine is a profile line proving a new branch actually executes —
	// the repo's standing answer to "shipped with zero executions".
	CoverageLine string `json:"coverage_line,omitempty"`
}

// present reports whether anything outside the author backs the claim.
func (c Corroboration) present() bool {
	return (c.FailsWithout && strings.TrimSpace(c.Command) != "") ||
		strings.TrimSpace(c.QueuedItemID) != "" ||
		strings.TrimSpace(c.CoverageLine) != ""
}

// signalDirs are path prefixes whose contents judge rather than are judged.
var signalDirs = []string{
	"go/acs/",             // acceptance predicates
	"agents/", ".agents/", // the personas that grade
	"skills/",                   // the procedures reviews follow
	".evolve/policy",            // the dials gates read
	".evolve/profiles",          // permission profiles
	".evolve/runs/",             // graded artifacts and their evidence
	".evolve/evals/",            // eval definitions
	"go/internal/policy/",       // the COMPILED gate defaults the JSON only overrides
	"go/internal/config/",       // ditto
	"go/internal/guards/",       // live deny hooks
	"go/internal/core/",         // the phase state machine and escalation
	"go/internal/phases/audit/", // the phase that grades
	"go/internal/phases/ship/",  // the phase that admits
	".github/workflows/",        // CI is judging apparatus too
}

// signalFileHints are basenames and suffixes that judge wherever they live.
var signalFileHints = []string{"_test.go", ".jsonl", "-baseline.json", "predicates_test.go",
	// Gate CONFIGURATION is apparatus even though it is not code: an
	// enrollment file edit can silently disable another package's coverage
	// gate, and it classified as ordinary subject before (review HIGH).
	".apicover-enforce", "go.mod", "go.sum"}

// SurfaceOf classifies a path as the judged or the judging (D2).
//
// Deliberately coarse and deliberately over-inclusive: a false SIGNAL costs one
// extra corroboration on an honest change, while a false SUBJECT is a cheat
// that never had to prove anything. The asymmetry of those two mistakes is the
// whole reason to guess in this direction.
func SurfaceOf(p string) Surface {
	for _, d := range signalDirs {
		if strings.HasPrefix(p, d) {
			return SurfaceSignal
		}
	}
	base := path.Base(p)
	for _, h := range signalFileHints {
		if strings.HasSuffix(base, h) {
			return SurfaceSignal
		}
	}
	// Gate implementations judge, even though they are ordinary Go.
	if strings.Contains(p, "/gate") || strings.HasSuffix(base, "_gate.go") ||
		strings.Contains(p, "/phases/ship/") || strings.Contains(p, "repocontract") {
		return SurfaceSignal
	}
	return SurfaceSubject
}

// Admissible reports whether an entry's DISPOSITION is supportable by the
// evidence attached to it — the merit test, applied where merit can actually be
// checked.
//
// Only KEEP is gated. Carve and refuse preserve the work without shipping it,
// so they cost nothing to be wrong about; keeping is the only disposition that
// puts unreviewed, out-of-scope bytes into the tree on somebody's say-so, and
// so it is the only one that has to earn it.
func Admissible(e Entry) error {
	if e.Disposition != DispositionKeep {
		return nil
	}
	// Computed closure is already established by a rule the producer cannot
	// influence. Demanding a second proof would tax the one class that is
	// mechanically certain, and teach agents that evidence is a ritual.
	// Closure and in-scope both skip the evidence layer, for opposite reasons
	// and both correctly: closure is established by a rule the producer cannot
	// influence, and in-scope work is not a delta at all — asking a cycle to
	// corroborate its own licensed work would turn this package into the
	// false-RED generator it was written against (review HIGH).
	//
	// NOTE the seam this depends on: the CLASS here is producer-supplied, so
	// Account re-derives it against the rules before trusting either exemption.
	// Admissible alone must never be the only thing standing between a declared
	// class and a ship.
	if e.Class == ClassClosure || e.Class == ClassInScope {
		return nil
	}
	// D3 — the case where the justification does not matter. A change that
	// loosens the apparatus judging this very change may be real, and it is
	// still not the producing agent's call: carve it and let a separate pass
	// decide with nothing riding on the outcome.
	// An UNDECLARED direction on the judging apparatus is not a safe one:
	// omission is cheaper than a lie, and EffectUnknown is the zero value, so
	// leaving the field out would have been the cheapest bypass in the file
	// (review HIGH). Declaring it does not make the claim true — it makes it
	// checkable by the diff review that follows.
	if SurfaceOf(e.Path) == SurfaceSignal && e.Effect == EffectUnknown {
		return fmt.Errorf("scopedelta: %s: a KEEP on the judging apparatus must DECLARE its direction (tightens|loosens) — an unset direction is a claim of safety nobody made", e.Path)
	}
	if SurfaceOf(e.Path) == SurfaceSignal && e.Effect == EffectLoosens {
		return fmt.Errorf("scopedelta: %s: refusing to KEEP a change that LOOSENS the apparatus judging this cycle — carve it instead so the decision is made by a pass with nothing riding on it (the change may well be right; the producer is the wrong adjudicator)", e.Path)
	}
	// D1/D4 — everything else that ships must be backed by something outside
	// the author.
	if !e.Corroboration.present() {
		return fmt.Errorf("scopedelta: %s: a KEEP needs corroboration the author did not write — an executed counterfactual (fails_without + command), a pre-existing queued item, or a coverage line proving the branch runs; narrative alone is what an agent can always produce", e.Path)
	}
	if e.Corroboration.FailsWithout && strings.TrimSpace(e.Corroboration.Command) == "" {
		return fmt.Errorf("scopedelta: %s: fails_without with no command is an assertion in the costume of evidence — name what was run", e.Path)
	}
	return nil
}

// GamingSignals reports patterns visible only across the WHOLE delta.
//
// Single instances are unfalsifiable by design: "I fixed an adjacent bug" can
// always be said, and sometimes it is true. What cannot be disguised is shape —
// a producer whose out-of-scope work is mostly aimed at the apparatus, or
// mostly in the loosening direction, is describing itself regardless of how
// each entry reads. This is population-level detection precisely because
// instance-level detection is the part that cannot be won.
//
// Returns nothing for an ordinary delta: a signal that fires routinely is one
// operators learn to skip.
func GamingSignals(entries []Entry) []string {
	// A floor, because "majority" degenerates to "presence" at len==1 and the
	// modal honest delta is a single covering test — which would have fired
	// every one of these on ordinary work, and a signal that fires routinely is
	// one operators learn to skip (review MEDIUM).
	const floor = 3
	if len(entries) < floor {
		return nil
	}
	var signalPaths, loosening, uncorroboratedKeeps int
	for _, e := range entries {
		// Mechanically-established classes are not evidence about intent.
		if e.Class == ClassClosure || e.Class == ClassInScope {
			continue
		}
		if SurfaceOf(e.Path) == SurfaceSignal {
			signalPaths++
		}
		if e.Effect == EffectLoosens {
			loosening++
		}
		if e.Disposition == DispositionKeep && !e.Corroboration.present() {
			uncorroboratedKeeps++
		}
	}
	var out []string
	// Majorities, not presences: one test edit in a delta is ordinary work.
	if signalPaths*2 > len(entries) {
		out = append(out, fmt.Sprintf("signal-heavy delta: %d of %d out-of-scope paths edit the judging apparatus rather than the code being judged", signalPaths, len(entries)))
	}
	if loosening*2 > len(entries) {
		out = append(out, fmt.Sprintf("loosening pattern: %d of %d changes move the bar DOWN — the direction nobody takes to make their own work harder", loosening, len(entries)))
	}
	if uncorroboratedKeeps*2 > len(entries) {
		out = append(out, fmt.Sprintf("narrative-only deltas: %d of %d keeps rest on the author's account with nothing outside it", uncorroboratedKeeps, len(entries)))
	}
	return out
}
