package selfheal

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gateclass"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type filedRepair struct {
	raw       []byte
	tier      gateclass.Tier
	protected []string
	calls     int
}

func (f *filedRepair) file(raw []byte, tier gateclass.Tier, protected []string) error {
	f.raw, f.tier, f.protected = raw, tier, protected
	f.calls++
	return nil
}

func repairAction(files ...string) phasecontract.DiagnosisAction {
	return phasecontract.DiagnosisAction{Kind: phasecontract.ActionOpenRepairItem, Params: phasecontract.DiagnosisParams{Files: files, Repro: "the boundary hangs"}}
}

func guardedPath(p string) bool { return strings.HasPrefix(p, "go/internal/guards/") }

func TestOpenRepairItem_FilesAConsoleItemWithTheTierOfTheIncident(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		tier      string
		files     []string
		wantTier  gateclass.Tier
		wantStamp gateclass.Tier
		wantRoute string
		protected []string
	}{
		"T1 incident, unprotected file":        {TierPipeline, []string{"go/internal/gc/sweep.go"}, gateclass.T1, gateclass.T1, "lane", nil},
		"T2 incident":                          {TierItem, []string{"go/internal/gc/sweep.go"}, gateclass.T2, gateclass.T2, "console", nil},
		"T0 incident, protected file":          {TierKernel, []string{"go/internal/gc/sweep.go", "go/internal/guards/role.go"}, gateclass.T0, "", "console", []string{"go/internal/guards/role.go"}},
		"T0 incident, no protected file named": {TierKernel, []string{"go/internal/gc/sweep.go"}, gateclass.T0, "", "console", []string{"go/internal/gc/sweep.go"}},
		"T1 incident, a protected file":        {TierPipeline, []string{"go/internal/guards/role.go"}, gateclass.T0, gateclass.T1, "console", []string{"go/internal/guards/role.go"}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var f filedRepair
			inc := Incident{ID: "inc-2026-10-27-0a1b2c3d", Class: ClassStall, Tier: c.tier, Summary: "the boundary hangs"}

			got, err := OpenRepairItem(inc, repairAction(c.files...), RepairPlan{Stage: "shadow", IsProtected: guardedPath, AuditRefused: true}, f.file)

			if err != nil || f.calls != 1 {
				t.Fatalf("err=%v calls=%d", err, f.calls)
			}
			if got.Tier != c.wantTier || got.WouldRoute != c.wantRoute || !got.WouldRefuseAudit || got.ItemID != "repair-inc-2026-10-27-0a1b2c3d" || got.Stage != "shadow" {
				t.Errorf("routed = %+v", got)
			}
			if f.tier != c.wantStamp || strings.Join(f.protected, ",") != strings.Join(c.protected, ",") {
				t.Errorf("stamp tier %q protected %v, want %q %v", f.tier, f.protected, c.wantStamp, c.protected)
			}
			var it inboxbatch.Item
			if err := json.Unmarshal(f.raw, &it); err != nil || it.ID != got.ItemID || it.Kind != inboxbatch.KindPipelineRepair || len(it.Files) != len(c.files) || len(it.Acceptance) == 0 {
				t.Errorf("filed item = %+v %v", it, err)
			}
		})
	}
}

func TestOpenRepairItem_RefusesAnActionThatIsNotAnOpenRepairItem(t *testing.T) {
	t.Parallel()
	var f filedRepair
	_, err := OpenRepairItem(Incident{Tier: TierPipeline}, phasecontract.DiagnosisAction{Kind: phasecontract.ActionHaltAndEscalate}, RepairPlan{}, f.file)
	if !errors.Is(err, ErrNotRepairAction) || f.calls != 0 {
		t.Fatalf("err=%v calls=%d", err, f.calls)
	}
}

func TestOpenRepairItem_ReturnsTheFilingError(t *testing.T) {
	t.Parallel()
	boom := errors.New("inbox is full")
	_, err := OpenRepairItem(Incident{Tier: TierPipeline}, repairAction("a.go"), RepairPlan{}, func([]byte, gateclass.Tier, []string) error { return boom })
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
}

func TestRepairRouted_EventCarriesTheShadowRecord(t *testing.T) {
	t.Parallel()
	r := RepairRouted{IncidentID: "inc-1", ItemID: "repair-inc-1", Tier: gateclass.T1, Stage: "shadow", WouldRoute: "lane", WouldRefuseAudit: true}

	e := r.Event()

	want := map[string]string{"incident": "inc-1", "item": "repair-inc-1", "tier": "T1", "stage": "shadow", "would_route": "lane", "would_refuse_audit": "true"}
	for k, v := range want {
		if e.Fields[k] != v {
			t.Errorf("field %s = %q, want %q", k, e.Fields[k], v)
		}
	}
	if e.Code != CodeRepairRouted || e.Module != signalcenter.ModuleHealing || e.Severity != signalcenter.SeverityInfo {
		t.Errorf("event = %+v", e)
	}
}

func TestOpenRepairItem_RefusesAnIncidentWhoseTierIsNotKnown(t *testing.T) {
	t.Parallel()
	for _, tier := range []string{"", "T9", "t1"} {
		var f filedRepair
		_, err := OpenRepairItem(Incident{ID: "inc-1", Tier: tier}, repairAction("a.go"), RepairPlan{}, f.file)
		if !errors.Is(err, ErrUnknownTier) || f.calls != 0 {
			t.Errorf("tier %q: err=%v calls=%d, want ErrUnknownTier and no item", tier, err, f.calls)
		}
	}
}

func TestOpenRepairItem_RefusesAnActionThatNamesNoFile(t *testing.T) {
	t.Parallel()
	var f filedRepair
	_, err := OpenRepairItem(Incident{ID: "inc-1", Tier: TierKernel}, repairAction(), RepairPlan{}, f.file)
	if !errors.Is(err, ErrNoRepairFiles) || f.calls != 0 {
		t.Fatalf("err=%v calls=%d", err, f.calls)
	}
}
