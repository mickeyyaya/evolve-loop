//go:build acs

package cycle2328

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/convergence"
	_ "github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var convergenceCodes = []signalcenter.Code{
	"CONVERGENCE_RUNG",
	"CONVERGENCE_NO_PROGRESS",
	"CONVERGENCE_REPAIR_DAMAGE",
	"CONVERGENCE_CONCENTRATION",
	"CONVERGENCE_DEFERRED",
	"CONVERGENCE_FILED",
	"CONVERGENCE_SPLIT",
	"CONVERGENCE_ACCEPTED_LIMITS",
	"CONVERGENCE_STOP",
	"CONVERGENCE_DISPUTED",
	"CONVERGENCE_NO_HEADROOM",
}

func moduleDir(t *testing.T) string {
	t.Helper()
	dir, err := acsassert.ModuleDir(filepath.Join(acsassert.RepoRoot(t), "go"))
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func goTest(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, errOut, code, err := acsassert.SubprocessOutput("go", append([]string{"-C", dir, "test", "-count=1"}, args...)...)
	if err != nil || code != 0 {
		t.Fatalf("RED: go test %s exit=%d err=%v\n%s", strings.Join(args, " "), code, err, acsassert.OutputTail(out+errOut, 30))
	}
	return out
}

func requirePass(t *testing.T, out string, names ...string) {
	t.Helper()
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name+" ") {
			t.Errorf("RED: %s did not run and pass", name)
		}
	}
}

func buildEvolve(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "evolve")
	out, errOut, code, err := acsassert.SubprocessOutput("go", "-C", moduleDir(t), "build", "-o", bin, "./cmd/evolve")
	if err != nil || code != 0 {
		t.Fatalf("RED: go build ./cmd/evolve exit=%d err=%v\n%s", code, err, acsassert.OutputTail(out+errOut, 30))
	}
	return bin
}

func roundsInput(loop convergence.Loop, rounds [][]convergence.Finding) convergence.Input {
	in := convergence.Input{Loop: loop, Round: len(rounds) - 1}
	for i, findings := range rounds {
		in.Rounds = append(in.Rounds, convergence.Judgment{Index: i, Findings: findings})
	}
	return in
}

func roundsFile(t *testing.T, name string, loop convergence.Loop, rounds ...[]convergence.Finding) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	raw, err := json.Marshal(roundsInput(loop, rounds))
	if err == nil {
		err = os.WriteFile(path, raw, 0o644)
	}
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func finding(id string, sev convergence.Severity, status convergence.Status, falsification convergence.Falsification) convergence.Finding {
	return convergence.Finding{ID: id, Severity: sev, Status: status, Late: falsification != "", Falsification: falsification}
}

func metricsFixtures(t *testing.T) (string, string) {
	t.Helper()
	open, deferred, fixed, filed := convergence.StatusOpen, convergence.StatusDeferred, convergence.StatusFixed, convergence.StatusFiled
	med, high := convergence.SeverityMedium, convergence.SeverityHigh
	survived, refuted := convergence.FalsificationSurvived, convergence.FalsificationRefuted
	three := roundsFile(t, "three.json", convergence.LoopConsoleLane,
		[]convergence.Finding{finding("d1", med, open, ""), finding("d2", med, open, ""), finding("h1", high, open, "")},
		[]convergence.Finding{finding("d1", med, deferred, ""), finding("d2", med, deferred, ""), finding("h1", high, open, survived)},
		[]convergence.Finding{finding("d1", med, fixed, ""), finding("d2", med, deferred, ""), finding("h1", high, fixed, survived)},
	)
	two := roundsFile(t, "two.json", convergence.LoopConsoleLane,
		[]convergence.Finding{finding("x", high, open, ""), finding("y", high, open, ""), finding("z", high, open, "")},
		[]convergence.Finding{finding("x", high, filed, refuted), finding("y", high, filed, refuted), finding("z", high, open, survived)},
	)
	return three, two
}

func coverFunc(t *testing.T, dir, pkg, run string) string {
	t.Helper()
	profile := filepath.Join(t.TempDir(), "cover.out")
	goTest(t, dir, "-run", run, "-coverprofile="+profile, pkg)
	out, errOut, code, err := acsassert.SubprocessOutput("go", "-C", dir, "tool", "cover", "-func="+profile)
	if err != nil || code != 0 {
		t.Fatalf("RED: go tool cover exit=%d err=%v\n%s", code, err, acsassert.OutputTail(out+errOut, 20))
	}
	return out
}

func funcsOfFile(coverFuncOutput, fileSuffix string) map[string]string {
	funcs := map[string]string{}
	for _, line := range strings.Split(coverFuncOutput, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 3 && strings.Contains(fields[0], fileSuffix+":") {
			funcs[fields[1]] = fields[2]
		}
	}
	return funcs
}

