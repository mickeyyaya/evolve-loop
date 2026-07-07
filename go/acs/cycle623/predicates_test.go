//go:build acs

// Package cycle623 materialises the cycle-623 acceptance criteria for the
// single triage-committed top_n task, token-resolver-production-wiring
// (weight 0.96, inbox
// 2026-07-08T02-10-00Z-token-resolver-production-wiring.json): both
// production composition roots that build gobridge.Deps — internal/adapters/
// bridge.Adapter (NewDefault's engineFactory) and internal/subagent's
// defaultExecAdapter — currently leave Deps.TokenResolver nil (confirmed via
// grep: 0 non-test hits in either file), so token telemetry has been
// silently all-zero since at least cycle 612 (fail-open masks the gap; see
// internal/bridge/engine.go:527's `if e.deps.TokenResolver == nil { return }`
// guard).
//
// Predicate strategy: behavioural-via-subprocess (the cycle-549…613
// precedent) — each predicate shells `go test -run` over the RED unit tests
// authored this cycle. None is a source-grep; every one exercises the system
// under test (tokenusage.DefaultResolver, Engine.HasTokenResolver,
// Adapter.productionEngineDeps, subagent.execAdapterDeps — each called with
// real arguments, several against real on-disk transcript fixtures) and
// asserts on its result. RED now: internal/tokenusage, internal/bridge,
// internal/adapters/bridge, and internal/subagent all fail to compile
// (DefaultResolver / HasTokenResolver / productionEngineDeps /
// execAdapterDeps all undefined). GREEN once Builder wires
// tokenusage.DefaultResolver(configRoot) into both composition roots per the
// contract documented in each RED test file's header comment.
//
// Scope: the third Acceptance Criteria Summary line ("nil-resolver path
// emits one boot WARN") is dispositioned manual+checklist in
// test-report.md, not predicated here — see that file's Coverage Map for the
// rationale (no reachable nil-resolver case remains once both composition
// roots route through tokenusage.DefaultResolver, which always returns a
// non-nil func) and the Auditor checklist.
package cycle623

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	tokenusagePkg     = "github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
	bridgePkg         = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	adaptersBridgePkg = "github.com/mickeyyaya/evolve-loop/go/internal/adapters/bridge"
	subagentPkg       = "github.com/mickeyyaya/evolve-loop/go/internal/subagent"
)

// runGoTest shells `go test -run '^(<pattern>)$' -count=1 <pkg>` and reports
// whether it exited cleanly plus the combined output. -count=1 defeats the
// test cache so the predicate always exercises current source. code<0 is a
// genuine launch failure (binary missing / killed by signal), never a test
// verdict — that must fail loudly, not be misread as RED.
func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

// TestC623_001_DefaultResolverWrapsTranscriptScanner — AC (verifiableBy,
// part 2): a resolver built by the one shared tokenusage.DefaultResolver
// helper recovers real usage from a real on-disk transcript fixture and
// reports SourceTranscript (never SourceNone/error) for a matching Window.
func TestC623_001_DefaultResolverWrapsTranscriptScanner(t *testing.T) {
	ok, out := runGoTest(t, tokenusagePkg, "TestDefaultResolver_TranscriptFixture_ReturnsTranscriptSource")
	if !ok {
		t.Errorf("tokenusage.DefaultResolver missing or does not recover real transcript usage:\n%s", out)
	}
}

// TestC623_002_DefaultResolverFailsOpenOnEmptyRoot — negative: a configRoot
// with no matching transcript must yield SourceNone + nil error, never a
// fabricated result and never a resolver error (telemetry stays best-effort).
func TestC623_002_DefaultResolverFailsOpenOnEmptyRoot(t *testing.T) {
	ok, out := runGoTest(t, tokenusagePkg, "TestDefaultResolver_EmptyConfigRoot_ReturnsSourceNoneNotError")
	if !ok {
		t.Errorf("tokenusage.DefaultResolver does not fail open on an empty configRoot:\n%s", out)
	}
}

// TestC623_003_EngineExposesTokenResolverPresence — the DI-inspection seam
// (Engine.HasTokenResolver) both composition-root wiring predicates below
// depend on: true when Deps.TokenResolver is set, false (not a panic, not a
// stub default) when it is nil.
func TestC623_003_EngineExposesTokenResolverPresence(t *testing.T) {
	ok, out := runGoTest(t, bridgePkg, "TestHasTokenResolver_TrueWhenDepsFieldSet|TestHasTokenResolver_FalseWhenDepsFieldNil")
	if !ok {
		t.Errorf("bridge.Engine.HasTokenResolver missing or incorrect:\n%s", out)
	}
}

// TestC623_004_AdaptersBridgeCompositionRootWiresResolver — AC
// (verifiableBy, part 1, EXACT test name scout-report.md names for this
// task): the production-built Engine from adapters/bridge.NewDefault's real
// engineFactory (not just the Deps-building helper in isolation) has a
// non-nil TokenResolver.
func TestC623_004_AdaptersBridgeCompositionRootWiresResolver(t *testing.T) {
	ok, out := runGoTest(t, adaptersBridgePkg,
		"TestProductionEngineDeps_WiresNonNilTokenResolver|TestEngineFactory_WiresTokenResolver")
	if !ok {
		t.Errorf("adapters/bridge.Adapter does not wire a non-nil TokenResolver into its production engineFactory:\n%s", out)
	}
}

// TestC623_005_AdaptersBridgeResolverIsGenuine — anti-gaming: the wired
// resolver isn't a disconnected stub that happens to be non-nil — invoked
// against a real HOME/.claude transcript fixture it recovers SourceTranscript,
// proving it genuinely delegates to tokenusage.DefaultResolver.
func TestC623_005_AdaptersBridgeResolverIsGenuine(t *testing.T) {
	ok, out := runGoTest(t, adaptersBridgePkg, "TestProductionEngineDeps_ResolverAppliesRealFixture")
	if !ok {
		t.Errorf("adapters/bridge's wired TokenResolver does not genuinely scan HOME/.claude (looks disconnected from tokenusage.DefaultResolver):\n%s", out)
	}
}

// TestC623_006_SubagentCompositionRootWiresResolver — AC (verifiableBy,
// part 1, the validateprofile.go half): defaultExecAdapter's Deps-building
// helper (execAdapterDeps) sets a non-nil TokenResolver, including for the
// edge case of an env map with no "HOME" key.
func TestC623_006_SubagentCompositionRootWiresResolver(t *testing.T) {
	ok, out := runGoTest(t, subagentPkg,
		"TestExecAdapterDeps_WiresNonNilTokenResolver|TestExecAdapterDeps_MissingHome_StillReturnsNonNilResolver")
	if !ok {
		t.Errorf("subagent.execAdapterDeps does not wire a non-nil TokenResolver (including the missing-HOME edge case):\n%s", out)
	}
}

// TestC623_007_SubagentResolverIsGenuine — anti-gaming counterpart to 005 for
// the subagent composition root.
func TestC623_007_SubagentResolverIsGenuine(t *testing.T) {
	ok, out := runGoTest(t, subagentPkg, "TestExecAdapterDeps_ResolverAppliesRealFixture")
	if !ok {
		t.Errorf("subagent's wired TokenResolver does not genuinely scan HOME/.claude (looks disconnected from tokenusage.DefaultResolver):\n%s", out)
	}
}
