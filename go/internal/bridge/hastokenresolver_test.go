package bridge

// hastokenresolver_test.go — RED contract for cycle-623 task
// token-resolver-production-wiring (inbox
// 2026-07-08T02-10-00Z-token-resolver-production-wiring.json, weight 0.96).
//
// Deps.TokenResolver is a DI seam nothing outside this package can inspect
// today (deps is unexported) — the two production composition roots
// (adapters/bridge.Adapter, subagent.defaultExecAdapter) build a gobridge.Deps
// and have no way to prove, in a test, that the field they set actually
// reached the constructed Engine. HasTokenResolver gives both call sites (and
// their tests — see adapters/bridge/tokenresolver_wiring_test.go and
// subagent/tokenresolver_wiring_test.go) that seam. It is undefined today, so
// this file fails to compile — the intended RED signal. Builder implements:
//
//	func (e *Engine) HasTokenResolver() bool { return e.deps.TokenResolver != nil }
import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
)

// TestHasTokenResolver_TrueWhenDepsFieldSet: a non-nil TokenResolver passed
// into NewEngine must be observable via the new accessor.
func TestHasTokenResolver_TrueWhenDepsFieldSet(t *testing.T) {
	eng := NewEngine(Deps{
		TokenResolver: func(tokenusage.Window) (tokenusage.Result, error) {
			return tokenusage.Result{}, nil
		},
	})
	if !eng.HasTokenResolver() {
		t.Error("HasTokenResolver() = false, want true for a Deps with TokenResolver set")
	}
}

// TestHasTokenResolver_FalseWhenDepsFieldNil: the documented "telemetry off"
// DI state (nil TokenResolver, per Deps.TokenResolver's doc comment) must
// report false, not panic and not default to some non-nil stub.
func TestHasTokenResolver_FalseWhenDepsFieldNil(t *testing.T) {
	eng := NewEngine(Deps{})
	if eng.HasTokenResolver() {
		t.Error("HasTokenResolver() = true, want false for a Deps with no TokenResolver set")
	}
}
