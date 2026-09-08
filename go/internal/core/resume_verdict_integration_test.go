//go:build integration

package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
)

func TestResumeVerdict_PostAuditCloseoutPreservesHostDisposition(t *testing.T) {
	for _, tc := range []struct {
		name, persisted, timing, want string
		failed                        bool
	}{
		{name: "persisted failure", persisted: VerdictFAIL, want: VerdictFAIL},
		{name: "persisted warning", persisted: VerdictWARN, want: VerdictWARN},
		{name: "persisted pass", persisted: VerdictPASS, want: VerdictPASS},
		{name: "legacy host rejection", failed: true, want: VerdictFAIL},
		{name: "legacy host timing", timing: `[{"phase":"audit","verdict":"WARN"},{"phase":"retro","verdict":"PASS"}]`, want: VerdictWARN},
		{name: "legacy missing final floor", timing: `[{"phase":"build","verdict":"PASS"}]`, want: VerdictWARN},
		{name: "legacy absent evidence", want: VerdictWARN},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, st, req, rp := resumedLifecycleFixture(t)
			rp.Phase = string(PhaseRetro)
			st.cycleState.CompletedPhases = append(st.cycleState.CompletedPhases, "audit")
			// Decode the additive checkpoint field to keep the RED test runnable
			// against legacy code that does not yet retain it.
			if err := json.Unmarshal([]byte(`{"final_verdict":"`+tc.persisted+`"}`), &st.cycleState); err != nil {
				t.Fatal(err)
			}
			if tc.failed {
				st.cycleState.AuditFailReasons = []string{"host test gate failed"}
			}
			if tc.timing != "" {
				if err := os.WriteFile(filepath.Join(st.cycleState.WorkspacePath, "phase-timing.json"), []byte(tc.timing), 0644); err != nil {
					t.Fatal(err)
				}
			}

			got, err := o.RunCycleFromPhase(context.Background(), req, rp)

			if err != nil || got.FinalVerdict != tc.want {
				t.Fatalf("resumed verdict=%q err=%v, want %q and nil", got.FinalVerdict, err, tc.want)
			}
			raw, err := os.ReadFile(filepath.Join(dossier.CyclesDir(req.ProjectRoot), "cycle-7.json"))
			if err != nil {
				t.Fatal(err)
			}
			d, err := dossier.ParseJSON(raw)
			if err != nil || d.FinalVerdict != tc.want {
				t.Fatalf("durable verdict=%q err=%v, want %q and nil", d.FinalVerdict, err, tc.want)
			}
		})
	}
}

func TestResumeVerdict_FloorCompletionPersistsBeforeNextDispatch(t *testing.T) {
	for _, resume := range []bool{false, true} {
		t.Run(map[bool]string{false: "fresh", true: "resumed"}[resume], func(t *testing.T) {
			o, st, req, rp := resumedLifecycleFixture(t)
			var err error
			if resume {
				_, err = o.RunCycleFromPhase(context.Background(), req, rp)
			} else {
				_, err = o.RunCycle(context.Background(), req)
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, cs := range st.cycleStateLog {
				if cs.Phase != "audit" || len(cs.CompletedPhases) == 0 || cs.CompletedPhases[len(cs.CompletedPhases)-1] != "audit" {
					continue
				}
				raw, err := json.Marshal(cs)
				if err != nil {
					t.Fatal(err)
				}
				var saved map[string]any
				if err := json.Unmarshal(raw, &saved); err != nil {
					t.Fatal(err)
				}
				if saved["final_verdict"] != VerdictPASS {
					t.Fatalf("completed audit persisted without its host verdict: %v", saved["final_verdict"])
				}
				return
			}
			t.Fatal("no completed audit state was persisted")
		})
	}
}
