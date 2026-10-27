//go:build acs

package cycle2050

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	amplification = "test-amplification"
	mutationGate  = "mutation-gate"
)

func reportWithSignals(sections, signals string) string {
	return sections + "\n<!-- evolve-signals: " + signals + " -->\n" +
		`<!-- evolve-verdict: {"phase":"x","verdict":"PASS","schema_version":1} -->` + "\n"
}

func amplificationReport(signals string) string {
	return reportWithSignals("## Generated Tests\n- `go/internal/x/x_test.go`: TestEdge\n\n## Results\nok\n", signals)
}

func catalogOf(t *testing.T, root string) phasespec.Catalog {
	t.Helper()
	cat, _, _, err := phasespec.MergedCatalog(root)
	if err != nil {
		t.Fatalf("load the production phase catalog: %v", err)
	}
	return cat
}

func reportName(t *testing.T, cat phasespec.Catalog, phase string) string {
	t.Helper()
	spec, ok := cat.Get(phase)
	if !ok || len(spec.Outputs.Files) == 0 {
		t.Fatalf("precondition: %s has no declared output file (found=%v)", phase, ok)
	}
	return filepath.Base(spec.Outputs.Files[0])
}

func completePhase(t *testing.T, cat phasespec.Catalog, phase, report string) string {
	t.Helper()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, reportName(t, cat, phase)), []byte(report), 0o644); err != nil {
		t.Fatal(err)
	}
	in := core.ReviewInput{Phase: phase, Cycle: 7, Workspace: ws, ProjectRoot: t.TempDir()}
	claimNothing := func(string, int, []string) error { return nil }
	if err := deliverable.NewHostEffects(cat, claimNothing).Perform(context.Background(), in); err != nil {
		t.Logf("Perform(%s): %v", phase, err)
	}
	return ws
}

func digestAfter(t *testing.T, ws string, completed ...string) router.RoutingSignals {
	t.Helper()
	sig, err := router.Digest(ws, completed)
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	return sig
}

func productionPolicy(t *testing.T, root string) router.PhasePolicy {
	t.Helper()
	registry := config.RegistryPath(root)
	cfg, _ := config.New().Load(registry, nil)
	builtin, err := phasespec.Load(registry)
	if err != nil {
		t.Fatalf("load the builtin registry: %v", err)
	}
	user, _, _ := phasespec.DiscoverUserSpecsFromRoots(phasespec.Roots(root))
	phasespec.ApplyUserRouting(&cfg, user, builtin)
	if len(cfg.Triggers[mutationGate].InsertWhen) == 0 {
		t.Fatalf("precondition: %s has no insert_when trigger in the production routing config", mutationGate)
	}
	return router.NewPhasePolicy(cfg)
}

func TestC2050_001_AmplificationDeclaredSignalsReachTheRoutingDigest(t *testing.T) {
	root := acsassert.RepoRoot(t)
	cat := catalogOf(t, root)
	ws := completePhase(t, cat, amplification, amplificationReport(`{"amplify.tests_added":3,"amplify.failures_found":0}`))
	sig := digestAfter(t, ws, "scout", amplification)
	if v, ok := sig.GenericValue("amplify.tests_added"); !ok || v != float64(3) {
		t.Errorf("amplify.tests_added = %v (present=%v), want 3 from the report; degraded=%v", v, ok, sig.DigestDegraded)
	}
	if v, ok := sig.GenericValue("amplify.failures_found"); !ok || v != float64(0) {
		t.Errorf("amplify.failures_found = %v (present=%v), want 0 from the report", v, ok)
	}
}

func TestC2050_002_MutationGateIsAdmissibleAfterAnAmplificationRunThatAddedTests(t *testing.T) {
	root := acsassert.RepoRoot(t)
	cat := catalogOf(t, root)
	policy := productionPolicy(t, root)
	ws := completePhase(t, cat, amplification, amplificationReport(`{"amplify.tests_added":3,"amplify.failures_found":0}`))
	sig := digestAfter(t, ws, "scout", "build", amplification)
	if !policy.Enabled(mutationGate, sig) {
		t.Errorf("%s is not admissible after %s added 3 tests; generic=%v degraded=%v", mutationGate, amplification, sig.Generic, sig.DigestDegraded)
	}
}

