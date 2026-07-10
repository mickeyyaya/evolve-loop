package phasestream

// RED wiring test for cycle-672 top_n task `echo-veto-wiring-completion`
// (third cycle on this defect class: 654 landed the leaf helpers, 656's
// wiring attempt was quota-killed, 672 completes the consumption wiring).
//
// TestC654_003 already proves Classifier.SetInjectedPrompt works when called
// directly — but grep shows ZERO production call sites: Produce() constructs
// the Classifier (produce.go) and never threads the phase prompt in, so the
// live emit path still classifies echoed prompt text as infra_failure
// (cycle-656 retro D3: a 100%-echoed pane classified rate_limit).
//
// This test pins the WIRING: ProduceConfig must carry the injected prompt and
// Produce must hand it to the Classifier before classifying. RED today:
// ProduceConfig has no InjectedPrompt field — compile failure. DO NOT MODIFY;
// Builder adds the field + the SetInjectedPrompt call to make it GREEN.

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// readEventEnvelopes parses <ws>/<phase>-events.ndjson written by Produce.
func readEventEnvelopes(t *testing.T, ws, phase string) []Envelope {
	t.Helper()
	f, err := os.Open(filepath.Join(ws, phase+"-events.ndjson"))
	if err != nil {
		t.Fatalf("open events file: %v", err)
	}
	defer func() { _ = f.Close() }()
	var envs []Envelope
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<10), 1<<22)
	for sc.Scan() {
		var e Envelope
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			t.Fatalf("unmarshal envelope %q: %v", sc.Text(), err)
		}
		envs = append(envs, e)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan events file: %v", err)
	}
	return envs
}

// TestC672_001_ProduceInjectedPromptEchoVeto — AC1 (mechanism wiring): Produce
// over an on-disk workspace whose stderr log echoes a line of the injected
// prompt must emit NO infra_failure for it; a genuine provider error frame in
// an otherwise-identical workspace must still emit (negative/anti-no-op axis).
func TestC672_001_ProduceInjectedPromptEchoVeto(t *testing.T) {
	const prompt = "Adversarial Reviewer checklist: unbounded allocation or recursion; " +
		"TOCTOU / race windows; missing rate limits. Report exploits only."

	cases := []struct {
		name       string
		stderrLine string
		wantInfra  bool
	}{
		{"echoed prompt line is suppressed", "missing rate limits.", false},
		{"genuine 429 frame still emits", "Error: 429 Too Many Requests (rate limit hit)", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ws := t.TempDir()
			if err := os.WriteFile(filepath.Join(ws, "review-stdout.log"), []byte("agent transcript line\n"), 0o644); err != nil {
				t.Fatalf("write stdout log: %v", err)
			}
			if err := os.WriteFile(filepath.Join(ws, "review-stderr.log"), []byte(tc.stderrLine+"\n"), 0o644); err != nil {
				t.Fatalf("write stderr log: %v", err)
			}

			err := Produce(ProduceConfig{
				Workspace:      ws,
				Phase:          "review",
				CLI:            "claude",
				Cycle:          672,
				InjectedPrompt: prompt, // RED: field does not exist yet
			})
			if err != nil {
				t.Fatalf("Produce: %v", err)
			}

			if got := hasInfraFailure(readEventEnvelopes(t, ws, "review")); got != tc.wantInfra {
				t.Errorf("infra_failure emitted = %v, want %v (stderr line %q, prompt-echo veto wiring)", got, tc.wantInfra, tc.stderrLine)
			}
		})
	}
}
