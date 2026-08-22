package bridge

// artifact_timeout_transient_test.go — an artifact-timeout death whose pane
// literally states the cause was a temporary upstream server error must SAY SO.
//
// The defect (inbox item transient-api-error-invisible-inside-artifact-timeout,
// weight 0.90): a transient provider error inside the artifact wait is
// recognized by NOTHING. It is not a quota wall (controls.usage.exhausted_regex
// correctly ignores it), not a transient exit code (80/85/86), and it surfaces
// as exit 81 — contractually non-retryable (transient-bridge-retry AC-1, which
// must stay green). Live evidence: 3 of 4 observed router stalls across cycles
// 1523/1524/1526 burned 600s each on "API Error: 529 Overloaded".
//
// Contract: the ONE self-describing artifact-timeout marker line carries a
// driver-authored transient= field derived from matching the family's
// manifest-declared transient_regex against the CAPTURED PANE.
//
// Two constraints are load-bearing and both are asserted below:
//
//  1. The classifier reads the PANE, never the stderr buffer Engine.Launch
//     inspects (cycle-1528 premise-challenge, severity CRITICAL): every stderr
//     write on the exit-81 path is a bridge-authored note, so a stderr
//     classifier would pass synthetic fixtures 3/3 while never firing live.
//     Hence the fixture here is the VERBATIM final_pane of cycle-1523's
//     router-escalation-report.json, not a hand-written string.
//  2. The field EXTENDS the existing marker line rather than competing with it,
//     so it can never displace artifactTimeoutSummary's cause selection — the
//     engine.go:551-561 regression class stays structurally unreachable.

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// livePane529 is the unedited pane captured when cycle-1523's router died at
// exit 81. Provenance: .evolve/runs/cycle-1523/router-escalation-report.json,
// key final_pane. Read from testdata rather than inlined so the evidence stays
// auditable against the source report.
func livePane529(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "cycle-1523-router-529-pane.txt"))
	if err != nil {
		t.Fatalf("read live 529 pane fixture: %v", err)
	}
	pane := string(b)
	// Guard the fixture itself: a truncated or scrubbed copy would make every
	// assertion below vacuous.
	if !strings.Contains(pane, "API Error: 529 Overloaded") {
		t.Fatalf("fixture no longer carries the live 529 error text — the evidence was lost")
	}
	if !strings.Contains(pane, tmuxPromptMarkerDefault) {
		t.Fatalf("fixture no longer carries the REPL prompt marker — the driver would never boot on it")
	}
	return pane
}

// TestRunTmuxREPL_ArtifactTimeout_MarkerFlagsTransientOnLivePane: driven through
// the real driver with the real captured pane, the summary must report
// transient=true. This is the assertion a stderr-buffer classifier cannot pass.
func TestRunTmuxREPL_ArtifactTimeout_MarkerFlagsTransientOnLivePane(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	tmux := &fakeTmux{paneSeq: []string{livePane529(t)}} // boots; artifact never lands
	rev := &scriptedReviewer{verdicts: []ReviewVerdict{
		{Action: ReviewExtend, Reason: "still working"},
		{Action: ReviewPause, Reason: "agent produced no output"},
	}}
	code, stderr := runTmuxOnStopReview(t, fx, tmux, rev, nil,
		Deps{ArtifactTimeoutS: 2, ArtifactMaxExtends: 5}, "--allow-bypass", "--agent=router")

	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want %d; stderr=%q", code, ExitArtifactTimeout, stderr)
	}
	summary := artifactTimeoutSummary(stderr)
	if summary == "" {
		t.Fatalf("no %q line on stderr; stderr=%q", artifactTimeoutMarker, stderr)
	}
	if !strings.Contains(summary, "transient=true") {
		t.Errorf("the timeout summary does not report the transient upstream error the pane states verbatim — "+
			"a reader (and the router) cannot tell this 600s burn from a genuine wedge\n  got: %s", summary)
	}
	// The added field must be driver-authored, never raw pane text (F1
	// indirect-prompt-injection hazard): no provider prose may ride the cause.
	if strings.Contains(summary, "Overloaded") || strings.Contains(summary, "status.claude.com") {
		t.Errorf("raw pane text reached the recorded cause — the transient field must be driver-authored\n  got: %s", summary)
	}
}

