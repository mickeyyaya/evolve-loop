package verdict

// replay_test.go — ADR-0103 unit 11 §6 test 33: the host's response and probe
// goldens (captured on 8e8f080f through runner.New + Run, before any code
// moved) replayed through Judge with equivalent fakes over real temp dirs —
// byte-identical JSON, identical probe/sleep counts. Any drift in a moved
// literal shows here.

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
)

const panePASS = "pane says PASS\n"

type replayScenario struct {
	name      string
	phase     string
	optional  bool
	bridgeErr error
	file      string
	stdout    string
	probe     probe
	verdict   string
	nextPhase string
	diags     []core.Diagnostic
	stale     bool
	acs       string
	wantErr   bool
}

func staleBody(phase string) string {
	return "# " + phase + "\n<!-- evolve-verdict: {\"phase\":\"" + phase + "\",\"verdict\":\"PASS\"} -->\n"
}

// replayScenarios mirrors the host's verdictScenarios table row for row.
func replayScenarios() []replayScenario {
	missing := []string{deliverable.CodeMissingArtifact}
	unverified := []string{deliverable.CodeMissingChallengeToken, deliverable.CodeBadVerdict}
	return []replayScenario{
		{name: "timeout_wellformed_pass", phase: "audit", bridgeErr: timeoutErr(), file: reportPASS, probe: probe{okFrom: 1}, verdict: core.VerdictPASS, nextPhase: "ship"},
		{name: "timeout_sentinel_fail", phase: "audit", bridgeErr: timeoutErr(), file: "# audit\nFAIL\n", probe: probe{okFrom: 1}, verdict: core.VerdictFAIL, nextPhase: "retro"},
		{name: "timeout_malformed_mandatory", phase: "audit", bridgeErr: timeoutErr(), probe: probe{codes: missing}, verdict: core.VerdictPASS, wantErr: true},
		{name: "transient_malformed_mandatory", phase: "audit", bridgeErr: transientErr(), file: unverifiedReport, probe: probe{codes: unverified}, verdict: core.VerdictPASS, wantErr: true},
		{name: "timeout_malformed_optional", phase: "build-planner", optional: true, bridgeErr: timeoutErr(), probe: probe{codes: missing}, verdict: core.VerdictPASS},
		{name: "timeout_stale_mandatory", phase: "audit", bridgeErr: timeoutErr(), stale: true, probe: probe{okFrom: 1}, verdict: core.VerdictPASS, wantErr: true},
		{name: "timeout_stale_optional", phase: "smell-scan", optional: true, bridgeErr: timeoutErr(), stale: true, probe: probe{okFrom: 1}, verdict: core.VerdictPASS},
		{name: "teardown_acs_floor", phase: "audit", bridgeErr: timeoutErr(), file: reportWithToken, acs: "PASS", probe: probe{codes: []string{deliverable.CodeStrayInWorktree}}, verdict: core.VerdictPASS, nextPhase: "ship"},
		{name: "substantive_error", phase: "audit", bridgeErr: errors.New("bridge: launch exit=2"), file: reportPASS, probe: probe{okFrom: 1}, verdict: core.VerdictPASS, wantErr: true},
		{name: "clean_contracted_ok", phase: "audit", file: reportPASS, stdout: panePASS, probe: probe{okFrom: 1}, verdict: core.VerdictPASS, nextPhase: "ship", diags: []core.Diagnostic{{Severity: "warning", Message: "classify note"}}},
		{name: "clean_contracted_unverified", phase: "audit", file: unverifiedReport, stdout: panePASS, probe: probe{codes: unverified}, verdict: core.VerdictPASS, nextPhase: "ship"},
		{name: "clean_uncontracted_pane", phase: "widget-scan", stdout: panePASS, probe: probe{err: errors.New("no deliverable contract for phase widget-scan")}, verdict: core.VerdictPASS, nextPhase: "audit"},
		// the probe-only rows (no response golden)
		{name: "timeout_settles_on_third", phase: "audit", bridgeErr: timeoutErr(), file: reportPASS, probe: probe{okFrom: 3, codes: missing}, verdict: core.VerdictPASS},
		{name: "clean_settles_on_third", phase: "audit", file: reportPASS, stdout: panePASS, probe: probe{okFrom: 3, codes: missing}, verdict: core.VerdictPASS},
		{name: "clean_never_settles", phase: "audit", file: unverifiedReport, stdout: panePASS, probe: probe{codes: []string{deliverable.CodeBadVerdict}}, verdict: core.VerdictFAIL},
	}
}

