package audit

import (
	"os/exec"
	"strings"
)

const integrationTierFailTemplate = "the integration tier (`go test -tags integration`) reported %d offender(s) locally.%s Offenders: %s"

func ciParityCaveat(lookPath func(string) (string, error)) string {
	if _, err := lookPath("tmux"); err != nil {
		return ""
	}
	return " NOTE — host/CI parity gap: this host HAS tmux and CI runners do not," +
		" so every test guarded by requireTmux runs here and SKIPs in CI; those offenders may correspond to no CI failure." +
		" An exit=80 (REPL boot timeout) offender is usually host contention — concurrent lanes hold tmux sessions — not a defect." +
		" Confirm against a quiet host before treating it as one."
}

func ciParityCaveatNow() string { return ciParityCaveat(exec.LookPath) }

func integrationTierTemplateWithCaveat(caveat string) string {
	return strings.Replace(integrationTierFailTemplate, "%s", strings.ReplaceAll(caveat, "%", "%%"), 1)
}