func TestC2050_003_MutationGateIsNotAdmissibleWhenAmplificationAddedNoTests(t *testing.T) {
	root := acsassert.RepoRoot(t)
	cat := catalogOf(t, root)
	policy := productionPolicy(t, root)
	ws := completePhase(t, cat, amplification, amplificationReport(`{"amplify.tests_added":0,"amplify.failures_found":0}`))
	sig := digestAfter(t, ws, "scout", "build", amplification)
	if v, ok := sig.GenericValue("amplify.tests_added"); !ok || v != float64(0) {
		t.Fatalf("amplify.tests_added = %v (present=%v), want the reported 0 in the digest", v, ok)
	}
	if policy.Enabled(mutationGate, sig) {
		t.Errorf("%s is admissible although %s added no tests", mutationGate, amplification)
	}
}

func TestC2050_004_ASignalTheSpecDoesNotDeclareIsNotWritten(t *testing.T) {
	root := acsassert.RepoRoot(t)
	cat := catalogOf(t, root)
	ws := completePhase(t, cat, amplification, amplificationReport(`{"amplify.tests_added":3,"mutation.score":99}`))
	sig := digestAfter(t, ws, "scout", amplification)
	if v, ok := sig.GenericValue("amplify.tests_added"); !ok || v != float64(3) {
		t.Fatalf("amplify.tests_added = %v (present=%v), want the declared signal written", v, ok)
	}
	if v, ok := sig.GenericValue("mutation.score"); ok {
		t.Errorf("mutation.score = %v reached the digest from %s, which does not declare it", v, amplification)
	}
}

func TestC2050_005_TheWriterIsGenericOverEveryNonSpinePhaseThatDeclaresSignals(t *testing.T) {
	root := acsassert.RepoRoot(t)
	cat := catalogOf(t, root)
	report := reportWithSignals("## Summary\nok\n\n## Survivors\nnone\n\n## Verdict\nPASS\n", `{"mutation.score":80,"mutation.survivors":2}`)
	ws := completePhase(t, cat, mutationGate, report)
	sig := digestAfter(t, ws, "scout", mutationGate)
	if v, ok := sig.GenericValue("mutation.score"); !ok || v != float64(80) {
		t.Errorf("mutation.score = %v (present=%v), want 80 from the %s report", v, ok, mutationGate)
	}
	if v, ok := sig.GenericValue("mutation.survivors"); !ok || v != float64(2) {
		t.Errorf("mutation.survivors = %v (present=%v), want 2 from the %s report", v, ok, mutationGate)
	}
}

func TestC2050_006_AReportWithoutASignalsSentinelYieldsNoSignals(t *testing.T) {
	root := acsassert.RepoRoot(t)
	cat := catalogOf(t, root)
	ws := completePhase(t, cat, amplification, "## Generated Tests\nnone\n\n## Results\nok\n")
	sig := digestAfter(t, ws, "scout", amplification)
	if v, ok := sig.GenericValue("amplify.tests_added"); ok {
		t.Errorf("amplify.tests_added = %v was invented for a report that carries no evolve-signals sentinel", v)
	}
	policy := productionPolicy(t, root)
	if policy.Enabled(mutationGate, sig) {
		t.Errorf("%s is admissible although %s reported no signals", mutationGate, amplification)
	}
}

func TestC2050_007_AMalformedSignalsSentinelYieldsNoSignals(t *testing.T) {
	root := acsassert.RepoRoot(t)
	cat := catalogOf(t, root)
	ws := completePhase(t, cat, amplification, amplificationReport(`{"amplify.tests_added":`))
	sig := digestAfter(t, ws, "scout", amplification)
	if v, ok := sig.GenericValue("amplify.tests_added"); ok {
		t.Errorf("amplify.tests_added = %v was produced from a malformed evolve-signals payload", v)
	}
}

func TestC2050_008_APhaseThatDeclaresNoSignalsGetsNoHandoffFile(t *testing.T) {
	root := acsassert.RepoRoot(t)
	cat := catalogOf(t, root)
	spec, ok := cat.Get("memo")
	if !ok || len(spec.Outputs.Signals) != 0 {
		t.Fatalf("precondition: memo must declare no outputs.signals, got %v (found=%v)", spec.Outputs.Signals, ok)
	}
	ws := completePhase(t, cat, "memo", reportWithSignals("## Memo\nok\n", `{"memo.count":4}`))
	if _, err := os.Stat(filepath.Join(ws, "handoff-memo.json")); !os.IsNotExist(err) {
		t.Errorf("handoff-memo.json exists for a phase that declares no signals (stat err=%v)", err)
	}
	if v, ok := digestAfter(t, ws, "scout", "memo").GenericValue("memo.count"); ok {
		t.Errorf("memo.count = %v reached the digest from a phase that declares no signals", v)
	}
}
