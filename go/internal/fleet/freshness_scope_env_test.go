package fleet

// freshness_scope_env_test.go — a pruned spec must not launch with its STALE
// env scope. filterScope trims s.Scope but the launcher (execCycleLaunch)
// passes s.Env verbatim, and materializeLaneScope pins lane-scope.json from
// the env CSV — so with multi-id lane menus (fleet-lane-batch-menu) a lane
// whose stale id was pruned would still hand the dead id to every phase as
// authoritative fleet scope. Invisible pre-menus only because single-id specs
// prune to empty and free the slot instead.

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
	// The input spec's Env must not be mutated (shared map).
	if specs[0].Env[ipcenv.FleetScopeKey] != "live-a,stale-b,live-c" {
		t.Errorf("input spec Env mutated in place: %q", specs[0].Env[ipcenv.FleetScopeKey])
	}
}

// TestFreshenSpecs_PrunedSpecDropsStaleContractLine (diff-review HIGH-2): the
// combined OutputContract labels each todo's objective "[id] text" and is
// threaded as goal prose — a lane whose stale id was pruned from Scope/Env
// must not still be INSTRUCTED to deliver the dead todo.
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

// TestFreshenSpecs_MultiLineStaleContractFullyDropped (re-review MEDIUM-A): a
// todo's own contract text may span lines, and only its FIRST line carries the
// "[id] " label — the continuation lines must fall with their label, not
// survive as unattributed instructions for dead work.
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
