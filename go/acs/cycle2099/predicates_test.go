//go:build acs

package cycle2099

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	evolveCmdPkg = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"
	measureDoc   = "docs/architecture/packages/internal-wavemeasure.md"
	spendDoc     = "docs/architecture/packages/internal-cyclespend.md"
	timeLayout   = "2006-01-02T15:04:05.000000Z"
)

var (
	cycleZero = time.Date(2026, 10, 27, 8, 0, 0, 0, time.UTC)
	builtOnce sync.Once
	builtBin  string
	builtErr  string
	builtDir  string
)

func TestMain(m *testing.M) {
	code := m.Run()
	if builtDir != "" {
		os.RemoveAll(builtDir)
	}
	os.Exit(code)
}

func minuteMark(m int) time.Time { return cycleZero.Add(time.Duration(m) * time.Minute) }

func callRow(startMinute, endMinute int) string {
	return fmt.Sprintf(`{"phase":"build","started_at":%q,"ended_at":%q,"duration_ms":%d,"usage_status":"measured","tokens":{"input":1,"output":1}}`+"\n",
		minuteMark(startMinute).Format(timeLayout), minuteMark(endMinute).Format(timeLayout), (endMinute-startMinute)*60_000)
}

func signalRow(minute int, kind, code, fields string) string {
	return fmt.Sprintf(`{"schema_version":"1.0","seq":1,"pid":1,"ts":%q,"module":"loop","origin":"x","kind":%q,"code":%q,"severity":"INFO","reason":"r","fields":{%s}}`+"\n",
		minuteMark(minute).Format(time.RFC3339Nano), kind, code, fields)
}

type project struct {
	t    *testing.T
	root string
}

func newProject(t *testing.T) project {
	t.Helper()
	return project{t: t, root: t.TempDir()}
}

func (p project) write(rel, body string) {
	p.t.Helper()
	path := filepath.Join(p.root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		p.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		p.t.Fatal(err)
	}
}

func (p project) seedCycle(id int, calls, signals string) {
	p.t.Helper()
	dir := fmt.Sprintf(".evolve/runs/cycle-%d/", id)
	p.write(dir+"llm-calls.ndjson", calls)
	p.write(dir+"signals.ndjson", signals)
}

func (p project) ship(id int) {
	p.t.Helper()
	p.write(fmt.Sprintf(".evolve/landing/cycle-%d.json", id), fmt.Sprintf(`{"cycle":%d,"status":"complete"}`, id))
}

func (p project) record(floor int) {
	p.t.Helper()
	rec := wave.Record{Number: 143, RunID: "r143", CycleFloor: floor, StartedAt: minuteMark(-60)}
	if err := wave.NewStore(filepath.Join(p.root, ".evolve")).Save(rec); err != nil {
		p.t.Fatal(err)
	}
}

func near(got *float64, want float64) bool { return got != nil && math.Abs(*got-want) < 1e-9 }

