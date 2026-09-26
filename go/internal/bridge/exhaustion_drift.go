package bridge

import (
	"fmt"
	"io"
	"strings"
)

func warnExhaustionRegexDrift(w io.Writer, pfx, cli, pane, exhaustedRegex string) {
	if w == nil || strings.TrimSpace(pane) == "" {
		return
	}
	probe := manifestDriftProbePattern(cli)
	if probe == "" {
		return
	}
	if matchExhausted(probe, pane) && !matchExhausted(exhaustedRegex, pane) {
		fmt.Fprintf(w, "%s POSSIBLE EXHAUSTION-REGEX DRIFT: the teardown pane matches a broad quota-wall heuristic but %s's controls.usage.exhausted_regex did not — the wall wording may have changed; update the exhausted_regex (diagnostic only, this exit-81 verdict is unchanged).\n", pfx, cli)
	}
}

func manifestDriftProbePattern(cli string) string {
	m, err := LoadManifest(cli)
	if err != nil {
		return ""
	}
	if spec, ok := m.Control("usage"); ok {
		return spec.DriftProbeRegex
	}
	return ""
}
