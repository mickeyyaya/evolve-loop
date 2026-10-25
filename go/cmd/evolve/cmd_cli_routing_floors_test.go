package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestCLIRoutingSet_FloorsWritesTheAgentsOfOneFamily(t *testing.T) {
	root := routingWriteProject(t, `{"cli_routing": {"clis": ["agy", "claude"], "default": ["agy", "claude"]}}`)

	rc, _, stderr := runRoutingVerb(t, inRoot(root, "set", "floors.claude", "auditor,router")...)

	if got := mustReadBlock(t, root).Floors; rc != 0 || !reflect.DeepEqual(got, policy.Floors{"claude": {"auditor", "router"}}) {
		t.Fatalf("rc=%d stderr=%q floors=%v", rc, stderr, got)
	}
}

func TestCLIRoutingSet_AFloorOnAFamilyOutsideClisIsRefused(t *testing.T) {
	root := routingWriteProject(t, `{"cli_routing": {"clis": ["agy", "claude"], "default": ["agy", "claude"]}}`)

	rc, _, stderr := runRoutingVerb(t, inRoot(root, "set", "floors.codex", "auditor")...)

	if rc != exitRoutingFinding || !strings.Contains(stderr, "codex must be listed") || mustReadBlock(t, root).Floors != nil {
		t.Fatalf("rc=%d stderr=%q: a floor on a family outside clis must not be written", rc, stderr)
	}
}

func TestCLIRoutingSet_FloorsTakeNoModelOrEffort(t *testing.T) {
	root := routingWriteProject(t, `{"cli_routing": {"clis": ["agy", "claude"], "default": ["agy", "claude"]}}`)
	for _, flag := range [][]string{{"--model", "deep"}, {"--effort", "high"}} {
		args := append([]string{"set", "floors.claude", "auditor"}, flag...)
		if rc, _, stderr := runRoutingVerb(t, inRoot(root, args...)...); rc != exitRoutingFinding || !strings.Contains(stderr, "not floors.claude") {
			t.Errorf("%v: rc=%d stderr=%q, want a refusal that names the key", flag, rc, stderr)
		}
	}
}

func TestCLIRoutingUnset_FloorsRemovesOneFamilyOrTheWholeKey(t *testing.T) {
	root := routingWriteProject(t, `{"cli_routing": {"clis": ["agy", "claude"], "default": ["agy", "claude"], "floors": {"claude": ["auditor"], "agy": ["router"]}}}`)

	if rc, _, stderr := runRoutingVerb(t, inRoot(root, "unset", "floors.agy")...); rc != 0 {
		t.Fatalf("unset floors.agy: rc=%d stderr=%q", rc, stderr)
	}
	if got := mustReadBlock(t, root).Floors; !reflect.DeepEqual(got, policy.Floors{"claude": {"auditor"}}) {
		t.Fatalf("floors = %v, want only the claude floor", got)
	}
	if rc, _, stderr := runRoutingVerb(t, inRoot(root, "unset", "floors.claude")...); rc != 0 {
		t.Fatalf("unset floors.claude: rc=%d stderr=%q", rc, stderr)
	}
	if got := mustReadBlock(t, root).Floors; got == nil || len(got) != 0 {
		t.Fatalf("removing the last floor family writes floors as {} (no floor), not the compiled default: %v", got)
	}
	if rc, _, stderr := runRoutingVerb(t, inRoot(root, "unset", "floors")...); rc != 0 || mustReadBlock(t, root).Floors != nil {
		t.Fatalf("unset floors: rc=%d stderr=%q", rc, stderr)
	}
	if rc, _, stderr := runRoutingVerb(t, inRoot(root, "unset", "floors.codex")...); rc != exitRoutingFinding || !strings.Contains(stderr, "floors.codex is not set") {
		t.Fatalf("unset of an absent floor: rc=%d stderr=%q", rc, stderr)
	}
}

func TestCLIRoutingSet_AnEmptyFloorsKeySurvivesAnUnrelatedWrite(t *testing.T) {
	root := routingWriteProject(t, `{"cli_routing": {"clis": ["agy", "claude"], "default": ["agy", "claude"], "floors": {}}}`)

	rc, _, stderr := runRoutingVerb(t, inRoot(root, "set", "default", "claude,agy")...)

	if got := mustReadBlock(t, root).Floors; rc != 0 || got == nil || len(got) != 0 {
		t.Fatalf("rc=%d stderr=%q floors=%v: a declared empty floors key must stay {} after a write", rc, stderr, got)
	}
}
