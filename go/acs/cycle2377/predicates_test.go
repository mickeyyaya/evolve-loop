//go:build acs

package cycle2377

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	evolveCmdPkg   = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"
	exceededCode   = "LATENCY_BUDGET_EXCEEDED"
	poolTarget     = 4
	spanMinutes    = 100
	healthyModelAt = 90
)

var (
	waveZero  = time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
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

type waveSpec struct {
	live       int
	modelUntil int
	extra      []event
}

type event struct {
	minute int
	kind   string
	code   string
	fields string
}

func healthy() waveSpec { return waveSpec{live: poolTarget, modelUntil: healthyModelAt} }

func lowLive() waveSpec { return waveSpec{live: 2, modelUntil: healthyModelAt} }

func gateHeavy() waveSpec { return waveSpec{live: poolTarget, modelUntil: 50} }

func (w waveSpec) with(e event) waveSpec {
	w.extra = append(append([]event{}, w.extra...), e)
	return w
}

func quotaPause() event {
	return event{minute: 30, kind: "quota.paused", code: "ORCHESTRATOR_QUOTA_PAUSED"}
}

func loopHalt(stopReason string) event {
	return event{minute: 40, kind: "loop.halt", code: "LOOP_HALT", fields: fmt.Sprintf(`"stop_reason":%q`, stopReason)}
}

func supplyShort() event {
	return event{minute: 20, kind: "loop.wave", code: "LOOP_ORCH_SUPPLY_SHORT", fields: `"admissible":"1","window":"4"`}
}

func waveStart(n int) time.Time { return waveZero.Add(time.Duration(n-1) * 3 * time.Hour) }

func wavePID(n int) int { return 5000 + n }

type project struct {
	t    *testing.T
	root string
	repo *gittest.Repo
}

func (p project) write(rel, body string) { p.repo.Write(rel, body) }

func stamp(t time.Time) string { return t.Format(time.RFC3339Nano) }

func seed(t *testing.T, specs ...waveSpec) project {
	t.Helper()
	repo := gittest.Fixture(t)
	p := project{t: t, root: repo.Dir, repo: repo}
	p.write(".evolve/state.json", fmt.Sprintf(`{"lastCycleNumber":%d}`, len(specs)))
	store := wave.NewStore(filepath.Join(p.root, ".evolve"))
	var stream strings.Builder
	for i, spec := range specs {
		n := i + 1
		start := waveStart(n)
		end := start.Add(2 * time.Hour)
		rec := wave.Record{Number: n, RunID: fmt.Sprintf("r%d", n), CycleFloor: n - 1, PID: wavePID(n), StartedAt: start, EndedAt: &end}
		if err := store.Save(rec); err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&stream, `{"ts":%q,"pid":%d,"kind":"loop.wave","code":"LOOP_ORCH_POOL_IDLE","fields":{"live":"%d","target":"%d"}}`+"\n",
			stamp(start), wavePID(n), spec.live, poolTarget)
		for _, e := range spec.extra {
			fmt.Fprintf(&stream, `{"ts":%q,"pid":%d,"kind":%q,"code":%q,"fields":{%s}}`+"\n",
				stamp(start.Add(time.Duration(e.minute)*time.Minute)), wavePID(n), e.kind, e.code, e.fields)
		}
		dir := fmt.Sprintf(".evolve/runs/cycle-%d/", n)
		p.write(dir+"signals.ndjson",
			fmt.Sprintf(`{"ts":%q,"kind":"phase.dispatched","phase":"build"}`+"\n", stamp(start))+
				fmt.Sprintf(`{"ts":%q,"kind":"phase.outcome","phase":"build","fields":{"verdict":"PASS"}}`+"\n", stamp(start.Add(spanMinutes*time.Minute))))
		p.write(dir+"llm-calls.ndjson",
			fmt.Sprintf(`{"phase":"build","cli":"claude-tmux","started_at":%q,"ended_at":%q}`+"\n", stamp(start), stamp(start.Add(time.Duration(spec.modelUntil)*time.Minute))))
	}
	p.write(".evolve/signals.ndjson", stream.String())
	return p
}

func (p project) setPolicy(body string) { p.write(".evolve/policy.json", body) }

