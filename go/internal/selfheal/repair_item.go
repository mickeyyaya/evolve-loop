package selfheal

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/gateclass"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	CodeRepairRouted signalcenter.Code = "HEALING_REPAIR_ROUTED"

	repairItemPrefix = "repair-"
	repairClass      = "stability"
	repairWeight     = 0.4
)

var (
	ErrNotRepairAction = errors.New("the action is not open-repair-item")
	ErrUnknownTier     = errors.New("the incident tier is not T0, T1 or T2")
	ErrNoRepairFiles   = errors.New("the repair action names no file")
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleHealing, CodeRepairRouted, "an accepted open-repair-item proposal was filed as a console-routed pipeline-repair item; fields incident, item, tier, stage, would_route (lane for a T1 item once the lane route ships, console for T0 and T2), would_refuse_audit (no other family can judge the builder) (llm-isolation.md section 20.2)")
}

type RepairFiler func(raw []byte, tier gateclass.Tier, protected []string) error

type RepairPlan struct {
	Stage        string
	IsProtected  func(string) bool
	AuditRefused bool
}

type RepairRouted struct {
	IncidentID       string
	ItemID           string
	Tier             gateclass.Tier
	Stage            string
	WouldRoute       string
	WouldRefuseAudit bool
}

func OpenRepairItem(inc Incident, a phasecontract.DiagnosisAction, plan RepairPlan, file RepairFiler) (RepairRouted, error) {
	if a.Kind != phasecontract.ActionOpenRepairItem {
		return RepairRouted{}, fmt.Errorf("%w: %q", ErrNotRepairAction, a.Kind)
	}
	if len(a.Params.Files) == 0 {
		return RepairRouted{}, fmt.Errorf("%w: incident %s", ErrNoRepairFiles, inc.ID)
	}
	stamp, err := stampTier(inc.Tier)
	if err != nil {
		return RepairRouted{}, fmt.Errorf("%w: incident %s has tier %q", err, inc.ID, inc.Tier)
	}
	protected := protectedFiles(inc, a.Params.Files, plan.IsProtected)
	raw, id := repairItemJSON(inc, a.Params)
	if err := file(raw, stamp, protected); err != nil {
		return RepairRouted{}, fmt.Errorf("file the repair item %s: %w", id, err)
	}
	tier := inboxbatch.RepairTierOf(string(stamp), protected)
	return RepairRouted{IncidentID: inc.ID, ItemID: id, Tier: tier, Stage: plan.Stage, WouldRoute: wouldRoute(tier), WouldRefuseAudit: plan.AuditRefused}, nil
}

func protectedFiles(inc Incident, files []string, isProtected func(string) bool) []string {
	var out []string
	for _, f := range files {
		if isProtected != nil && isProtected(f) {
			out = append(out, f)
		}
	}
	if len(out) == 0 && inc.Tier == TierKernel {
		return slices.Clone(files)
	}
	return out
}

func stampTier(incidentTier string) (gateclass.Tier, error) {
	switch incidentTier {
	case TierPipeline:
		return gateclass.T1, nil
	case TierItem:
		return gateclass.T2, nil
	case TierKernel:
		return "", nil
	}
	return "", ErrUnknownTier
}

func wouldRoute(t gateclass.Tier) string {
	if t == gateclass.T1 {
		return inboxbatch.RouteLaneValue
	}
	return inboxbatch.RouteConsoleAutoValue
}

func repairItemJSON(inc Incident, p phasecontract.DiagnosisParams) ([]byte, string) {
	id := repairItemPrefix + inc.ID
	raw, _ := json.Marshal(map[string]any{
		"id": id, "kind": inboxbatch.KindPipelineRepair, "priority_class": repairClass, "weight": repairWeight,
		"title":      "Repair the " + string(inc.Class) + " incident " + inc.ID,
		"summary":    inc.Summary + " Reproduction: " + p.Repro,
		"fix":        "Repair the incident in a normal repair cycle: scout, a red test that reproduces it, build, audit by another family, ship.",
		"files":      p.Files,
		"acceptance": []string{"a red test reproduces the incident: " + p.Repro},
	})
	return raw, id
}

func (r RepairRouted) Event() signalcenter.Event { return repairRouted(r) }

func repairRouted(r RepairRouted) signalcenter.Event {
	return signalcenter.Event{
		Module: signalcenter.ModuleHealing, Kind: signalcenter.KindSelfhealDecision, Origin: "selfheal.OpenRepairItem",
		Severity: signalcenter.SeverityInfo, Code: CodeRepairRouted,
		Reason: "incident " + r.IncidentID + " filed as " + r.ItemID + " for the console (would route to " + r.WouldRoute + ")",
		Fields: map[string]string{
			"incident": r.IncidentID, "item": r.ItemID, "tier": string(r.Tier), "stage": r.Stage,
			"would_route": r.WouldRoute, "would_refuse_audit": strconv.FormatBool(r.WouldRefuseAudit),
		},
	}
}
