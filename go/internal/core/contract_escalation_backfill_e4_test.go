package core

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/codereview"
	"github.com/mickeyyaya/evolve-loop/go/internal/dispositionrouter"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestRoutingWalk_ADecisionTheTierCeilingEmptiesHasNoWalk(t *testing.T) {
	t.Setenv("EVOLVE_CLI", "")
	root := t.TempDir()
	writeRawProfile(t, root, "builder", map[string]any{
		"name": "builder", "cli": "agy-tmux", "allowed_clis": []string{"agy"},
		"model_tier_default": "deep", "model_tier_envelope": map[string]any{"min": "balanced", "max": "deep"},
	})
	block := policy.CLIRouting{CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"}, Tiers: map[string]policy.TierRule{"deep": {CLIs: []string{"claude"}}}}
	r, _, err := cliroute.Build(cliroute.Setup{
		Policy: policy.Policy{CLIRouting: &block}, Profiles: profiles.NewFromDir(filepath.Join(root, ".evolve", "profiles")),
		Host: cliroute.Host{LookPath: func(string) (string, error) { return "/fake", nil }},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	cr := &cycleRun{o: routedOrchestrator(r), req: CycleRequest{ProjectRoot: root}}

	var (
		d      cliroute.Decision
		walk   []string
		routed bool
	)
	log := captureStderr(t, func() {
		dec, plan, ok := cr.routingWalk(PhaseBuild)
		d, walk, routed = dec, plan.Candidates, ok
	})

	if routed || walk != nil {
		t.Fatalf("routingWalk = (walk %v, routed %v), want no route when the deep ceiling admits no candidate", walk, routed)
	}
	if d.Rule != "default" || len(d.Plan.Candidates) != 1 || d.Plan.Candidates[0] != "agy-tmux" {
		t.Fatalf("the resolved decision must still be returned for diagnostics: %+v", d)
	}
	if !strings.Contains(log, "phase build: contract escalation has no route: ") || !strings.Contains(log, "is permitted at tier deep") {
		t.Fatalf("stderr %q must name the phase and the ceiling refusal", log)
	}
}

func readStagedIntentsE4(t *testing.T, root string) []dispositionrouter.Intent {
	t.Helper()
	dir := filepath.Join(root, ".evolve", "escalations")
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return nil
	}
	f, err := os.Open(dispositionrouter.PendingActionsPath(dir))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	var out []dispositionrouter.Intent
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var in dispositionrouter.Intent
		if err := json.Unmarshal(sc.Bytes(), &in); err != nil {
			t.Fatalf("staged intent %q: %v", sc.Text(), err)
		}
		out = append(out, in)
	}
	return out
}

func demotionRunE4(t *testing.T, root string, ledger *fakeLedger) *cycleRun {
	t.Helper()
	t.Setenv("EVOLVE_CLI", "")
	o := NewOrchestrator(&fakeStorage{}, ledger, buildRunners(nil))
	return &cycleRun{o: o, ctx: context.Background(), cycle: 41, req: CycleRequest{ProjectRoot: root}}
}

func TestNoteContractGateDemotion_EachFailingSinkWarnsAndTheOthersStillRecord(t *testing.T) {
	compiled := policy.Policy{}.RetroAutofileDefaultWeight()
	cases := []struct {
		name        string
		failAppend  bool
		policyJSON  string
		blockStage  bool
		wantWarn    string
		wantLedger  int
		wantIntents int
		wantWeight  float64
	}{
		{"ledger append fails", true, "", false, "contract_gate_demoted ledger append: forced ledger append fail", 0, 1, compiled},
		{"malformed policy uses the compiled weight", false, "{not json", false, "policy load for autofile weight", 1, 1, compiled},
		{"escalation staging fails", false, "", true, "stage escalation intent", 1, 0, compiled},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if tc.policyJSON != "" {
				writeFile(t, filepath.Join(root, ".evolve", "policy.json"), tc.policyJSON)
			}
			if tc.blockStage {
				writeFile(t, filepath.Join(root, ".evolve", "escalations"), "a file where the dir belongs")
			}
			ledger := &fakeLedger{failOnAppend: tc.failAppend}
			cr := demotionRunE4(t, root, ledger)

			log := captureStderr(t, func() {
				cr.noteContractGateDemotion(PhaseBuild, contractDispatch{cli: "claude-tmux", escalated: true}, 2, "missing Verdict line")
			})

			if !strings.Contains(log, tc.wantWarn) {
				t.Fatalf("stderr %q lacks %q", log, tc.wantWarn)
			}
			if len(ledger.entries) != tc.wantLedger {
				t.Fatalf("ledger entries = %+v, want %d", ledger.entries, tc.wantLedger)
			}
			if tc.wantLedger == 1 {
				e := ledger.entries[0]
				if e.Kind != ledgerKindContractGateDemoted || e.Cycle != 41 || !strings.Contains(e.Action, "escalated=true") || !strings.Contains(e.Action, "blocks=2: missing Verdict line") {
					t.Fatalf("ledger entry = %+v", e)
				}
			}
			intents := readStagedIntentsE4(t, root)
			if len(intents) != tc.wantIntents {
				t.Fatalf("staged intents = %+v, want %d", intents, tc.wantIntents)
			}
			if tc.wantIntents == 1 && (intents[0].ItemID != "contract-gate-demoted-build" || intents[0].Weight != tc.wantWeight || intents[0].Cycle != 41 || intents[0].Recurrence != 2) {
				t.Fatalf("staged intent = %+v, want weight %v", intents[0], tc.wantWeight)
			}
		})
	}
}

func TestNoteContractExhaustionSkip_ALedgerFailureWarnsWithoutDroppingTheSkip(t *testing.T) {
	t.Parallel()

	ledger := &fakeLedger{failOnAppend: true}
	center := signalcenter.New()
	var events []signalcenter.Event
	center.Subscribe(func(e signalcenter.Event) { events = append(events, e) })
	o := NewOrchestrator(&fakeStorage{}, ledger, buildRunners(nil), WithSignalCenter(center))

	var sink injectedStderr
	o.stderr = &sink
	o.noteContractExhaustionSkip(context.Background(), CycleState{CycleID: 5, WorkspacePath: t.TempDir()}, Phase(codereview.PhaseName))
	log := sink.String()

	if !strings.Contains(log, "WARN "+ledgerKindContractExhaustionSkip+" ledger append: forced ledger append fail") {
		t.Fatalf("stderr %q must surface the lost exhaustion-skip ledger row", log)
	}
	if len(ledger.entries) != 0 {
		t.Fatalf("a failed append recorded %+v", ledger.entries)
	}
	if len(events) != 1 || events[0].Code != codereview.CodeSkipped || events[0].Cycle != 5 {
		t.Fatalf("the code-review skip must still be signalled after a ledger failure: %+v", events)
	}
}
