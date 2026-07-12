package bridge

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
)

// Cycle-745 task token-resolver-boot-warn: Deps.TokenResolver is a fail-open
// seam — nil silently disables token telemetry (recordTokenUsage no-ops), which
// is exactly how the all-zeros first telemetry batch shipped unnoticed. The
// remaining AC from inbox item token-resolver-production-wiring: fail-open must
// be LOUD. Constructing an Engine with a nil TokenResolver must emit a
// WARN-level line on the engine's Stderr at construction (boot) time, naming
// TokenResolver so the operator can grep for it; a wired resolver must stay
// silent (no per-boot noise on the healthy path).

// stubResolver is a minimal non-nil TokenResolver for the wired case.
func stubResolver(tokenusage.Window) (tokenusage.Result, error) {
	return tokenusage.Result{}, nil
}

// TestEngine_WarnsOnNilTokenResolver — AC1 (positive): a nil TokenResolver at
// construction produces exactly one WARN line mentioning TokenResolver on the
// engine's own Stderr, so telemetry fail-open is loud instead of silent.
func TestEngine_WarnsOnNilTokenResolver(t *testing.T) {
	var buf bytes.Buffer
	NewEngine(Deps{Stderr: &buf})
	out := buf.String()
	if !strings.Contains(out, "WARN") || !strings.Contains(out, "TokenResolver") {
		t.Fatalf("NewEngine with nil TokenResolver emitted no WARN naming TokenResolver on Stderr; got: %q", out)
	}
	if n := strings.Count(out, "TokenResolver"); n != 1 {
		t.Fatalf("boot WARN should fire exactly once per construction, got %d mentions of TokenResolver in: %q", n, out)
	}
}

// TestEngine_NoTokenResolverWarnWhenWired — AC2 (negative / anti-noise): an
// Engine constructed WITH a TokenResolver must not emit the nil-resolver WARN —
// otherwise every healthy production boot logs a false alarm and the signal
// value of AC1 is destroyed.
func TestEngine_NoTokenResolverWarnWhenWired(t *testing.T) {
	var buf bytes.Buffer
	NewEngine(Deps{Stderr: &buf, TokenResolver: stubResolver})
	if out := buf.String(); strings.Contains(out, "TokenResolver") {
		t.Fatalf("NewEngine with a wired TokenResolver must be silent about it; got: %q", out)
	}
}