// TestRunTmuxREPL_ArtifactTimeout_SilentPaneIsNotTransient: a genuine wedge —
// no recognized error anywhere on the pane — keeps today's behavior exactly
// (exit 81, pause) and must NOT be labelled transient. Without this, the field
// is a constant and tells the reader nothing.
func TestRunTmuxREPL_ArtifactTimeout_SilentPaneIsNotTransient(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	tmux := &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}}
	rev := &scriptedReviewer{verdicts: []ReviewVerdict{
		{Action: ReviewPause, Reason: "agent idle"},
	}}
	code, stderr := runTmuxOnStopReview(t, fx, tmux, rev, nil,
		Deps{ArtifactTimeoutS: 2, ArtifactMaxExtends: 5}, "--allow-bypass", "--agent=router")

	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want %d; stderr=%q", code, ExitArtifactTimeout, stderr)
	}
	summary := artifactTimeoutSummary(stderr)
	if !strings.Contains(summary, "transient=false") {
		t.Errorf("a silent (wedged) pane must report transient=false — otherwise the field cannot discriminate\n  got: %s", summary)
	}
}

// transientFamilyCase is one CLI family's transient-recognition contract.
//
// Recognition MUST be family-agnostic: the driver resolves the pattern from
// whichever manifest the launch names (lp.name), so a family that declares no
// transient_regex silently loses the diagnosis — the router stalls the same 600s
// on codex or agy as it did on claude, with nothing in the record to say why.
// The exemplars are that provider's OWN transient surface; only claude's is
// live-captured (cycle-1523), the rest are the providers' documented error
// shapes and are marked as such.
type transientFamilyCase struct {
	cli       string
	transient []string // MUST read as transient for this family
	walls     []string // that family's real quota walls — MUST NOT read as transient
}

func transientFamilyCases() []transientFamilyCase {
	return []transientFamilyCase{
		{
			cli: "claude-tmux",
			// Live-captured: .evolve/runs/cycle-1523/router-escalation-report.json.
			transient: []string{
				"API Error: 529 Overloaded. This is a server-side issue, usually temporary — try again in a moment.",
				"API Error: 503 Service Unavailable",
			},
			walls: []string{"You have reached your usage limit", "You've reached your weekly limit · /usage-credits"},
		},
		{
			cli: "codex-tmux",
			// OpenAI documented error surface (not live-captured).
			transient: []string{
				"stream error: server_error; retrying",
				"The server had an error while processing your request",
				"502 Bad Gateway",
			},
			walls: []string{"usage limit reached", "You hit your weekly limit"},
		},
		{
			cli: "agy-tmux",
			// Gemini documented error surface (not live-captured).
			transient: []string{
				"503 UNAVAILABLE: The model is overloaded. Please try again later.",
				"500 INTERNAL",
			},
			walls: []string{"quota exceeded", "usage limit reached"},
		},
		{
			cli: "ollama-tmux",
			// Local server surface (not live-captured).
			transient: []string{
				"Error: 500 Internal Server Error",
				"Error: llama runner process has terminated: exit status 2",
			},
			walls: nil, // ollama is local: it declares no quota wall
		},
	}
}

func TestArtifactTimeoutTransient_LivePaneIsFamilyScopedNotHardCodedText(t *testing.T) {
	pane := livePane529(t)
	for _, tc := range transientFamilyCases() {
		t.Run(tc.cli, func(t *testing.T) {
			got := classifyTransientPane(tc.cli, pane)
			want := tc.cli == "claude-tmux"
			if got != want {
				t.Errorf("classifyTransientPane(%q, live claude 529 pane) = %t, want %t; recognition must come from the launched family's manifest", tc.cli, got, want)
			}
		})
	}
}

// bridgeAuthoredChatter is stderr the BRIDGE itself writes on the exit-81 path.
// None of it may read as transient for ANY family: matching it would prove the
// classifier is pointed at the stderr buffer rather than the pane (the
// cycle-1528 premise-challenge, severity CRITICAL) and would resurrect the class
// of regression where the drift alarm or a workspace listing displaces the
// recorded cause.
var bridgeAuthoredChatter = map[string]string{
	"exit-81 drift alarm (exhaustion_drift.go:39-41)": "POSSIBLE EXHAUSTION-REGEX DRIFT: the teardown pane matches a broad quota-wall heuristic but claude-tmux's controls.usage.exhausted_regex did not — the wall wording may have changed; update the exhausted_regex (diagnostic only, this exit-81 verdict is unchanged).",
	"workspace file listing (driver_common.go:388)":   "routing-plan.json (0 bytes)",
	"completion-never-signalled note":                 "FAIL: completion never signalled (artifact audit-report.md; stop-review paused after 2 interval(s) of 300s)",
	"429 burst vocabulary":                            "429 too many requests, retrying in 20s",
}