func evolveBinary(t *testing.T) string {
	t.Helper()
	builtOnce.Do(func() {
		dir, err := os.MkdirTemp("", "acs-cycle2377-")
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

type checkedWave struct {
	Wave    int      `json:"wave"`
	Status  string   `json:"status"`
	Reasons []string `json:"reasons"`
}

type checkReport struct {
	Waves    []checkedWave `json:"waves"`
	Exceeded bool          `json:"exceeded"`
}

func (r checkReport) byWave(t *testing.T, n int) checkedWave {
	t.Helper()
	for _, w := range r.Waves {
		if w.Wave == n {
			return w
		}
	}
	t.Fatalf("RED: the check report has no row for wave %d: %+v", n, r.Waves)
	return checkedWave{}
}

func (w checkedWave) mentions(word string) bool {
	return strings.Contains(strings.ToLower(strings.Join(w.Reasons, " ")), word)
}

func (p project) check() checkReport {
	p.t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(evolveBinary(p.t), "wave", "latency", "--check", "--json", "--project-root", p.root)
	if err != nil || code != 0 {
		p.t.Fatalf("RED: evolve wave latency --check exit %d (%v); a soft gate never fails the command\nstdout: %s\nstderr: %s",
			code, err, acsassert.OutputTail(stdout, 20), acsassert.OutputTail(stderr, 20))
	}
	var report checkReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil || len(report.Waves) == 0 {
		p.t.Fatalf("RED: --check --json did not print a report with a waves array (%v):\n%s", err, acsassert.OutputTail(stdout, 20))
	}
	return report
}

func (p project) streamLines() []map[string]any {
	p.t.Helper()
	raw, err := os.ReadFile(filepath.Join(p.root, ".evolve", "signals.ndjson"))
	if err != nil {
		p.t.Fatal(err)
	}
	var out []map[string]any
	for _, line := range strings.Split(string(raw), "\n") {
		var row map[string]any
		if json.Unmarshal([]byte(line), &row) == nil {
			out = append(out, row)
		}
	}
	return out
}

func (p project) exceededEvents() []map[string]any {
	p.t.Helper()
	var out []map[string]any
	for _, row := range p.streamLines() {
		if row["code"] == exceededCode {
			out = append(out, row)
		}
	}
	return out
}

func (p project) inboxItems() []string {
	p.t.Helper()
	entries, err := os.ReadDir(filepath.Join(p.root, ".evolve", "inbox"))
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, e.Name())
		}
	}
	return names
}

func TestC2377_001_AQuotaPauseLeavesAWaveOutOfTheCheckWhileTheSameWaveWithoutItBreaches(t *testing.T) {
	control := seed(t, lowLive()).check().byWave(t, 1)
	if control.Status != "breach" {
		t.Fatalf("RED: the control wave (2 of 4 live lanes, no pause) has status %q, want breach: the exclusion below proves nothing without it", control.Status)
	}

	paused := seed(t, lowLive().with(quotaPause())).check().byWave(t, 1)

	if paused.Status != "left_out" || !paused.mentions("quota") {
		t.Errorf("RED: the same wave with an ORCHESTRATOR_QUOTA_PAUSED signal has status %q reasons %v, want left_out because of the quota pause", paused.Status, paused.Reasons)
	}
}

func TestC2377_002_ALoopStopReasonAndTheOperatorBrakeEachLeaveAWaveOut(t *testing.T) {
	cases := []struct {
		name       string
		halt       event
		wantReason string
	}{
		{"a loop stop_reason", loopHalt("disk_space_halt"), "stop"},
		{"the operator brake", loopHalt("loop_operator_brake"), "brake"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := seed(t, lowLive().with(c.halt)).check().byWave(t, 1)

			if got.Status != "left_out" || !got.mentions(c.wantReason) {
				t.Errorf("RED: a low-lane wave with %s has status %q reasons %v, want left_out naming %q", c.name, got.Status, got.Reasons, c.wantReason)
			}
		})
	}
}

func TestC2377_003_AWaveBreachesOnTheLiveLaneRuleOnlyWhileTheReadySupplyIsNotShort(t *testing.T) {
	starved := seed(t, lowLive().with(supplyShort())).check().byWave(t, 1)
	if starved.Status != "ok" {
		t.Errorf("RED: low live lanes with a LOOP_ORCH_SUPPLY_SHORT signal has status %q, want ok: the supply, not the pool, held the lanes down", starved.Status)
	}

	breached := seed(t, lowLive()).check().byWave(t, 1)
	if breached.Status != "breach" || !breached.mentions("live") {
		t.Errorf("RED: 2 of 4 live lanes with the supply at the target has status %q reasons %v, want a breach naming the live lanes", breached.Status, breached.Reasons)
	}

	full := seed(t, healthy()).check().byWave(t, 1)
	if full.Status != "ok" {
		t.Errorf("RED: 4 of 4 live lanes and a 10%% gate share has status %q, want ok", full.Status)
	}
}

func TestC2377_004_AWaveBreachesWhenTheHostGateShareIsAboveAQuarterAndNotAtExactlyAQuarter(t *testing.T) {
	over := seed(t, gateHeavy()).check().byWave(t, 1)
	if over.Status != "breach" || !over.mentions("gate") {
		t.Errorf("RED: a 50%% gate share with full lanes has status %q reasons %v, want a breach naming the gate share", over.Status, over.Reasons)
	}

	atQuarter := seed(t, waveSpec{live: poolTarget, modelUntil: 75}).check().byWave(t, 1)
	if atQuarter.Status != "ok" {
		t.Errorf("RED: a gate share of exactly 25%% has status %q, want ok: the rule is above 25%%", atQuarter.Status)
	}
}

