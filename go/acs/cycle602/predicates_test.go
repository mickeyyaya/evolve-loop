//go:build acs

// Package cycle602 materialises the cycle-602 acceptance criteria for the one
// triage-committed top_n task (see triage-report.md):
//
//   - token-telemetry-s3-engine-launch-instrumentation (inbox 0.93): the
//     Engine.Launch chokepoint (go/internal/bridge/engine.go:335) attributes
//     every LLM invocation's token cost. On each Launch it (1) populates
//     core.BridgeResponse.Tokens from an injected Deps.TokenResolver, (2)
//     appends exactly one record to <Workspace>/llm-calls.ndjson per Launch
//     attempt — so a fallback retry on a different CLI is its own record, making
//     double-dispatch waste measurable — and (3) is fail-open: a resolver error
//     WARNs to Deps.Stderr, leaves resp.Tokens zero, and NEVER turns an
//     otherwise-successful Launch into an error.
//
// Predicate strategy: behavioural-via-subprocess (the cycle-549…596 precedent).
// Each predicate shells `go test -run` over the named acceptance tests in
// go/internal/bridge/engine_launch_tokens_test.go — every one drives the real
// Engine.Launch pipeline (a live *Engine with an injected TokenResolver + a
// fakeRunner), asserting on resp.Tokens, the on-disk llm-calls.ndjson records,
// and the Launch error path. None is a source grep, so none passes on an
// EMPTY repo: with the feature absent the bridge package fails to compile
// (Deps.TokenResolver / BridgeRequest.Attempt undefined) and every `go test`
// below exits non-zero.
//
// State note: the S3 implementation and these named tests landed in this cycle's
// own goal-hash commit (dca6398c) — this is a resumed run — so the predicates
// are GREEN today (documented as pre-existing GREEN in test-report.md). They
// remain the audit-gating contract: any regression that breaks token attribution
// re-REDs them.
package cycle602

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const bridgePkg = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"

// runGoTest shells `go test -run '^(<pattern>)$' -count=1 <pkg>` and reports
// whether it exited cleanly plus the combined output. -count=1 defeats the test
// cache so the predicate always exercises current source. A compile failure or
// assertion failure in the target package surfaces as a non-zero exit — the RED
// signal a regression (or the not-yet-built feature) produces.
func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	// code < 0 is a genuine launch failure (binary missing / killed by signal),
	// not a test verdict; SubprocessOutput returns non-nil err for ANY non-zero
	// exit, so a plain compile/assertion failure (code 1/2) must flow through as
	// ok=false, NOT be misread as "failed to launch".
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

// TestC602_001_LaunchPopulatesBridgeResponseTokens — AC-1: a Launch whose
// injected Deps.TokenResolver resolves non-empty usage surfaces that usage on
// core.BridgeResponse.Tokens (the field existed but Launch never populated it).
// Drives the named acceptance test, which builds a real *Engine and asserts on
// resp.Tokens — not a source grep, so it fails on an EMPTY repo (no TokenResolver
// seam ⇒ bridge package does not compile).
func TestC602_001_LaunchPopulatesBridgeResponseTokens(t *testing.T) {
	ok, out := runGoTest(t, bridgePkg, "TestEngineLaunch_PopulatesBridgeResponseTokens")
	if !ok {
		t.Errorf("Engine.Launch does not populate core.BridgeResponse.Tokens from the injected TokenResolver:\n%s", out)
	}
}

// TestC602_002_LaunchAppendsOneRecordPerAttempt — AC-2 + anti-no-op: two Launch
// calls against the same Workspace with distinct BridgeRequest.Attempt values
// (a fallback retry on a different CLI) append TWO records to llm-calls.ndjson,
// each carrying its own attempt/cli — never overwriting a shared single record.
// This is the strongest anti-no-op signal for this task: a degenerate impl that
// truncates/overwrites the file, or writes one shared record, fails the
// "exactly 2 lines with attempt=1/cli=claude-p then attempt=2/cli=codex"
// assertion. Making double-dispatch waste measurable is the whole point of S3.
func TestC602_002_LaunchAppendsOneRecordPerAttempt(t *testing.T) {
	ok, out := runGoTest(t, bridgePkg, "TestEngineLaunch_AppendsLLMCallRecordPerFallbackAttempt")
	if !ok {
		t.Errorf("Engine.Launch does not append exactly one llm-calls.ndjson record per fallback attempt:\n%s", out)
	}
}

// TestC602_003_ResolverErrorNeverFailsLaunch — AC-3 negative / fail-open: a
// TokenResolver that returns an error must WARN to Deps.Stderr and leave
// resp.Tokens at its zero value, but must NEVER turn an otherwise-successful
// Launch (ExitOK) into an error. Telemetry is best-effort and must not gate the
// pipeline. Drives the named fail-open acceptance test, which asserts err==nil,
// ExitOK, zero Tokens, and the WARN reaching Stderr.
func TestC602_003_ResolverErrorNeverFailsLaunch(t *testing.T) {
	ok, out := runGoTest(t, bridgePkg, "TestEngineLaunch_CollectorErrorNeverFailsLaunch")
	if !ok {
		t.Errorf("a TokenResolver error is not handled fail-open (must WARN, zero Tokens, never fail the Launch):\n%s", out)
	}
}