// replay drives one row through Judge exactly as the host's Run would have:
// the pre-dispatch snapshot before the "launch", the agent's report written
// by the "bridge", then the engine.
func replay(t *testing.T, ctx context.Context, sc replayScenario) (*harness, core.PhaseResponse, error) {
	t.Helper()
	h := newHarness(t, sc.probe, WithOptional(sc.optional))
	if sc.acs != "" {
		h.writeACSFloor(t, sc.acs)
	}
	d := h.dispatch(sc.phase, sc.bridgeErr)
	d.Bridge.Stdout = sc.stdout
	if sc.stale {
		d.PreDispatch, d.HadPreDispatch = seedStale(t, d.ArtifactPath, staleBody(sc.phase)), true
	}
	if sc.file != "" {
		h.writeReport(t, sc.phase, sc.file)
	}
	resp, err := h.e.Judge(ctx, d, classifyAs(sc.verdict, sc.nextPhase, sc.diags...))
	return h, resp, err
}

func TestJudge_ReplaysTheHostGoldens(t *testing.T) {
	var probes map[string]struct{ Verify, Sleep int }
	if err := json.Unmarshal([]byte(readGolden(t, "probes.golden.json")), &probes); err != nil {
		t.Fatal(err)
	}
	for _, sc := range replayScenarios() {
		t.Run(sc.name, func(t *testing.T) {
			h, resp, err := replay(t, context.Background(), sc)
			if (err != nil) != sc.wantErr {
				t.Fatalf("err=%v, want error=%v", err, sc.wantErr)
			}
			if want, ok := probes[sc.name]; !ok || h.n.verify != want.Verify || h.n.sleep != want.Sleep {
				t.Errorf("probes/sleeps = %d/%d, want %+v (in golden: %v)", h.n.verify, h.n.sleep, want, ok)
			}
			golden, gerr := os.ReadFile(filepath.Join("testdata", "response_"+sc.name+".golden.json"))
			if gerr != nil {
				return // a probe-only row
			}
			raw, merr := json.Marshal(resp)
			if merr != nil {
				t.Fatal(merr)
			}
			got := strings.NewReplacer(h.ws+"-wt", "{wt}", h.ws, "{ws}", h.root, "{root}").Replace(string(raw))
			if got != string(golden) {
				t.Errorf("response drifted from the host golden:\n got %s\nwant %s", got, golden)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for name, sc := range map[string]replayScenario{
		"cancelled_teardown_never_settles": {phase: "audit", bridgeErr: timeoutErr(), probe: probe{codes: []string{deliverable.CodeMissingArtifact}}, verdict: core.VerdictPASS, wantErr: true},
		"cancelled_clean_never_settles":    {phase: "audit", file: unverifiedReport, stdout: panePASS, probe: probe{codes: []string{deliverable.CodeBadVerdict}}, verdict: core.VerdictFAIL},
	} {
		h, _, err := replay(t, ctx, sc)
		if (err != nil) != sc.wantErr {
			t.Fatalf("%s: err=%v, want error=%v", name, err, sc.wantErr)
		}
		if want := probes[name]; h.n.verify != want.Verify || h.n.sleep != want.Sleep {
			t.Errorf("%s: probes/sleeps = %d/%d, want %+v", name, h.n.verify, h.n.sleep, want)
		}
	}
}
