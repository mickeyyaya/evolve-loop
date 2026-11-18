package redact_test

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/redact"
)

func TestSecrets_RedactsSecretShapesAndKeepsCleanText(t *testing.T) {
	t.Parallel()
	got := redact.Secrets("key sk-livesecret0123456789ABCDEF and api_key=hunter2hunter2 end")
	for _, secret := range []string{"sk-livesecret", "hunter2hunter2"} {
		if strings.Contains(got, secret) {
			t.Errorf("Secrets left %q in %q", secret, got)
		}
	}
	if !strings.Contains(got, redact.Token) {
		t.Errorf("Secrets must mark the redaction with %q, got %q", redact.Token, got)
	}
	if clean := "plain log line 3f786850e387550fdab836ed7e6dc881de23001b"; redact.Secrets(clean) != clean {
		t.Errorf("Secrets changed clean text")
	}
	if once := redact.Secrets("password: abc"); redact.Secrets(once) != once {
		t.Errorf("Secrets must be idempotent, got %q then %q", once, redact.Secrets(once))
	}
}

func TestEntropyRuns_FlagsRandomRunsAndExemptsHexAndFlatRuns(t *testing.T) {
	t.Parallel()
	random := "Zq8Xv3Lp0Rt7Ym2Kc9Wd4Hs6Jb1Nf5Ga8Ue3OiPx"
	if got := redact.EntropyRuns("handle " + random + " ok"); len(got) != 1 || got[0] != random {
		t.Errorf("EntropyRuns must flag the random run, got %q", got)
	}
	exempt := map[string]string{
		"commit":    "3f786850e387550fdab836ed7e6dc881de23001b",
		"digest":    "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"short":     random[:31],
		"flat":      strings.Repeat("a", 40),
		"two-alpha": strings.Repeat("ab", 20),
	}
	for name, run := range exempt {
		if got := redact.EntropyRuns("x " + run + " y"); len(got) != 0 {
			t.Errorf("%s must pass the entropy rule, got %q", name, got)
		}
	}
}