// TestEveryTmuxFamilyDeclaresTransientRecognition is the LLM-agnostic contract:
// EVERY tmux CLI family must recognize its own transient upstream errors, and no
// family's pattern may collide with its quota wall or with bridge-authored
// chatter. A new CLI family added without a transient_regex fails here rather
// than silently shipping a blind spot.
func TestEveryTmuxFamilyDeclaresTransientRecognition(t *testing.T) {
	cases := transientFamilyCases()
	for _, tc := range cases {
		t.Run(tc.cli, func(t *testing.T) {
			m, err := LoadManifest(tc.cli)
			if err != nil {
				t.Fatalf("load %s manifest: %v", tc.cli, err)
			}
			pattern := m.TransientRegex
			if pattern == "" {
				t.Fatalf("%s declares no top-level transient_regex — this family cannot tell a temporary "+
					"upstream failure from a wedged pane, so it burns the whole silence budget with nothing "+
					"in the record (the pattern must be manifest-sourced per family, never a Go literal)", tc.cli)
			}
			for _, text := range tc.transient {
				if !matchExhausted(pattern, text) {
					t.Errorf("%s transient_regex misses its own provider's transient error %q", tc.cli, text)
				}
			}
			for _, wall := range tc.walls {
				if matchExhausted(pattern, wall) {
					t.Errorf("%s transient_regex claims the quota wall %q — a permanent wall would be "+
						"labelled temporary", tc.cli, wall)
				}
			}
			for name, text := range bridgeAuthoredChatter {
				if matchExhausted(pattern, text) {
					t.Errorf("%s transient_regex matches bridge-authored %s (%q) — the classifier is reading "+
						"the wrong surface", tc.cli, name, text)
				}
			}
			// No collision in the other direction: the family's wall pattern must
			// not claim its transient errors.
			if spec, ok := m.Control("usage"); ok && spec.ExhaustedRegex != "" {
				for _, text := range tc.transient {
					if matchExhausted(spec.ExhaustedRegex, text) {
						t.Errorf("%s exhausted_regex claims the transient error %q — a temporary blip would "+
							"escalate as a quota wall", tc.cli, text)
					}
				}
			}
		})
	}
	// Guard the guard: the case table must cover every embedded tmux family, so
	// adding a CLI cannot quietly skip this contract.
	covered := map[string]bool{}
	for _, tc := range cases {
		covered[tc.cli] = true
	}
	names, err := manifestFS.ReadDir("manifests")
	if err != nil {
		t.Fatalf("read embedded manifests: %v", err)
	}
	for _, e := range names {
		cli := strings.TrimSuffix(e.Name(), ".json")
		if !strings.HasSuffix(cli, "-tmux") || covered[cli] {
			continue
		}
		t.Errorf("tmux family %q is not covered by this contract — every family that can hit the artifact "+
			"timeout must declare transient recognition", cli)
	}
}

// TestRunTmuxREPL_ArtifactTimeout_TransientFieldIsFamilyAgnostic drives the REAL
// driver for a NON-claude family. The classifier resolves its pattern from the
// launched CLI's manifest, so a claude-only implementation (or a hard-coded
// pattern) fails here while passing every claude-based test.
func TestRunTmuxREPL_ArtifactTimeout_TransientFieldIsFamilyAgnostic(t *testing.T) {
	fx := newFixture(t, "codex-tmux", "")
	// Codex's own prompt marker plus its own transient error text: this pane
	// could not be produced by, or recognized through, the claude manifest.
	pane := "› working…\nstream error: server_error; retrying\n›"
	tmux := &fakeTmux{paneSeq: []string{pane}}
	rev := &scriptedReviewer{verdicts: []ReviewVerdict{{Action: ReviewPause, Reason: "no output"}}}

	d := Deps{Tmux: tmux, Sleep: func(time.Duration) {}, Reviewer: rev,
		ArtifactTimeoutS: 2, ArtifactMaxExtends: 5}
	eng := newTestEngine(d)
	var stdout, stderr bytes.Buffer
	code := eng.LaunchArgs(context.Background(),
		fx.args("codex-tmux", "--allow-bypass", "--agent=router"), nil, &stdout, &stderr)

	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want %d; stderr=%q", code, ExitArtifactTimeout, stderr.String())
	}
	summary := artifactTimeoutSummary(stderr.String())
	if !strings.Contains(summary, "transient=true") {
		t.Errorf("codex-tmux transient error was not recognized — transient recognition is not family-agnostic\n  got: %s", summary)
	}
}

