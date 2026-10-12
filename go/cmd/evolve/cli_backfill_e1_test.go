package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestCanary_ACancelDuringTheProbeLeavesTheBenchAndStopsTheSweep(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	benchExpired(t, root, "codex", 1)
	benchExpired(t, root, "agy", 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	probes := 0
	var out bytes.Buffer

	runCLIHealthCanary(ctx, root, nil, func(string) (int, string, string) {
		probes++
		cancel()
		return 0, "", ""
	}, &out)

	if probes != 1 {
		t.Errorf("probes = %d, want 1: a cancel mid-sweep stops before the next family", probes)
	}
	benches, err := clihealth.NewStore(root, nil).Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(benches) != 2 {
		t.Errorf("benches = %v, want both untouched: a probe that ends under a cancel clears nothing", benches)
	}
	if !strings.Contains(out.String(), "probe — bench untouched") || !strings.Contains(out.String(), "cancelled (context canceled) during the") {
		t.Errorf("stderr = %q, want the mid-probe cancel line", out.String())
	}
}

func TestMigrateLegacyKeys_RefusesAPinOrRouterThatTheTableAlreadyRoutes(t *testing.T) {
	t.Parallel()
	taken := policy.CLIRouting{Agents: map[string]policy.AgentRule{"scout": {CLI: []string{"claude"}}, "router": {CLI: []string{"codex"}}}}
	cases := []struct {
		name    string
		pol     policy.Policy
		block   policy.CLIRouting
		wantErr string
	}{
		{"pin collides with an agent rule", policy.Policy{Pins: map[string]policy.Pin{"scout": {CLI: "codex", Model: "deep"}}}, taken,
			"pins.scout and agents.scout both route scout: keep one"},
		{"router.cli collides with agents.router", policy.Policy{Router: &policy.RouterPolicy{CLI: "claude", Model: "fast"}}, taken,
			"router.cli and agents.router both route the router: keep one"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			patch, err := migrateLegacyKeys([]byte(`{"router":{"cli":"claude"}}`), tc.pol, tc.block)
			if err == nil || err.Error() != tc.wantErr {
				t.Fatalf("err = %v, want %q", err, tc.wantErr)
			}
			if patch != nil {
				t.Errorf("patch = %v, want nil on a refusal", patch)
			}
			if got := taken.Agents["scout"].CLI; !reflect.DeepEqual(got, []string{"claude"}) {
				t.Errorf("the caller's block was mutated: scout = %v", got)
			}
		})
	}
}

func TestMigrateLegacyKeys_MovesAFreePinAndRouterIntoAgentRules(t *testing.T) {
	t.Parallel()
	existing := []byte(`{"router":{"cli":"codex","model":"deep","replan_depth":2}}`)
	pol := policy.Policy{
		Pins:   map[string]policy.Pin{"audit": {CLI: "agy", Model: "fast"}},
		Router: &policy.RouterPolicy{CLI: "codex", Model: "deep"},
	}

	patch, err := migrateLegacyKeys(existing, pol, policy.CLIRouting{})

	if err != nil {
		t.Fatal(err)
	}
	block := patch["cli_routing"].(policy.CLIRouting)
	want := map[string]policy.AgentRule{
		"audit":  {CLI: []string{"agy"}, Model: "fast"},
		"router": {CLI: []string{"codex"}, Model: "deep"},
	}
	if !reflect.DeepEqual(block.Agents, want) {
		t.Errorf("agents = %v, want %v", block.Agents, want)
	}
	if v, ok := patch["pins"]; !ok || v != nil {
		t.Errorf("pins patch = %v (present=%v), want an explicit null that deletes the key", v, ok)
	}
	rest, _ := json.Marshal(patch["router"])
	if string(rest) != `{"replan_depth":2}` {
		t.Errorf("router rest = %s, want only the unmigrated key", rest)
	}
}

