package core

// lost_landing_floor.go — a cycle whose landing was destroyed must not report PASS.
//
// Live incident (wave-20260822a-verify, 2026-08-22): two lanes raced one main.
// cycle-1536 won and landed; cycle-1535 rebased into a genuine non-derived
// conflict on a peer's eval file, routed to the debugger exactly as designed,
// and produced NO commit — then closed out with final_verdict PASS.
//
// Nothing at cycle close was asking the question. `finalizeOutcome` only
// reclassifies a SKIPPED verdict (cycle_outcome.go), so a PASS handed up by the
// audit floor survives untouched whether or not ship ever landed. The one
// existing no-ship notice keys on CycleOutcomeSkippedUnknown, which such a cycle
// never becomes.
//
// Why this is worth a floor rather than a log line: a lost landing that reports
// PASS makes a zero-ship wave read as a BUILDER-QUALITY problem, when it is
// really landing-queue contention. That is the first thing the zero-ship halt
// protocol tells an operator to rule out, and the misreading sends them at the
// agents instead of the queue. The loss was invisible here until someone diffed
// ship-error.json against ship-binding.json by hand, per cycle.
//
// The evidence is the cycle's OWN artifacts, deliberately NOT git HEAD movement:
// in fleet mode a sibling lane moves HEAD constantly, so a HEAD delta would have
// credited cycle-1535 with cycle-1536's commit — the precise confusion this
// exists to end.
//
// Detection is deliberately NARROW: ship ran and recorded an error, ship left no
// binding, and the cycle still claims a shipping verdict. A cycle that never
// reached ship has no landing to lose, and a cycle already reporting FAIL/WARN is
// telling the truth.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// lostLandingVerdict is what a cycle whose landing was destroyed reports
// instead. WARN because the work and its verdict were real — the audit passed
// and the artifacts stand — but the cycle did not deliver. It is also a value
// the dossier's own validation accepts (PASS|WARN|FAIL) and, critically, one
// that IsShippingVerdict rejects, so the cycle stops counting as throughput.
func lostLandingVerdict() string { return VerdictWARN }

// detectLostLanding reports a system-class signal when a cycle claims a shipping
// verdict but its own ship phase produced no binding.
//
// Pure apart from two file stats, so it is exhaustively unit-testable against
// real cycle artifacts. nil means "nothing to say" — the overwhelmingly common
// case, including every cycle that never planned a ship.
func detectLostLanding(workspace, finalVerdict string) *SystemFailureSignal {
	if workspace == "" || !IsShippingVerdict(finalVerdict) {
		return nil
	}
	// A binding is the ship phase's own proof of delivery. Present ⇒ landed,
	// whatever transient errors it survived on the way (cycle-1536 recorded a
	// GIT_FLEET_REBASE_NEEDED and landed anyway — flagging that would make every
	// contended wave a wall of false alarms).
	if _, err := os.Stat(filepath.Join(workspace, "ship-binding.json")); err == nil {
		return nil
	}
	code, class, msg, ok := readShipError(workspace)
	if !ok {
		// No ship error either: the cycle never reached ship. Not a lost
		// landing, and not this floor's business.
		return nil
	}
	return &SystemFailureSignal{
		Category: "landing-lost",
		Level:    "system",
		Halt:     false,
		Evidence: fmt.Sprintf(
			"cycle reported %s but ship produced no commit: ship-error %s (%s) with no ship-binding.json in %s — the work was completed and then discarded. %s",
			finalVerdict, code, class, workspace, msg),
	}
}

// readShipError lifts the code/class/message from the ship phase's own sidecar.
// ok is false when the file is absent or unreadable — treated as "ship never
// ran", the conservative reading, since inventing a landing loss from a missing
// file would fire on every pre-ship cycle.
func readShipError(workspace string) (code, class, msg string, ok bool) {
	b, err := os.ReadFile(filepath.Join(workspace, "ship-error.json"))
	if err != nil {
		return "", "", "", false
	}
	var se struct {
		Code    string `json:"code"`
		Class   string `json:"class"`
		Message string `json:"message"`
	}
	if json.Unmarshal(b, &se) != nil || se.Code == "" {
		return "", "", "", false
	}
	return se.Code, se.Class, se.Message, true
}
