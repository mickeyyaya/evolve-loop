package phasestream

import "testing"

const (
	zshCompinitLine = "⎿  compinit:527: operation not permitted: /opt/homebrew/share/zsh/site-functions/_docker-compose"
	verifyLockWarn  = "[acs] WARN: verification single-flight unavailable (verifylock: open /Users/x/evolve-loop/.repo.git/evolve-verify.lock: operation not permitted); running unlocked"
)

func TestClassifier_PaneNoiseIsNotASandboxDenial(t *testing.T) {
	c := newTestClassifier()
	for _, line := range []string{zshCompinitLine, "compinit:12: operation not permitted: /usr/share/zsh/functions/_git", verifyLockWarn} {
		for _, e := range c.Stderr([]byte(line)) {
			if e.Kind == KindInfraFailure {
				t.Fatalf("event %#v; every pane prints this line, so it says nothing about why the phase failed", e)
			}
		}
	}
	if out := c.Stderr([]byte("sandbox-exec: sandbox_apply: Operation not permitted")); len(out) != 1 || out[0].Kind != KindInfraFailure {
		t.Fatalf("a real sandbox denial is still infra: %#v", out)
	}
}

func TestInfraMarker_StripsTheNoiseAndKeepsARealMarkerBesideIt(t *testing.T) {
	for text, want := range map[string]string{
		zshCompinitLine: "",
		verifyLockWarn:  "",
		zshCompinitLine + "\nAPI Error 529 Overloaded":   "api_529",
		verifyLockWarn + "\nconnect: connection refused": "conn_refused",
		"sandbox-exec: deny Operation not permitted":     "eperm",
	} {
		if got := InfraMarker(text); got != want {
			t.Errorf("InfraMarker(%q) = %q, want %q", text, got, want)
		}
	}
}