// TestClassifyTransientPane_IgnoresEchoedPromptText: the classifier is scanned
// on the AGENT-STRIPPED pane, mirroring the exhaustion detector (cycle-654
// prompt-echo veto). This repo is the sharpest case for it — a cycle whose task
// IS "handle API Error: 529" injects that phrase into the prompt, and the agent
// echoes it back. A raw-pane scan would then label EVERY artifact timeout in
// that cycle a transient upstream failure, which is the classic
// classifier-reads-its-own-instructions defect.
func TestClassifyTransientPane_IgnoresEchoedPromptText(t *testing.T) {
	const echoed = "Task: make the bridge recognize API Error: 529 Overloaded inside an artifact timeout."
	injected := "Instructions.\n" + echoed + "\nWrite the deliverable."

	pane := "thinking...\n" + echoed + "\nstill working..."
	if classifyTransientPane("claude-tmux", strippedForExhaustionScan(pane, injected)) {
		t.Errorf("the agent's OWN echoed instructions were classified as an upstream outage — "+
			"a cycle working ON this defect would mislabel every one of its timeouts\n  pane: %q", pane)
	}

	// A genuine provider error, absent from the prompt, must still survive the strip.
	genuine := "⏺ API Error: 529 Overloaded. This is a server-side issue, usually temporary."
	if !classifyTransientPane("claude-tmux", strippedForExhaustionScan(genuine, injected)) {
		t.Errorf("a real provider error was stripped away as a prompt echo: %q", genuine)
	}
}

func TestArtifactTimeoutTransient_EchoedProviderTextNeverClassifiesForAnyFamily(t *testing.T) {
	for _, tc := range transientFamilyCases() {
		t.Run(tc.cli, func(t *testing.T) {
			for _, providerText := range tc.transient {
				injected := "Task:\n" + providerText + "\nWrite the deliverable."
				pane := "thinking...\n" + providerText + "\nstill working..."
				stripped := strippedForExhaustionScan(pane, injected)
				if classifyTransientPane(tc.cli, stripped) {
					t.Errorf("%s classified provider text echoed from the injected prompt as a live upstream failure: %q", tc.cli, providerText)
				}
			}
		})
	}
}

// TestRunTmuxREPL_ArtifactTimeout_EchoedPromptIsNotTransient is the DRIVER-level
// half of the prompt-echo veto. The unit guard above pins the helper, but the
// driver chooses which pane it hands that helper — and handing it the raw
// lastGoodPane compiles, passes every other test here, and silently mislabels a
// working agent's echoed instructions as an upstream outage. This test is the
// one that fails on that mutation.
func TestRunTmuxREPL_ArtifactTimeout_EchoedPromptIsNotTransient(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	// The cycle's TASK mentions the provider error verbatim — the realistic case
	// for this repo, whose own backlog contains exactly such an item.
	const taskLine = "Task: recognize API Error: 529 Overloaded inside an artifact timeout."
	if err := os.WriteFile(fx.promptFile, []byte("Instructions.\n"+taskLine+"\nWrite the deliverable.\n"), 0o644); err != nil {
		t.Fatalf("write prompt: %v", err)
	}
	// The agent echoes its instructions back into the pane and then stalls. There
	// is no genuine provider error anywhere on this pane.
	pane := tmuxPromptMarkerDefault + "\n" + taskLine + "\nthinking...\n" + tmuxPromptMarkerDefault
	tmux := &fakeTmux{paneSeq: []string{pane}}
	rev := &scriptedReviewer{verdicts: []ReviewVerdict{{Action: ReviewPause, Reason: "agent idle"}}}

	code, stderr := runTmuxOnStopReview(t, fx, tmux, rev, nil,
		Deps{ArtifactTimeoutS: 2, ArtifactMaxExtends: 5}, "--allow-bypass", "--agent=router")
	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want %d; stderr=%q", code, ExitArtifactTimeout, stderr)
	}
	summary := artifactTimeoutSummary(stderr)
	if !strings.Contains(summary, "transient=false") {
		t.Errorf("the agent's echoed task text was reported as an upstream outage — the driver is scanning "+
			"the RAW pane, not the agent-stripped one\n  got: %s", summary)
	}
}

// TestClassifyTransientPane_UnknownDriverFailsOpen: an unloadable manifest must
// report false, never a fabricated cause. The driver calls this on a failure
// path with whatever cli the launch named, including test//operator drivers that
// have no embedded manifest at all.
func TestClassifyTransientPane_UnknownDriverFailsOpen(t *testing.T) {
	if classifyTransientPane("itest-tmux", "API Error: 529 Overloaded") {
		t.Error("a driver with no manifest reported a transient cause — recognition must fail OPEN, " +
			"never invent a diagnosis for a family it knows nothing about")
	}
}
