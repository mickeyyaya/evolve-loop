package bridge

import (
	"fmt"
	"io"
	"regexp"
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

func warnPromptRuleDrift(w io.Writer, pfx, cli, pane string) {
	if w == nil || strings.TrimSpace(pane) == "" {
		return
	}
	m, err := LoadManifest(cli)
	if err != nil {
		return
	}
	subject := autoRespondSubject(pane)
	for _, p := range m.InteractivePrompts {
		if p.DriftProbe == "" || promptMatches(p, subject) {
			continue
		}
		tail := subject
		if p.TailLines > 0 {
			tail = lastLines(subject, p.TailLines)
		}
		if probe, err := regexp.Compile(p.DriftProbe); err == nil && probe.MatchString(tail) {
			fmt.Fprintf(w, "%s POSSIBLE PROMPT-RULE DRIFT: the teardown pane matches the drift_probe_regex of %s's interactive_prompts.%s but its regex did not; the dialog wording may have changed; update the rule regex (diagnostic only, this exit-81 verdict is unchanged).\n", pfx, cli, p.Name)
		}
	}
}
