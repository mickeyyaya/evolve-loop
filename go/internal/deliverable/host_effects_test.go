package deliverable

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

type claimCall struct {
	inboxDir string
	cycle    int
	ids      []string
}

func recordingClaimer(calls *[]claimCall) Claimer {
	return func(inboxDir string, cycle int, ids []string) error {
		*calls = append(*calls, claimCall{inboxDir, cycle, append([]string(nil), ids...)})
		return nil
	}
}

func triageInput(t *testing.T, root, pin, decision string) core.ReviewInput {
	t.Helper()
	ws := t.TempDir()
	if pin != "" {
		writeFile(t, ws, "lane-scope.json", pin)
	}
	if decision != "" {
		writeFile(t, ws, "triage-decision.json", decision)
	}
	return core.ReviewInput{Phase: "triage", Cycle: 7, Workspace: ws, ProjectRoot: root}
}

func TestHostEffects_ClaimsExactlyWhatTheCheckOwes(t *testing.T) {
	for _, tc := range []struct {
		name, phase, pin, decision string
		want                       []string
	}{
		{name: "top_n minus deferred", phase: "triage", decision: `{"top_n":[{"id":"a"},{"id":"b"}],"deferred":[{"id":"b"}]}`, want: []string{"a"}},
		{name: "a lane pin is the commitment", phase: "triage", pin: `{"todo_ids":["x"],"goal_hash":"g"}`, decision: `{"top_n":[]}`, want: []string{"x"}},
		{name: "nothing recorded", phase: "triage"},
		{name: "an empty commitment", phase: "triage", decision: `{"top_n":[]}`},
		{name: "a phase without the effect", phase: "build", decision: `{"top_n":[{"id":"a"}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls []claimCall
			in := triageInput(t, "/project", tc.pin, tc.decision)
			in.Phase = tc.phase
			if err := NewHostEffects(effectCatalog(t), recordingClaimer(&calls)).Perform(context.Background(), in); err != nil {
				t.Fatalf("Perform: %v", err)
			}
			if tc.want == nil {
				if len(calls) != 0 {
					t.Fatalf("no claim expected, got %+v", calls)
				}
				return
			}
			want := claimCall{filepath.Join("/project", ".evolve", "inbox"), 7, tc.want}
			if len(calls) != 1 || !reflect.DeepEqual(calls[0], want) {
				t.Fatalf("claims = %+v, want [%+v]", calls, want)
			}
		})
	}
}

func TestHostEffects_TheDeclarationIsConfig(t *testing.T) {
	cat, warnings := (phasespec.Catalog{}).Merge([]phasespec.PhaseSpec{
		{Name: "triage", Role: "triage", Outputs: phasespec.IO{Files: []string{".evolve/runs/cycle-{cycle}/triage-report.md"}}},
		{Name: "user-selector", Outputs: phasespec.IO{Files: []string{".evolve/runs/cycle-{cycle}/user-selector-report.md"}}, Effects: []string{"inbox-claim"}},
	})
	if len(warnings) != 0 {
		t.Fatal(warnings)
	}
	var calls []claimCall
	var effects core.HostEffects = NewHostEffects(cat, recordingClaimer(&calls))
	in := triageInput(t, "/project", "", `{"top_n":[{"id":"a"}]}`)
	if err := effects.Perform(context.Background(), in); err != nil || len(calls) != 0 {
		t.Fatalf("triage without the declaration: err=%v claims=%+v", err, calls)
	}
	in.Phase = "user-selector"
	if err := effects.Perform(context.Background(), in); err != nil || len(calls) != 1 {
		t.Fatalf("the declaring user phase: err=%v claims=%+v", err, calls)
	}
}

func TestHostEffects_WithoutACycleIsAnError(t *testing.T) {
	var calls []claimCall
	in := triageInput(t, "/project", "", `{"top_n":[{"id":"a"}]}`)
	in.Cycle = 0
	if err := NewHostEffects(effectCatalog(t), recordingClaimer(&calls)).Perform(context.Background(), in); err == nil || len(calls) != 0 {
		t.Fatalf("a claim without the cycle must fail loudly and claim nothing: err=%v claims=%+v", err, calls)
	}
}

func TestEffects_EveryEntryIsVerified(t *testing.T) {
	for name, e := range effects {
		if e.check == nil {
			t.Errorf("effect %q has no check: a host-performed effect the gate does not verify could pass silently", name)
		}
	}
}

func TestHostEffects_WithoutAProjectRootIsAnError(t *testing.T) {
	var calls []claimCall
	in := triageInput(t, "", "", `{"top_n":[{"id":"a"}]}`)
	if err := NewHostEffects(effectCatalog(t), recordingClaimer(&calls)).Perform(context.Background(), in); err == nil || len(calls) != 0 {
		t.Fatalf("a claim with no project root must fail loudly and claim nothing: err=%v claims=%+v", err, calls)
	}
}
