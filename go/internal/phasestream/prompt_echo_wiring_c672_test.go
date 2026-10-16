package phasestream

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

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
				InjectedPrompt: prompt,
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