func TestBlockWithout(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		existing string
		block    string
		keys     []string
		want     string
		wantErr  string
	}{
		{"malformed document", `{`, "router", nil, "", "unexpected end of JSON input"},
		{"block is not an object", `{"router":5}`, "router", nil, "", "router: json: cannot unmarshal number"},
		{"absent block", `{}`, "router", nil, "", "router: unexpected end of JSON input"},
		{"every key removed yields nil", `{"router":{"cli":"x","model":"y"}}`, "router", []string{"cli", "model"}, "null", ""},
		{"empty block yields nil", `{"router":{}}`, "router", []string{"cli"}, "null", ""},
		{"other keys survive", `{"router":{"cli":"x","replan_depth":1,"ünï":"ç"}}`, "router", []string{"cli", "absent"}, `{"replan_depth":1,"ünï":"ç"}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := blockWithout([]byte(tc.existing), tc.block, tc.keys...)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want it to contain %q", err, tc.wantErr)
				}
				if got != nil {
					t.Errorf("got = %v, want nil beside an error", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			b, _ := json.Marshal(got)
			if string(b) != tc.want {
				t.Errorf("got %s, want %s", b, tc.want)
			}
		})
	}
}

func routingRootPolicy(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestBuildCLIRouter_AMalformedPolicyRefusesWithoutARouter(t *testing.T) {
	t.Parallel()
	root := routingRootPolicy(t, `{"cli_routing":`)

	r, findings, err := buildCLIRouter(root, routingCatalog(root), cliroute.Host{LookPath: everyBinaryPresent})

	if err == nil || !strings.Contains(err.Error(), "policy: parse") {
		t.Fatalf("err = %v, want the policy parse error", err)
	}
	if r != nil || findings != nil {
		t.Errorf("router = %v findings = %v, want nil on a setup failure", r, findings)
	}
}

func TestRootRouter_ARefusalNamesTheVerbAndReturnsNoRouter(t *testing.T) {
	t.Parallel()
	root := routingRootPolicy(t, `{"cli_routing":`)
	var stderr bytes.Buffer

	r, ok := rootRouter(paths.Layout{ProjectRoot: root}, "evolve subagent run", &bytes.Buffer{}, &stderr)

	if ok || r != nil {
		t.Fatalf("rootRouter = (%v, %v), want (nil, false)", r, ok)
	}
	if !strings.HasPrefix(stderr.String(), "evolve subagent run: policy: parse") {
		t.Errorf("stderr = %q, want the verb-prefixed refusal", stderr.String())
	}
}

func TestRootRouter_AFindingRefusalPrintsEachFindingBeforeTheVerbLine(t *testing.T) {
	t.Parallel()
	root := routingRootPolicy(t, `{"cli_routing":{"clis":["no-such-cli"],"default":["no-such-cli"]}}`)
	var stderr bytes.Buffer

	r, ok := rootRouter(paths.Layout{ProjectRoot: root}, "verb-x", &bytes.Buffer{}, &stderr)

	if ok || r != nil {
		t.Fatalf("rootRouter = (%v, %v), want a refusal", r, ok)
	}
	out := stderr.String()
	finding, verb := strings.Index(out, "[cli-routing] "), strings.Index(out, "verb-x: ")
	if finding < 0 || verb < 0 || finding > verb {
		t.Errorf("stderr = %q, want the findings first, then the verb line", out)
	}
}

func TestRootRouter_AnEmptyProjectRoutes(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer

	r, ok := rootRouter(paths.Layout{ProjectRoot: t.TempDir()}, "verb-x", &bytes.Buffer{}, &stderr)

	if !ok || r == nil {
		t.Fatalf("rootRouter = (%v, %v), want a router; stderr=%q", r, ok, stderr.String())
	}
	if strings.Contains(stderr.String(), "verb-x:") {
		t.Errorf("stderr = %q, want no refusal line", stderr.String())
	}
}

func usageProbeFakePATH(t *testing.T, names ...string) {
	t.Helper()
	dir := t.TempDir()
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
}

func TestRunUsageProbe_NoInstalledFamilyProbesNothing(t *testing.T) {
	usageProbeFakePATH(t)
	evolveDir := writeProbePolicy(t, `{"cli_health":{"proactive_probe":true}}`)
	var stderr bytes.Buffer

	runUsageProbe(context.Background(), t.TempDir(), evolveDir, nil, &stderr)

	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want silence with no CLI on PATH", stderr.String())
	}
}

func TestRunUsageProbe_ProbesEveryInstalledFamilyAndBenchesNoneWithoutAUsageControl(t *testing.T) {
	usageProbeFakePATH(t, "ollama")
	root := t.TempDir()
	evolveDir := writeProbePolicy(t, `{"cli_health":{"proactive_probe":true}}`)
	var stderr bytes.Buffer

	runUsageProbe(context.Background(), root, evolveDir, nil, &stderr)

	if !strings.Contains(stderr.String(), "[loop] usage-probe: checking [ollama] for quota caps before dispatch") {
		t.Errorf("stderr = %q, want the probe announcement naming ollama", stderr.String())
	}
	if active := clihealth.NewStore(root, nil).Active(); len(active) != 0 {
		t.Errorf("benched = %v, want none: an unsupported usage control fails open", active)
	}
}

func TestRunUsageProbe_DisabledByPolicyDoesNotLookForFamilies(t *testing.T) {
	usageProbeFakePATH(t, "ollama")
	var stderr bytes.Buffer

	runUsageProbe(context.Background(), t.TempDir(), writeProbePolicy(t, `{}`), nil, &stderr)

	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want silence when the policy leaves the probe off", stderr.String())
	}
}

func TestProbeWaveQuota_NoInstalledFamilyReturnsNil(t *testing.T) {
	usageProbeFakePATH(t)
	var stderr bytes.Buffer

	got := probeWaveQuota(context.Background(), t.TempDir(), t.TempDir(), time.Now(), &stderr)

	if got != nil || stderr.Len() != 0 {
		t.Errorf("probeWaveQuota = %v stderr=%q, want nil and silence", got, stderr.String())
	}
}

func TestProbeWaveQuota_AFamilyWithoutAUsageControlIsOmitted(t *testing.T) {
	usageProbeFakePATH(t, "ollama")
	var stderr bytes.Buffer

	got := probeWaveQuota(context.Background(), t.TempDir(), t.TempDir(), time.Now(), &stderr)

	if got == nil || len(got) != 0 {
		t.Errorf("probeWaveQuota = %#v, want an empty non-nil slice", got)
	}
	if !strings.Contains(stderr.String(), "[budget] probing [ollama] for quota before wave sizing") {
		t.Errorf("stderr = %q, want the probe announcement", stderr.String())
	}
}
