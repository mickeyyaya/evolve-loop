package bridge

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
)

// stubResolver is a minimal non-nil TokenResolver for the wired case.
func stubResolver(tokenusage.Window) (tokenusage.Result, error) {
	return tokenusage.Result{}, nil
}

func TestEngine_WarnsOnNilTokenResolver(t *testing.T) {
	var buf bytes.Buffer
	NewEngine(Deps{Stderr: &buf, Signals: sinkDeps(&buf)})
	out := buf.String()
	if !strings.Contains(out, "WARN") || !strings.Contains(out, "TokenResolver") {
		t.Fatalf("NewEngine with nil TokenResolver emitted no WARN naming TokenResolver on Stderr; got: %q", out)
	}
	if n := strings.Count(out, "TokenResolver"); n != 1 {
		t.Fatalf("boot WARN should fire exactly once per construction, got %d mentions of TokenResolver in: %q", n, out)
	}
}

func TestEngine_NoTokenResolverWarnWhenWired(t *testing.T) {
	var buf bytes.Buffer
	NewEngine(Deps{Stderr: &buf, Signals: sinkDeps(&buf), TokenResolver: stubResolver})
	if out := buf.String(); strings.Contains(out, "TokenResolver") {
		t.Fatalf("NewEngine with a wired TokenResolver must be silent about it; got: %q", out)
	}
}
