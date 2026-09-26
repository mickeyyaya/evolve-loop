package fleet

import (
	"io"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

func TestFreshenSpecs_PrunedSpecRewritesFleetScopeEnv(t *testing.T) {
	specs := []CycleSpec{{
		Scope: []string{"live-a", "stale-b", "live-c"},
		Env:   map[string]string{ipcenv.FleetScopeKey: "live-a,stale-b,live-c"},
	}}
	probe := func(id string) TaskFreshness {
		if id == "stale-b" {
			return TaskFreshness{Fresh: false, Reason: "consumed: processed"}
		}
		return TaskFreshness{Fresh: true}
	}
	kept, _ := FreshenSpecs(specs, probe, func(map[string]bool) (CycleSpec, bool) { return CycleSpec{}, false }, io.Discard)
	if len(kept) != 1 {
		t.Fatalf("kept = %d, want 1", len(kept))
	}
	got := kept[0].Env[ipcenv.FleetScopeKey]
	if strings.Contains(got, "stale-b") {
		t.Errorf("env scope still carries the pruned id: %q — the lane would pin stale-b in lane-scope.json as authoritative scope", got)
	}
	if got != "live-a,live-c" {
		t.Errorf("env scope = %q, want live-a,live-c (rebuilt from the pruned Scope)", got)
	}
	if specs[0].Env[ipcenv.FleetScopeKey] != "live-a,stale-b,live-c" {
		t.Errorf("input spec Env mutated in place: %q", specs[0].Env[ipcenv.FleetScopeKey])
	}
}

func TestFreshenSpecs_PrunedSpecDropsStaleContractLine(t *testing.T) {
	specs := []CycleSpec{{
		Scope:          []string{"live-a", "stale-b"},
		OutputContract: "[live-a] ship the live thing\n[stale-b] ship the dead thing",
		Env:            map[string]string{ipcenv.FleetScopeKey: "live-a,stale-b"},
	}}
	probe := func(id string) TaskFreshness {
		if id == "stale-b" {
			return TaskFreshness{Fresh: false, Reason: "consumed: processed"}
		}
		return TaskFreshness{Fresh: true}
	}
	kept, _ := FreshenSpecs(specs, probe, func(map[string]bool) (CycleSpec, bool) { return CycleSpec{}, false }, io.Discard)
	if len(kept) != 1 {
		t.Fatalf("kept = %d, want 1", len(kept))
	}
	got := kept[0].OutputContract
	if strings.Contains(got, "stale-b") || strings.Contains(got, "dead thing") {
		t.Errorf("contract still instructs the lane to deliver the pruned todo: %q", got)
	}
	if !strings.Contains(got, "[live-a] ship the live thing") {
		t.Errorf("contract lost the LIVE todo's objective: %q", got)
	}
	if specs[0].OutputContract != "[live-a] ship the live thing\n[stale-b] ship the dead thing" {
		t.Errorf("input spec contract mutated in place: %q", specs[0].OutputContract)
	}
}

func TestFreshenSpecs_MultiLineStaleContractFullyDropped(t *testing.T) {
	specs := []CycleSpec{{
		Scope:          []string{"stale-a", "live-b"},
		OutputContract: "[stale-a] do X\nand Y across two lines\n[live-b] do Z\nplus its second line",
		Env:            map[string]string{ipcenv.FleetScopeKey: "stale-a,live-b"},
	}}
	probe := func(id string) TaskFreshness {
		if id == "stale-a" {
			return TaskFreshness{Fresh: false, Reason: "consumed: processed"}
		}
		return TaskFreshness{Fresh: true}
	}
	kept, _ := FreshenSpecs(specs, probe, func(map[string]bool) (CycleSpec, bool) { return CycleSpec{}, false }, io.Discard)
	if len(kept) != 1 {
		t.Fatalf("kept = %d, want 1", len(kept))
	}
	got := kept[0].OutputContract
	if strings.Contains(got, "and Y across two lines") {
		t.Errorf("stale todo's CONTINUATION line survived the prune: %q", got)
	}
	if got != "[live-b] do Z\nplus its second line" {
		t.Errorf("contract = %q, want the live todo's full multi-line objective and nothing else", got)
	}
}
