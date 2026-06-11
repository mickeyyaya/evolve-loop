package bridge

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type adversarialFaultCase struct {
	family     string
	cli        string
	marker     string
	fault      string
	panes      []string
	newSessErr error
	seedDone   bool
	wantCode   int
	assert     func(*testing.T, *fakeTmux, string)
}

func adversarialFaultCases() []adversarialFaultCase {
	families := []struct {
		family string
		cli    string
		marker string
	}{
		{"claude", "claude-tmux", tmuxPromptMarkerDefault},
		{"codex", "codex-tmux", "›"},
		{"agy", "agy-tmux", "? for shortcuts"},
	}
	var out []adversarialFaultCase
	for _, f := range families {
		out = append(out,
			adversarialFaultCase{
				family: f.family, cli: f.cli, marker: f.marker, fault: "stall",
				panes: []string{"booting without ready marker"}, wantCode: ExitREPLBootTimeout,
			},
			adversarialFaultCase{
				family: f.family, cli: f.cli, marker: f.marker, fault: "crash",
				newSessErr: errors.New("tmux server crashed"), wantCode: ExitBadFlags,
			},
			adversarialFaultCase{
				family: f.family, cli: f.cli, marker: f.marker, fault: "update-menu",
				panes: []string{f.marker}, seedDone: true, wantCode: ExitOK,
			},
			adversarialFaultCase{
				family: f.family, cli: f.cli, marker: f.marker, fault: "weak-busy",
				panes:    []string{f.marker, "tokens: 10\nstill thinking", "tokens: 20\nstill thinking"},
				wantCode: ExitArtifactTimeout,
			},
			adversarialFaultCase{
				family: f.family, cli: f.cli, marker: f.marker, fault: "empty-pane",
				panes: []string{""}, wantCode: ExitREPLBootTimeout,
			},
			adversarialFaultCase{
				family: f.family, cli: f.cli, marker: f.marker, fault: "malformed",
				panes:    []string{f.marker, "!!! malformed terminal frame !!!"},
				wantCode: ExitArtifactTimeout,
			},
			// rate-limit-wall: the cycle-283 quota wall — the REPL boots, work
			// is submitted, then the provider wall appears. The autoresponder
			// must CLASSIFY (pattern rate_limit), persist the escalation
			// report (the artifact the runner's bench-writer consumes), and
			// escalate with ExitUnknownPrompt instead of stalling to the
			// artifact timeout.
			adversarialFaultCase{
				family: f.family, cli: f.cli, marker: f.marker, fault: "rate-limit-wall",
				panes:    []string{f.marker, rateLimitWallFor(f.family)},
				wantCode: ExitUnknownPrompt,
			},
		)
	}
	for i := range out {
		if out[i].family == "codex" && out[i].fault == "update-menu" {
			out[i].panes = []string{cycle274CodexUpdateMenu, "ready ›"}
			out[i].assert = func(t *testing.T, tmux *fakeTmux, _ string) {
				t.Helper()
				if !tmux.sentContains("2") {
					t.Fatalf("codex update menu should be dismissed with Skip=2; sent=%v", tmux.sentSeq)
				}
			}
		}
	}
	return out
}

// rateLimitWallFor returns each family's real wall phrasing (matching that
// family's manifest rate_limit regex). The codex text is verbatim from the
// cycle-283 escalation report — the incident this fault replays.
func rateLimitWallFor(family string) string {
	switch family {
	case "codex":
		return "■ You've hit your usage limit. Upgrade to Pro (https://chatgpt.com/explore/pro), " +
			"visit https://chatgpt.com/codex/settings/usage to purchase more credits or try again at 6:11 AM."
	case "claude":
		return "Claude usage limit reached. Your limit will reset at 6:11 AM."
	default: // agy / gemini family
		return "RESOURCE_EXHAUSTED: rate limit exceeded — try again in 2 hours."
	}
}

func TestAdversarialFaultMatrix(t *testing.T) {
	for _, tc := range adversarialFaultCases() {
		tc := tc
		t.Run(tc.family+"_"+tc.fault, func(t *testing.T) {
			fx := newFixture(t, tc.cli, "")
			if tc.seedDone {
				if err := os.WriteFile(fx.artifact, []byte("<!-- challenge-token: "+fx.token+" -->\nDONE\n"), 0o644); err != nil {
					t.Fatalf("seed artifact: %v", err)
				}
			}
			tmux := &fakeTmux{paneSeq: tc.panes, newSessErr: tc.newSessErr}
			code, stderr := runTmuxCLI(t, fx, tc.cli, tmux, nil, "--allow-bypass")
			if code != tc.wantCode {
				t.Fatalf("%s/%s exit=%d, want %d; stderr=%s", tc.family, tc.fault, code, tc.wantCode, stderr)
			}
			if tc.fault == "rate-limit-wall" {
				raw, err := os.ReadFile(filepath.Join(fx.ws, "escalation-report.json"))
				if err != nil {
					t.Fatalf("%s wall left no escalation report (the bench-writer's input): %v", tc.family, err)
				}
				if !strings.Contains(string(raw), `"pattern_name": "rate_limit"`) && !strings.Contains(string(raw), `"pattern_name":"rate_limit"`) {
					t.Fatalf("%s wall escalation report not classified rate_limit: %s", tc.family, raw)
				}
			}
			if tc.assert != nil {
				tc.assert(t, tmux, stderr)
			}
		})
	}
}

func TestAdversarialFaultMatrix_RequiredFamiliesCovered(t *testing.T) {
	seen := map[string]bool{}
	for _, tc := range adversarialFaultCases() {
		seen[tc.family] = true
	}
	for _, family := range []string{"claude", "codex", "agy"} {
		if !seen[family] {
			t.Fatalf("missing adversarial driver family %q", family)
		}
	}
}

func TestAdversarialFaultMatrix_RequiredFaultTypesPresent(t *testing.T) {
	seen := map[string]bool{}
	for _, tc := range adversarialFaultCases() {
		seen[strings.ReplaceAll(tc.fault, "-", "")] = true
	}
	for _, fault := range []string{"stall", "crash", "updatemenu", "weakbusy", "emptypane", "malformed", "ratelimitwall"} {
		if !seen[fault] {
			t.Fatalf("missing adversarial fault type %q", fault)
		}
	}
}