func cycleByID(t *testing.T, cycles []wave.Cycle, id int) wave.Cycle {
	t.Helper()
	for _, c := range cycles {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("no cycle %d among %d cycle(s)", id, len(cycles))
	return wave.Cycle{}
}

func TestC2099_001_AFailedCycleWithAFloorRejectionAtMinute30AndTwentyMoreMinutesHasFailPoint30Waste20Ratio06(t *testing.T) {
	p := newProject(t)
	p.seedCycle(21, callRow(0, 10)+callRow(10, 30)+callRow(30, 50),
		signalRow(0, "ledger.appended", "", "")+
			signalRow(30, "gate.rejected", "ORCHESTRATOR_BUILD_FLOOR", "")+
			signalRow(50, "cycle.sealed", "", `"final_verdict":"FAIL"`))

	cycles, warnings := wave.ReadCycles(p.root, 0, 0)

	if len(cycles) != 1 || len(warnings) != 0 {
		t.Fatalf("RED: ReadCycles = %d cycle(s), warnings %q; want one cycle", len(cycles), warnings)
	}
	v := cycles[0].LLMVerdict
	if v.LLMMinutesToVerdict != 50 || !near(v.FailPointMinutes, 30) || !near(v.WasteMinutes, 20) || !near(v.FailFastRatio, 0.6) {
		t.Errorf("RED: LLMVerdict = %+v, want 50 minutes to verdict, fail point 30, waste 20, ratio 0.6", v)
	}
}

func TestC2099_002_TheFirstBlockingSignalSetsTheFailPointAndAPassOutcomeBlocksNothing(t *testing.T) {
	p := newProject(t)
	p.seedCycle(22, callRow(0, 10)+callRow(10, 30)+callRow(30, 50),
		signalRow(10, "phase.outcome", "", `"verdict":"PASS"`)+
			signalRow(40, "gate.rejected", "ORCHESTRATOR_BUILD_FLOOR", "")+
			signalRow(30, "phase.outcome", "ORCHESTRATOR_PHASE_VERDICT_FAIL", `"verdict":"FAIL"`))

	cycles, _ := wave.ReadCycles(p.root, 0, 0)

	v := cycleByID(t, cycles, 22).LLMVerdict
	if !near(v.FailPointMinutes, 30) || !near(v.FailFastRatio, 0.6) {
		t.Errorf("RED: LLMVerdict = %+v, want fail point 30 and ratio 0.6: the earliest FAIL outcome wins, the PASS outcome at minute 10 blocks nothing", v)
	}
}

func TestC2099_003_AFailedCycleWithNoBlockingSignalHasNoFailPointInsteadOfAnInventedOne(t *testing.T) {
	p := newProject(t)
	p.seedCycle(24, callRow(0, 10)+callRow(10, 40),
		signalRow(0, "ledger.appended", "", "")+signalRow(40, "cycle.sealed", "", `"final_verdict":"FAIL"`))

	cycles, _ := wave.ReadCycles(p.root, 0, 0)

	v := cycleByID(t, cycles, 24).LLMVerdict
	if v.LLMMinutesToVerdict != 40 || v.FailPointMinutes != nil || v.WasteMinutes != nil || v.FailFastRatio != nil {
		t.Errorf("RED: LLMVerdict = %+v, want 40 minutes to verdict and no fail point, waste or ratio", v)
	}
}

func TestC2099_004_AShippedCycleGivesMinutesToShipAndNoFailPointEvenAfterACorrectedRejection(t *testing.T) {
	p := newProject(t)
	p.seedCycle(23, callRow(0, 15)+callRow(15, 40),
		signalRow(0, "ledger.appended", "", "")+
			signalRow(15, "gate.rejected", "ORCHESTRATOR_BUILD_FLOOR", "")+
			signalRow(40, "cycle.sealed", "", `"final_verdict":"PASS"`))
	p.ship(23)

	cycles, _ := wave.ReadCycles(p.root, 0, 0)

	c := cycleByID(t, cycles, 23)
	v := c.LLMVerdict
	if !c.Shipped || v.LLMMinutesToVerdict != 40 || v.FailPointMinutes != nil || v.WasteMinutes != nil || v.FailFastRatio != nil {
		t.Errorf("RED: shipped=%v LLMVerdict = %+v, want shipped with 40 minutes to ship and no fail point", c.Shipped, v)
	}
}

func TestC2099_005_ACycleWithNoLLMLedgerKeepsAZeroVerdictAndNeverADividedByZeroRatio(t *testing.T) {
	p := newProject(t)
	p.write(".evolve/runs/cycle-25/signals.ndjson", signalRow(0, "ledger.appended", "", "")+signalRow(5, "gate.rejected", "ORCHESTRATOR_BUILD_FLOOR", ""))

	cycles, _ := wave.ReadCycles(p.root, 0, 0)

	v := cycleByID(t, cycles, 25).LLMVerdict
	if v.LLMMinutesToVerdict != 0 || v.FailPointMinutes != nil || v.FailFastRatio != nil {
		t.Errorf("RED: LLMVerdict = %+v, want the zero verdict for a cycle with no LLM calls", v)
	}
}

func evolveBinary(t *testing.T) string {
	t.Helper()
	builtOnce.Do(func() {
		dir, err := os.MkdirTemp("", "acs-cycle2099-")
		if err != nil {
			builtErr = err.Error()
			return
		}
		builtDir = dir
		bin := filepath.Join(dir, "evolve")
		_, stderr, code, err := acsassert.SubprocessOutput("go", "build", "-o", bin, evolveCmdPkg)
		if code != 0 || err != nil {
			builtErr = fmt.Sprintf("go build exit %d: %v\n%s", code, err, acsassert.OutputTail(stderr, 30))
			return
		}
		builtBin = bin
	})
	if builtBin == "" {
		t.Fatalf("RED: the evolve binary did not build: %s", builtErr)
	}
	return builtBin
}

func seedWaveOfTwoCycles(t *testing.T) project {
	t.Helper()
	p := newProject(t)
	p.seedCycle(1835, callRow(0, 15)+callRow(15, 40), signalRow(0, "ledger.appended", "", "")+signalRow(40, "cycle.sealed", "", `"final_verdict":"PASS"`))
	p.ship(1835)
	p.seedCycle(1837, callRow(0, 30)+callRow(30, 50),
		signalRow(0, "ledger.appended", "", "")+signalRow(30, "gate.rejected", "ORCHESTRATOR_BUILD_FLOOR", "")+signalRow(50, "cycle.sealed", "", `"final_verdict":"FAIL"`))
	p.record(1834)
	return p
}

func waveStatus(t *testing.T, root string, extra ...string) string {
	t.Helper()
	args := append([]string{"wave", "status", "--project-root", root}, extra...)
	stdout, stderr, code, err := acsassert.SubprocessOutput(evolveBinary(t), args...)
	if code != 0 || err != nil {
		t.Fatalf("RED: evolve %s exit %d: %v\n%s%s", strings.Join(args, " "), code, err, acsassert.OutputTail(stdout, 20), acsassert.OutputTail(stderr, 20))
	}
	return stdout
}

func TestC2099_006_WaveStatusJSONListsThePerCycleValuesAndTheWaveMediansFromTheBuiltBinary(t *testing.T) {
	p := seedWaveOfTwoCycles(t)

	out := waveStatus(t, p.root, "--json")

	var envelope struct {
		OK   bool `json:"ok"`
		Data struct {
			Cycles []struct {
				ID         int `json:"id"`
				LLMVerdict struct {
					Minutes   float64  `json:"llm_minutes_to_verdict"`
					FailPoint *float64 `json:"fail_point_minutes"`
					Waste     *float64 `json:"waste_minutes"`
					Ratio     *float64 `json:"fail_fast_ratio"`
				} `json:"llm_verdict"`
			} `json:"cycles"`
			Measures struct {
				Cost struct {
					MedianMinutes *float64 `json:"median_llm_minutes_to_verdict"`
					MedianRatio   *float64 `json:"median_fail_fast_ratio"`
				} `json:"cost"`
			} `json:"measures"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &envelope); err != nil || !envelope.OK {
		t.Fatalf("RED: wave status --json is not JSON: %v\n%s", err, out)
	}
	report := envelope.Data
	seen := map[int]bool{}
	for _, c := range report.Cycles {
		seen[c.ID] = true
		v := c.LLMVerdict
		switch c.ID {
		case 1835:
			if v.Minutes != 40 || v.FailPoint != nil || v.Ratio != nil {
				t.Errorf("RED: cycle 1835 llm_verdict = %+v, want 40 minutes to ship and no fail point", v)
			}
		case 1837:
			if v.Minutes != 50 || !near(v.FailPoint, 30) || !near(v.Waste, 20) || !near(v.Ratio, 0.6) {
				t.Errorf("RED: cycle 1837 llm_verdict = %+v, want 50 minutes, fail point 30, waste 20, ratio 0.6", v)
			}
		}
	}
	if !seen[1835] || !seen[1837] {
		t.Errorf("RED: cycles listed = %v, want 1835 and 1837", seen)
	}
	cost := report.Measures.Cost
	if !near(cost.MedianMinutes, 45) || !near(cost.MedianRatio, 0.6) {
		t.Errorf("RED: wave medians = %v and %v, want 45 minutes and ratio 0.6", cost.MedianMinutes, cost.MedianRatio)
	}
}

func TestC2099_007_WaveStatusTextListsThePerCycleValuesAndTheWaveMedianFromTheBuiltBinary(t *testing.T) {
	p := seedWaveOfTwoCycles(t)

	out := waveStatus(t, p.root)

	for _, want := range []string{
		"40.0 LLM min to ship",
		"50.0 LLM min to verdict, fail point 30.0 min, waste 20.0 min, fail-fast 0.60",
		"median 45.0 LLM min to verdict · median fail-fast 0.60",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("RED: wave status text lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(waveStatusOfShippedOnlyWave(t), "fail-fast") {
		t.Errorf("RED: a wave with no failed cycle must show no fail-fast value")
	}
}

func waveStatusOfShippedOnlyWave(t *testing.T) string {
	t.Helper()
	p := newProject(t)
	p.seedCycle(1835, callRow(0, 15)+callRow(15, 40), signalRow(0, "ledger.appended", "", "")+signalRow(40, "cycle.sealed", "", `"final_verdict":"PASS"`))
	p.ship(1835)
	p.record(1834)
	return waveStatus(t, p.root)
}

func lintedDoc(t *testing.T, rel string) (string, []stelint.Finding) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	words, err := stelint.LoadStandard(root)
	if err != nil {
		t.Fatalf("load the STE standard: %v", err)
	}
	path := filepath.Join(root, filepath.FromSlash(rel))
	findings, err := stelint.LintFile(path, stelint.Options{Words: words})
	if err != nil {
		t.Fatalf("lint %s: %v", rel, err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(body), findings
}

func TestC2099_008_TheDocOfTheMeasureNamesTheVerdictMeasureAndEveryLineAboutItPassesSTE(t *testing.T) {
	for _, rel := range []string{measureDoc, spendDoc} {
		body, findings := lintedDoc(t, rel)
		aboutTheMeasure := 0
		for i, line := range strings.Split(body, "\n") {
			lower := strings.ToLower(line)
			if !strings.Contains(lower, "fail point") && !strings.Contains(lower, "fail-fast") && !strings.Contains(line, "VerdictOf") {
				continue
			}
			aboutTheMeasure++
			for _, f := range findings {
				if f.Line == i+1 {
					t.Errorf("RED: %s:%d breaks STE (%s): %s", rel, f.Line, f.Rule, f.Message)
				}
			}
		}
		if aboutTheMeasure == 0 {
			t.Errorf("RED: %s has no line about the fail point, the fail-fast ratio or VerdictOf", rel)
		}
	}
	body, _ := lintedDoc(t, measureDoc)
	for _, key := range []string{"median_llm_minutes_to_verdict", "median_fail_fast_ratio", "llm_verdict"} {
		if !strings.Contains(body, key) {
			t.Errorf("RED: %s does not name the JSON key %s", measureDoc, key)
		}
	}
}
