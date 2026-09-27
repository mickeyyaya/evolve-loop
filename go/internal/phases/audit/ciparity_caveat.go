package audit

import (
	"os/exec"
	"strings"
)

// integrationTierFailTemplate is the integration-tier gate's finding. Takes
// (offenderCount, parityCaveat, offenders). It deliberately reports a local
// observation, and says so when the host diverges from CI, rather than
// asserting a CI outcome a gate cannot actually observe.
const integrationTierFailTemplate = "the integration tier (`go test -tags integration`) reported %d offender(s) locally.%s Offenders: %s"

// ciParityCaveat names the host↔CI divergence that makes a local offender fail
// to imply a CI failure, or "" when the two environments agree. lookPath is
// injected so the caveat is testable without depending on the test machine's
// own PATH.
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
// template's caveat slot. The result is itself a format string that gets
// Sprintf'd again, so a literal '%' in the caveat is escaped here to avoid
// corrupting the finding.
func integrationTierTemplateWithCaveat(caveat string) string {
	return strings.Replace(integrationTierFailTemplate, "%s", strings.ReplaceAll(caveat, "%", "%%"), 1)
}
