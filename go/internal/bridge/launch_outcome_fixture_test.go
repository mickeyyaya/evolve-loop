package bridge

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
)

const (
	launchOutcomeFixtureCLI      = "unit10-launch-outcome"
	launchFixtureArtifactBody    = "ARTIFACT-BYTES\n"
	launchFixtureScrollbackBody  = "SCROLLBACK-BYTES\n"
	launchFixtureMarkerSubmitWed = "[bridge] artifact-timeout: cause=submit_wedged reason=\"prompt submit wedged\" phase=build cycle=3 driver=claude-tmux artifact=\"a.md\" waited=300s interval=300s extends_used=0 max_extends=6 last_review=none liveness=hung progressed=false busy=false transient=false detector_error=\"\""
)

// launchStderrFixtures are the seven captured-stderr shapes the classifier sees: nothing; a gauntlet cause
// first; driver chatter ending in the causal line; tmux chatter then the artifact-timeout marker with a
// typed cause; a marker whose only `cause=` token is quoted prose; a 301-rune last line; a 1100-rune marker line.
var launchStderrFixtures = map[string]string{
	"empty": "",
	"first-bridge-line": "[bridge] launch: missing required (flag or env): --profile\n" +
		"[bridge] valid: plan, default\n",
	"last-line-only": "[claude-tmux] NOTE: stream_output=true is no-op for this driver\n" +
		"[claude-tmux] prompt delivered\n" +
		"[claude-tmux] FAIL: completion never signalled\n",
	"marker-submit-wedged": "[bridge] WARN: EVOLVE_SANDBOX=on but inner sandbox not applied\n" +
		"[claude-tmux] FAIL: completion never signalled\n" +
		"[claude-tmux]   audit-report.md\n" +
		launchFixtureMarkerSubmitWed + "\n",
	"marker-prose": "[claude-tmux] chatter\n" +
		"[bridge] artifact-timeout: phase=build reason=\"quoted cause=submit_wedged text\" waited=20s\n",
	"long-line":   "[claude-tmux] chatter\n" + strings.Repeat("界", 301) + "\n",
	"long-marker": "[bridge] artifact-timeout: cause=incomplete " + strings.Repeat("界", 1100) + "\n",
}

// launchOutcomeFixtureDriver replays its ExtraFlags script.
type launchOutcomeFixtureDriver struct{}

func (launchOutcomeFixtureDriver) Name() string { return launchOutcomeFixtureCLI }

func (launchOutcomeFixtureDriver) Launch(_ context.Context, cfg *Config, deps Deps) (int, error) {
	code := ExitOK
	for _, flag := range cfg.ExtraFlags {
		switch {
		case strings.HasPrefix(flag, "exit="):
			code, _ = strconv.Atoi(strings.TrimPrefix(flag, "exit="))
		case strings.HasPrefix(flag, "stderr="):
			fmt.Fprint(deps.Stderr, launchStderrFixtures[strings.TrimPrefix(flag, "stderr=")])
		case flag == "artifact=both":
			_ = os.WriteFile(cfg.Artifact, []byte(launchFixtureArtifactBody), 0o644)
			_ = os.WriteFile(cfg.StdoutLog, []byte(launchFixtureScrollbackBody), 0o644)
		}
	}
	return code, nil
}

// ensureLaunchFixtureDriver registers the fixture driver on demand rather than at init, so the
// registry-reset tests (which restore only the seven builtins) and TestDriverRegistry's strict count are
// unaffected; Register panics on a duplicate, so a prior launch's registration is skipped.
func ensureLaunchFixtureDriver(t *testing.T) {
	t.Helper()
	if _, ok := LookupDriver(launchOutcomeFixtureCLI); !ok {
		Register(launchOutcomeFixtureDriver{})
	}
}

// launchFixtureScript builds the ExtraFlags script for one launch.
func launchFixtureScript(code int, stderrFixture string, extra ...string) []string {
	script := []string{"exit=" + strconv.Itoa(code)}
	if stderrFixture != "" {
		script = append(script, "stderr="+stderrFixture)
	}
	return append(script, extra...)
}
