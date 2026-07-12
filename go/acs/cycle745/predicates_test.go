//go:build acs

// Package cycle745 materializes the cycle-745 acceptance criteria for the sole
// triage-committed top_n task token-resolver-boot-warn (triage-report.md
// ## top_n; the two other scout selections were dropped as out-of-fleet-scope,
// so per R9.3 no predicates bind to them and no deferred-floor predicates
// exist).
//
// AC map (1:1), derived from the top_n task text ("add boot-time WARN when
// Deps.TokenResolver is nil at construction") plus the scout AC summary
// ("regression test asserts this"):
//
//	AC1 nil TokenResolver at Engine construction ⇒ exactly one
//	    WARN line naming TokenResolver on the engine Stderr        → C745_001
//	AC2 wired TokenResolver ⇒ NO such WARN (negative / anti-noise) → C745_002
//	AC3 both production composition roots keep wiring a non-nil
//	    resolver, so the WARN never fires on a healthy boot
//	    (regression pin; pre-existing GREEN)                       → C745_003
//
// Each predicate shells `go test -race -count=1 -v -run '^<name>$'` over the
// unit-test contract, which EXERCISES the SUT (NewEngine against an injected
// Stderr buffer / the real composition-root constructors) — behavioral via
// subprocess, no source-grep predicates (cycle-85 rule). The `-v` +
// "--- PASS:" guard rejects a rename/no-tests-matched silent green.
package cycle745

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	bridgePkg         = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	adaptersBridgePkg = "github.com/mickeyyaya/evolve-loop/go/internal/adapters/bridge"
	subagentPkg       = "github.com/mickeyyaya/evolve-loop/go/internal/subagent"
)

// runGoTest executes the named unit test under -race and requires an explicit
// verbose PASS marker so the predicate fails on: compile failure, test
// failure, a race report, OR the test not existing (rename gaming).
func runGoTest(t *testing.T, pkg, name string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-race", "-count=1", "-v", "-run", "^"+name+"$", pkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -race %s -run %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			pkg, name, code, err, stdout, stderr)
	}
	if !strings.Contains(stdout, "--- PASS: "+name) {
		t.Fatalf("go test reported no PASS for %s (renamed or not run?)\nstdout:\n%s", name, stdout)
	}
}

// AC1 — the incident twin: constructing an Engine with a nil TokenResolver
// emits exactly one WARN line naming TokenResolver on the injected Stderr, so
// telemetry fail-open (the all-zeros first-batch incident) is loud, not silent.
func TestC745_001_EngineWarnsOnNilTokenResolver(t *testing.T) {
	runGoTest(t, bridgePkg, "TestEngine_WarnsOnNilTokenResolver")
}

// AC2 — the negative: an Engine constructed WITH a resolver stays silent about
// TokenResolver; a WARN on the healthy path would be a per-boot false alarm.
func TestC745_002_NoWarnWhenResolverWired(t *testing.T) {
	runGoTest(t, bridgePkg, "TestEngine_NoTokenResolverWarnWhenWired")
}

// AC3 — the regression pin (pre-existing GREEN): both production composition
// roots (adapters/bridge and subagent) still wire a non-nil resolver via
// tokenusage.DefaultResolver, so the new boot WARN never fires in production.
func TestC745_003_CompositionRootsWireResolver(t *testing.T) {
	runGoTest(t, adaptersBridgePkg, "TestProductionEngineDeps_WiresNonNilTokenResolver")
	runGoTest(t, subagentPkg, "TestExecAdapterDeps_WiresNonNilTokenResolver")
}