func TestC2377_005_TwoBreachingWavesInARowEmitOneWarnSignalAndFileOneItemAndARerunFilesNoSecond(t *testing.T) {
	p := seed(t, healthy(), lowLive(), gateHeavy())

	report := p.check()

	if !report.Exceeded {
		t.Fatalf("RED: waves 2 and 3 breach in a row, but the report says exceeded=false: %+v", report.Waves)
	}
	events := p.exceededEvents()
	if len(events) != 1 {
		t.Fatalf("RED: want exactly one %s event in .evolve/signals.ndjson, got %d", exceededCode, len(events))
	}
	if events[0]["severity"] != "WARN" {
		t.Errorf("RED: the %s event has severity %v, want WARN", exceededCode, events[0]["severity"])
	}
	for _, row := range p.streamLines() {
		if code, _ := row["code"].(string); strings.HasPrefix(code, "SIGNALCENTER_UNREGISTERED") || strings.HasPrefix(code, "SIGNALCENTER_UNKNOWN_MODULE") {
			t.Errorf("RED: %s is not registered with a module of the closed set: the stream holds %v", exceededCode, row)
		}
	}
	items := p.inboxItems()
	if len(items) != 1 {
		t.Fatalf("RED: want exactly one inbox item filed, got %v", items)
	}
	raw, err := os.ReadFile(filepath.Join(p.root, ".evolve", "inbox", items[0]))
	if err != nil {
		t.Fatal(err)
	}
	var item struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	if err := json.Unmarshal(raw, &item); err != nil || item.ID == "" || !strings.Contains(strings.ToLower(item.Title+" "+item.ID), "latency") {
		t.Errorf("RED: the filed item needs an id and a title that names the latency breach, got %+v (%v)", item, err)
	}

	p.check()
	if again := p.inboxItems(); len(again) != 1 {
		t.Errorf("RED: a second boundary run while the item is pending must file no second item, got %v", again)
	}
}

func TestC2377_006_OnlyTheTrailingRunOfBreachesCountsAndALeftOutWaveNeitherCountsNorResetsTheRun(t *testing.T) {
	cases := []struct {
		name  string
		specs []waveSpec
		want  bool
	}{
		{"breach then ok", []waveSpec{lowLive(), healthy()}, false},
		{"breach, ok, breach", []waveSpec{lowLive(), healthy(), lowLive()}, false},
		{"two breaches then ok", []waveSpec{lowLive(), lowLive(), healthy()}, false},
		{"one breach alone", []waveSpec{lowLive()}, false},
		{"breach, left out, breach", []waveSpec{lowLive(), lowLive().with(quotaPause()), lowLive()}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := seed(t, c.specs...)

			report := p.check()

			if report.Exceeded != c.want {
				t.Errorf("RED: exceeded=%v, want %v: %+v", report.Exceeded, c.want, report.Waves)
			}
			if got := len(p.exceededEvents()) == 1; got != c.want {
				t.Errorf("RED: %s emitted=%v, want %v", exceededCode, got, c.want)
			}
			if got := len(p.inboxItems()) == 1; got != c.want {
				t.Errorf("RED: item filed=%v, want %v", got, c.want)
			}
		})
	}
}

func TestC2377_007_TheLiveLaneAndGateShareBoundsComeFromPolicyAndNotFromCode(t *testing.T) {
	mostlyFull := waveSpec{live: 3, modelUntil: healthyModelAt}
	if got := seed(t, mostlyFull).check().byWave(t, 1); got.Status != "breach" {
		t.Fatalf("RED: 3 of 4 live lanes is 75%% and the default floor is 80%%, status %q, want breach", got.Status)
	}
	tolerantLive := seed(t, mostlyFull)
	tolerantLive.setPolicy(`{"wave_latency":{"min_live_share":0.5}}`)
	if got := tolerantLive.check().byWave(t, 1); got.Status != "ok" {
		t.Errorf("RED: with wave_latency.min_live_share 0.5 in policy.json, 75%% live lanes has status %q, want ok", got.Status)
	}
	strictLive := seed(t, healthy())
	strictLive.setPolicy(`{"wave_latency":{"min_live_share":1.1}}`)
	if got := strictLive.check().byWave(t, 1); got.Status != "breach" {
		t.Errorf("RED: with wave_latency.min_live_share 1.1 in policy.json, full lanes has status %q, want breach", got.Status)
	}

	tolerantGate := seed(t, gateHeavy())
	tolerantGate.setPolicy(`{"wave_latency":{"max_gate_share":0.75}}`)
	if got := tolerantGate.check().byWave(t, 1); got.Status != "ok" {
		t.Errorf("RED: with wave_latency.max_gate_share 0.75 in policy.json, a 50%% gate share has status %q, want ok", got.Status)
	}
	strictGate := seed(t, healthy())
	strictGate.setPolicy(`{"wave_latency":{"max_gate_share":0.05}}`)
	if got := strictGate.check().byWave(t, 1); got.Status != "breach" || !got.mentions("gate") {
		t.Errorf("RED: with wave_latency.max_gate_share 0.05 in policy.json, a 10%% gate share has status %q reasons %v, want a breach naming the gate share", got.Status, got.Reasons)
	}
}
