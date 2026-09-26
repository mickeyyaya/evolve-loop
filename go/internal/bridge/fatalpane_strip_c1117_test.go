package bridge

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
)

func c1117Ev(tail, injectedPrompt string) StopEvent {
	return StopEvent{
		Kind: StopArtifactTimeout, Phase: "build", Cycle: 1117,
		ElapsedS: 300, IntervalS: 300, Attempt: 0,
		Progressed:     true, // the nudge-echo trap: a dead pane CAN read as progressed
		Busy:           false,
		StdoutTail:     tail,
		InjectedPrompt: injectedPrompt,
	}
}

// c1117EchoLine carries no fatal signature of its own — it is the plain
// instruction echo the stripper must neutralize.
const c1117EchoLine = "Write the report to workspace/build-report.md when you are done."

// c1117AnchoredSeeds are newline-anchored so a bare word cannot false-positive;
// line position is load-bearing for every one of them.
var c1117AnchoredSeeds = []string{"\nquote>", "\nbquote>", "\ndquote>", "\nheredoc>"}

func TestC1117_AnchoredSeedSurvivesEchoStripping(t *testing.T) {
	t.Parallel()
	det := recovery.SeedDetector()
	for _, seed := range c1117AnchoredSeeds {
		token := strings.TrimPrefix(seed, "\n")
		for _, tc := range []struct {
			name   string
			prompt string
		}{
			{"prompt echoes an ordinary line", c1117EchoLine},
			{"prompt also quotes the continuation token", c1117EchoLine + "\nWatch for a " + token + " spill."},
		} {
			pane := c1117EchoLine + "\n" + token + "\n"
			if _, _, ok := det.Detect(pane); !ok {
				t.Fatalf("%s: raw pane does not detect — fixture is wrong, not the code", token)
			}

			stripped := strippedForFatalPaneScan(pane, tc.prompt, det.Signatures())
			if got, want := strings.Count(stripped, "\n"), strings.Count(pane, "\n"); got != want {
				t.Errorf("%s / %s: stripped pane has %d newlines, want %d — lines were DELETED, not blanked; every anchored seed below the cut loses its leading newline (D1)",
					token, tc.name, got, want)
			}
			if !strings.Contains(stripped, seed) {
				t.Errorf("%s / %s: anchored seed %q lost from the stripped pane (%q) — the cycle-274 dead-shell fast-fail is reverted for prompt-spill panes (D1)",
					token, tc.name, seed, stripped)
			}

			var buf bytes.Buffer
			v, preempted := fatalPaneVerdict(det, c1117Ev(pane, tc.prompt), "enforce", nil, &buf, "[c1117]")
			if !preempted {
				t.Errorf("%s / %s: enforce did not preempt a dead shell — the phase burns the full maxExtends backstop on a REPL that no longer exists", token, tc.name)
				continue
			}
			if v.Action != ReviewStop {
				t.Errorf("%s / %s: action=%s, want stop", token, tc.name, v.Action)
			}
		}
	}
}

func TestC1117_PromptQuotingSeedDoesNotSuppressBanner(t *testing.T) {
	t.Parallel()
	det := recovery.SeedDetector()
	pane := "⏺ booting\n" + fatalTail + "\n"
	prompt := "Task: harden the fatal-pane registry.\n" + fatalTail + "\nAuthor the predicate."

	stripped := strippedForFatalPaneScan(pane, prompt, det.Signatures())
	if !strings.Contains(stripped, "There's an issue with the selected model") {
		t.Errorf("the CLI's real banner was stripped because the prompt quotes it — a prompt that mentions a fatal signature must never suppress that signature on-pane (D2); stripped=%q", stripped)
	}

	var buf bytes.Buffer
	v, preempted := fatalPaneVerdict(det, c1117Ev(pane, prompt), "enforce", nil, &buf, "[c1117]")
	if !preempted {
		t.Fatal("enforce did not preempt a self-describing model-invalid boot because the prompt quoted the banner (D2) — cycle-262's 40-minute burn returns")
	}
	if !strings.Contains(v.Reason, string(recovery.CauseModelInvalid)) {
		t.Errorf("reason must carry the typed cause for the justification trail; got %q", v.Reason)
	}
}