func TestC2328_001_TheConvergenceCodesAreRegisteredToTheConvergenceModule(t *testing.T) {
	for _, c := range convergenceCodes {
		owner, ok := signalcenter.IsRegistered(c)
		if !ok || owner != signalcenter.ModuleConvergence {
			t.Errorf("RED: %s owner=%q registered=%v, want module %q", c, owner, ok, signalcenter.ModuleConvergence)
		}
	}
	if conflicts := signalcenter.RegistryConflicts(); len(conflicts) != 0 {
		t.Errorf("RED: registry conflicts: %+v", conflicts)
	}
	out := goTest(t, moduleDir(t), "-v", "-run", "^TestRegistry_Convergence", "./internal/signalcenter")
	requirePass(t, out,
		"TestRegistry_ConvergenceCodesAreOwnedByTheConvergenceModuleWithADoc",
		"TestRegistry_ConvergenceDocsNameTheFieldsOfTheSpecTable",
		"TestRegistry_ConvergenceCodesAddNoConflict")
}

func TestC2328_002_TheSignalCodesDocIsRegeneratedAndItsGeneratorCheckPasses(t *testing.T) {
	doc, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), "docs", "architecture", "signal-codes.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range convergenceCodes {
		if !strings.Contains(string(doc), "| `"+string(c)+"` |") {
			t.Errorf("RED: signal-codes.md has no table row for %s", c)
		}
	}
	root := acsassert.RepoRoot(t)
	out, errOut, code, runErr := acsassert.SubprocessOutput("env", "EVOLVE_PROJECT_ROOT="+root, buildEvolve(t), "signals", "codes", "check")
	if runErr != nil || code != 0 {
		t.Errorf("RED: evolve signals codes check exit=%d err=%v\n%s", code, runErr, acsassert.OutputTail(out+errOut, 10))
	}
}

func TestC2328_003_TheMetricsVerbPrintsTheThreeReadoutsAsTextAndJSON(t *testing.T) {
	bin := buildEvolve(t)
	three, two := metricsFixtures(t)

	text, errText, code, err := acsassert.SubprocessOutput(bin, "convergence", "metrics", three, two)
	if err != nil || code != 0 {
		t.Fatalf("RED: convergence metrics exit=%d err=%v\n%s", code, err, errText)
	}
	for _, want := range []string{
		"console-lane: median 1.5, max 2, runs 2",
		"deferred then closed: 1 of 2 (0.50)",
		"falsification drop rate: 2 of 4 (0.50)",
	} {
		if !strings.Contains(text, want+"\n") {
			t.Errorf("RED: text output lacks %q:\n%s", want, text)
		}
	}

	raw, errJSON, code, err := acsassert.SubprocessOutput(bin, "convergence", "metrics", "--json", three, two)
	if err != nil || code != 0 {
		t.Fatalf("RED: convergence metrics --json exit=%d err=%v\n%s", code, err, errJSON)
	}
	var got convergence.Metrics
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("RED: --json output is not the metrics document: %v\n%s", err, raw)
	}
	if got.DeferredClosed != (convergence.DeferredClosed{Deferred: 2, Closed: 1, Share: 0.5}) ||
		got.Falsification != (convergence.FalsificationDrop{Checked: 4, Refuted: 2, Rate: 0.5}) ||
		len(got.RoundsToLand) != 1 || got.RoundsToLand[0].Median != 1.5 || got.RoundsToLand[0].Max != 2 {
		t.Errorf("RED: --json metrics = %+v", got)
	}
}

func TestC2328_004_TheMetricsVerbRefusesABadInvocation(t *testing.T) {
	bin := buildEvolve(t)
	three, _ := metricsFixtures(t)
	missing := filepath.Join(t.TempDir(), "absent.json")
	for name, args := range map[string][]string{
		"no file":      {"convergence", "metrics"},
		"unknown flag": {"convergence", "metrics", "--bogus", three},
		"missing file": {"convergence", "metrics", missing},
	} {
		out, _, code, _ := acsassert.SubprocessOutput(bin, args...)
		if code != 10 {
			t.Errorf("RED: %s: exit=%d, want 10", name, code)
		}
		if out != "" {
			t.Errorf("RED: %s: stdout = %q, want nothing", name, out)
		}
	}
}

func TestC2328_005_EveryNewAndChangedFunctionHasFullCoverage(t *testing.T) {
	dir := moduleDir(t)

	measure := funcsOfFile(coverFunc(t, dir, "./internal/convergence", "^TestMeasure"), "/convergence/metrics.go")
	if _, ok := measure["Measure"]; !ok {
		t.Fatalf("RED: internal/convergence/metrics.go declares no Measure function: %v", measure)
	}
	cliCover := coverFunc(t, dir, "./cmd/evolve", "^TestCmd_Convergence")
	cli := funcsOfFile(cliCover, "/cmd_convergence_metrics.go")
	if _, ok := cli["encodeConvergenceMetrics"]; !ok {
		t.Fatalf("RED: cmd_convergence_metrics.go declares no encodeConvergenceMetrics function: %v", cli)
	}
	for _, funcs := range []map[string]string{measure, cli} {
		for name, pct := range funcs {
			if pct != "100.0%" {
				t.Errorf("RED: %s coverage = %s, want 100.0%%", name, pct)
			}
		}
	}
	if pct, err := acsassert.FuncCoverage(cliCover, "runConvergence"); err != nil || pct != 100 {
		t.Errorf("RED: runConvergence coverage = (%v, %v), want (100, nil)", pct, err)
	}
}
