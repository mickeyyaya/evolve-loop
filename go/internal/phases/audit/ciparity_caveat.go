package audit

// ciparity_caveat.go — the integration-tier gate reports what it MEASURED, not
// what it guesses CI would do.
//
// The gate runs `go test -tags integration` on the cycle host and used to
// conclude "CI's integration-tier test step would FAIL". That inference is only
// valid when the host and CI execute the SAME set of tests, and they do not: the
// real-tmux tier is guarded by requireTmux, which t.Skip()s when tmux is absent
// from PATH. GitHub runners have no tmux. So every requireTmux-guarded test runs
// here and skips there, and a local offender in that set corresponds to no CI
// failure at all.
//
// cycle-1543 was blocked on exactly that: 13 offenders, all
// TestRealTmux_Interactive_*, all exit=80 (REPL BOOT timeout) — host contention
// from the wave's own concurrent agent tmux sessions, not defects. Measured with
// the wave stopped: 7/7 PASS in 17.2s, versus 3.6x-7.7x slower and failing under
// load. Meanwhile main's go job ran the same tier in CI and passed.
//
// The discriminator is DERIVED from the same predicate requireTmux uses rather
// than a list of test files, so the caveat stays true if the guarded set changes
// — a hardcoded file list would rot into a second falsehood.

import (
	"os/exec"
	"strings"
)

// integrationTierFailTemplate is the integration-tier gate's finding. Takes
// (offenderCount, parityCaveat, offenders).
//
// It deliberately reports a LOCAL observation and, when the host diverges from
// CI, says so — instead of asserting a CI outcome. A gate that blocks real work
// citing an impossible CI failure teaches operators to bypass gates, which costs
// far more than the offenders it was trying to surface.
const integrationTierFailTemplate = "the integration tier (`go test -tags integration`) reported %d offender(s) locally.%s Offenders: %s"

// ciParityCaveat names the host↔CI divergence that makes a local offender fail
// to imply a CI failure, or "" when the two environments agree.
//
// lookPath is injected so the caveat is testable without depending on whatever
// the test machine happens to have installed.
func ciParityCaveat(lookPath func(string) (string, error)) string {
	if _, err := lookPath("tmux"); err != nil {
		return ""
	}
	return " NOTE — host/CI parity gap: this host HAS tmux and CI runners do not," +
		" so every test guarded by requireTmux runs here and SKIPs in CI; those offenders may correspond to no CI failure." +
		" An exit=80 (REPL boot timeout) offender is usually host contention — concurrent lanes hold tmux sessions — not a defect." +
		" Confirm against a quiet host before treating it as one."
}

// ciParityCaveatNow is the production reading, using the real PATH.
func ciParityCaveatNow() string { return ciParityCaveat(exec.LookPath) }

// integrationTierTemplateWithCaveat splices the caveat into the finding
// template's caveat slot.
//
// The result is itself a FORMAT STRING (applyCIGate Sprintf's it with count and
// offenders), so any '%' inside the caveat would be read as a verb and corrupt
// the finding. Escaping here means a future editor can write a caveat containing
// '100% of lanes' without silently producing '%!o(MISSING)' in a gate message an
// operator has to act on.
func integrationTierTemplateWithCaveat(caveat string) string {
	return strings.Replace(integrationTierFailTemplate, "%s", strings.ReplaceAll(caveat, "%", "%%"), 1)
}
