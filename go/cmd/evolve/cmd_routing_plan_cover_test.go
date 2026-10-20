package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func TestRouting_NoSubcommandExitsTenWithUsage(t *testing.T) {
	t.Parallel()
	var out, errb bytes.Buffer
	if code := runRouting(nil, nil, &out, &errb); code != 10 || !strings.Contains(errb.String(), "plan") {
		t.Errorf("code=%d stderr=%q, want 10 with usage naming plan", code, errb.String())
	}
}

func TestRoutingSubcommands_BadFlagExitsTen(t *testing.T) {
	t.Parallel()
	for _, sub := range []string{"explain", "replay", "plan"} {
		var out, errb bytes.Buffer
		if code := runRouting([]string{sub, "--no-such-flag"}, nil, &out, &errb); code != 10 {
			t.Errorf("%s with an unknown flag exit = %d, want 10", sub, code)
		}
	}
}

func TestRoutingSubcommands_UnreadableWorkingDirectoryExitsOne(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", "")
	prev := routingGetwd
	routingGetwd = func() (string, error) { return "", errors.New("getwd-refused") }
	t.Cleanup(func() { routingGetwd = prev })
	for _, sub := range []string{"explain", "plan"} {
		var out, errb bytes.Buffer
		if code := runRouting([]string{sub, "--cycle", "1"}, nil, &out, &errb); code != 1 {
			t.Errorf("%s exit = %d, want 1; stderr=%s", sub, code, errb.String())
		}
	}
}

func TestWritePlanEntries_MarksMintedPhases(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	writePlanEntries(&buf, []router.PhasePlanEntry{{Phase: "newphase", Run: true, Mint: &router.MintSpec{}}})
	if got := buf.String(); !strings.Contains(got, "RUN  newphase [minted]") {
		t.Errorf("output = %q, want a minted marker", got)
	}
}

func TestRoutingPlanInput_RefusesBrokenRecordedState(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		setup func(t *testing.T, root string)
		want  string
	}{
		{"unreadable state", func(t *testing.T, root string) {
			if err := os.MkdirAll(filepath.Join(root, ".evolve", "runs", "cycle-7", "cycle-state.json"), 0o755); err != nil {
				t.Fatal(err)
			}
		}, "cycle-state.json"},
		{"unparsable state", func(t *testing.T, root string) {
			writeRaw(t, filepath.Join(root, ".evolve", "runs", "cycle-7", "cycle-state.json"), "{not json")
		}, "parse"},
		{"state of another cycle", func(t *testing.T, root string) {
			writeRaw(t, filepath.Join(root, ".evolve", "runs", "cycle-7", "cycle-state.json"), `{"cycle_id": 8}`)
		}, "records cycle 8"},
		{"broken registry", func(t *testing.T, root string) {
			writeRaw(t, filepath.Join(root, ".evolve", "runs", "cycle-7", "cycle-state.json"), `{"cycle_id": 7}`)
			writeRaw(t, filepath.Join(root, "docs", "architecture", "phase-registry.json"), "{not json")
		}, "phase registry"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			tc.setup(t, root)
			_, err := routingPlanInput(root, 7, "", t.TempDir())
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestRoutingPlan_UnwritableScratchExitsOne(t *testing.T) {
	p := newPlanProject(t)
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))
	var out, errb bytes.Buffer
	if code := runRouting([]string{"plan", "--cycle", "7", "--project-root", p.root}, nil, &out, &errb); code != 1 {
		t.Errorf("exit = %d, want 1; stderr=%s", code, errb.String())
	}
}

func TestWireRoutingPlanAdvisor_WiresAnAdvisorForAProjectRoot(t *testing.T) {
	adv, err := wireRoutingPlanAdvisor(t.TempDir(), io.Discard)
	if err != nil {
		t.Skipf("the bare project root does not wire a CLI router: %v", err)
	}
	if adv == nil {
		t.Fatal("wireRoutingPlanAdvisor returned a nil advisor without an error")
	}
}

func writeRaw(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRoutingPlanInput_LoadsTheAdvisorCatalogFromTheRegistry(t *testing.T) {
	t.Parallel()
	p := newPlanProject(t)
	registry, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "architecture", "phase-registry.json"))
	if err != nil {
		t.Fatal(err)
	}
	writeRaw(t, filepath.Join(p.root, "docs", "architecture", "phase-registry.json"), string(registry))

	in, err := routingPlanInput(p.root, planCycle, "", t.TempDir())

	if err != nil {
		t.Fatalf("routingPlanInput: %v", err)
	}
	if len(in.Catalog) == 0 {
		t.Error("the advisor catalog is empty although the project registry lists phases")
	}
}

func TestWireRoutingPlanAdvisor_RefusesAProjectWhoseRoutingTableDoesNotCompile(t *testing.T) {
	root := t.TempDir()
	writeRaw(t, filepath.Join(root, ".evolve", "policy.json"), `{"cli_routing":{"clis":["no-such-family"],"default":["no-such-family"]}}`)
	if _, err := wireRoutingPlanAdvisor(root, io.Discard); err == nil {
		t.Error("a routing table naming an unknown family wired an advisor")
	}
}