func TestC1117_AgentDiffSeedTextDoesNotFastFail(t *testing.T) {
	t.Parallel()
	det := recovery.SeedDetector()
	pane := strings.Join([]string{
		"  editing go/internal/recovery/detector.go",
		"   223 +\t\t{",
		"   224 +\t\t\tSubstr: \"There's an issue with the selected model\",",
		"   225 +\t\t\tCause:  CauseModelInvalid,",
		"  ⏺ Working… (esc to interrupt)",
	}, "\n")
	if _, _, ok := det.Detect(pane); !ok {
		t.Fatal("raw pane does not detect — fixture is wrong; this test only means something if the RAW pane would fast-fail")
	}

	// an empty prompt exercises the diff half alone; the echo half fails open.
	stripped := strippedForFatalPaneScan(pane, "", det.Signatures())
	if _, _, ok := det.Detect(stripped); ok {
		t.Errorf("seed text on an agent DIFF line still detects after stripping — the fatal-pane path is reading agent-authored content as CLI chrome; stripped=%q", stripped)
	}
	if got, want := strings.Count(stripped, "\n"), strings.Count(pane, "\n"); got != want {
		t.Errorf("stripped pane has %d newlines, want %d — diff lines must be BLANKED in place too, or an anchored seed below them loses its leading newline (D1)", got, want)
	}

	var buf bytes.Buffer
	if _, preempted := fatalPaneVerdict(det, c1117Ev(pane, ""), "enforce", nil, &buf, "[c1117]"); preempted {
		t.Fatal("enforce fast-failed a WORKING agent on its own edit buffer — this is the false-FAIL the task exists to remove, and the asymmetry with strippedForExhaustionScan")
	}
}

func TestC1117_StopEventCarriesInjectedPromptFromDriver(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	tmux := &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}} // boots; artifact never appears
	rev := &scriptedReviewer{}

	code, _ := runTmuxRev(t, fx, tmux, rev, Deps{ArtifactTimeoutS: 2}, "--allow-bypass")
	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want ExitArtifactTimeout", code)
	}
	if len(rev.events) == 0 {
		t.Fatal("reviewer never consulted — no checkpoint ran")
	}
	got := rev.events[0].InjectedPrompt
	if got == "" {
		t.Fatal("StopEvent.InjectedPrompt is empty at the real checkpoint — the driver never threads the resolved prompt in, leaving the echo half of the fatal-pane strip inert in production (the cycle-1115 gap)")
	}
	if !strings.Contains(got, fx.token) {
		t.Errorf("StopEvent.InjectedPrompt = %q, want the resolved prompt containing %q — some other string is being threaded through", got, fx.token)
	}
}

func TestC1117_SignaturesAccessorMatchesRegistry(t *testing.T) {
	t.Parallel()
	det := recovery.SeedDetector()
	sigs := det.Signatures()
	if len(sigs) == 0 {
		t.Fatal("Signatures() returned nothing — the protect-list would be empty and D2 unfixed")
	}
	for _, s := range sigs {
		if _, _, ok := det.Detect("preamble" + s + "\ntail"); !ok {
			t.Errorf("Signatures() returned %q but Detect does not fire on it — the accessor is out of sync with the registry it reports", s)
		}
	}
	for _, seed := range c1117AnchoredSeeds {
		found := false
		for _, s := range sigs {
			if s == seed {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("anchored seed %q missing from Signatures() — it would be unprotected against a prompt that quotes it", seed)
		}
	}
	det.Promote(recovery.FatalSignature{Substr: "c1117 promoted marker", Cause: recovery.CauseDeadShell, Note: "c1117 accessor liveness"})
	live := det.Signatures()
	if len(live) <= len(sigs) {
		t.Errorf("Signatures() returned %d entries after Promote, was %d — the accessor snapshots instead of reporting the live registry, so promoted signatures go unprotected", len(live), len(sigs))
	}
}
