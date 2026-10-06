// Package cliroutetest holds the legacy dispatch-plan scenarios and golden records that pin today's CLI routing.
package cliroutetest

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/envchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/repostate"
)

const (
	ResolverRunner      = "runner"
	ResolverBridgechain = "bridgechain"
	DefaultModel        = "balanced"
)

var fixedNow = time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)

func Now() time.Time { return fixedNow }

type Variant struct {
	Name            string
	Env             map[string]string
	AgentCLI        string
	Pin             *policy.Pin
	ExcludeNoFamily bool
	Bypass          bool
	Overlay         llmroute.Overlay
	CallerCLI       string
	Router          *policy.RouterPolicy
	Installed       []string
	Discovered      []string
	BenchedFamily   string
}

func Variants() []Variant {
	everyBinary := []string{"claude", "codex", "agy", "ollama"}
	toolCapable := []string{"claude-tmux", "codex-tmux", "agy-tmux"}
	host := func(v Variant) Variant {
		if v.Installed == nil {
			v.Installed, v.Discovered = everyBinary, toolCapable
		}
		return v
	}
	return []Variant{
		host(Variant{Name: "none"}),
		host(Variant{Name: "env_cli", Env: map[string]string{"EVOLVE_CLI": "claude-p"}}),
		host(Variant{Name: "env_agent", AgentCLI: "agy-tmux"}),
		host(Variant{Name: "pin", Pin: &policy.Pin{CLI: "agy", Model: "deep"}}),
		host(Variant{Name: "pin_model", Pin: &policy.Pin{Model: "balanced"}}),
		host(Variant{Name: "advisor", Overlay: llmroute.Overlay{CLI: "agy", Tier: "deep"}}),
		host(Variant{Name: "benched", BenchedFamily: "codex"}),
		host(Variant{Name: "missing_binary", Installed: []string{"claude", "agy", "ollama"}, Discovered: []string{"claude-tmux", "agy-tmux"}}),
		host(Variant{Name: "caller_cli", CallerCLI: "claude-p"}),
		host(Variant{Name: "checked_in_tail", ExcludeNoFamily: true}),
		host(Variant{Name: "bypass", Pin: &policy.Pin{CLI: "agy", Model: "deep"}, ExcludeNoFamily: true, Bypass: true}),
		host(Variant{Name: "router_keys", Router: &policy.RouterPolicy{CLI: "claude-tmux", Model: "balanced", PlanModel: "top"}}),
	}
}

func (v Variant) EnvFor(agent string) map[string]string {
	env := make(map[string]string, len(v.Env)+1)
	for k, val := range v.Env {
		env[k] = val
	}
	if v.AgentCLI != "" {
		env[envchain.PhaseEnvKey(agent, "CLI")] = v.AgentCLI
	}
	return env
}

func (v Variant) LookPath(bin string) (string, error) {
	for _, installed := range v.Installed {
		if installed == bin {
			return filepath.Join("/fake/bin", bin), nil
		}
	}
	return "", exec.ErrNotFound
}

func (v Variant) PathDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, bin := range v.Installed {
		if err := os.WriteFile(filepath.Join(dir, bin), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatalf("stub %s: %v", bin, err)
		}
	}
	return dir
}

func (v Variant) ProductionDiscover(p policy.Policy) func() []string {
	wf := p.WorkflowConfig()
	if !wf.UniversalFallback {
		return nil
	}
	tail := llmroute.ExcludeFamilies(v.Discovered, wf.UniversalFallbackExclude)
	return func() []string { return tail }
}

func (v Variant) RawDiscover() []string { return v.Discovered }

func (v Variant) ProjectRoot(t *testing.T, phases []string) string {
	t.Helper()
	root := t.TempDir()
	if doc := v.policyDoc(phases); doc != nil {
		writePolicy(t, root, doc)
	}
	if v.BenchedFamily != "" {
		entry := clihealth.Entry{
			Family: v.BenchedFamily, Reason: "rate_limit", Strikes: 1,
			BenchedAt: fixedNow.Add(-10 * time.Minute), BenchedUntil: fixedNow.Add(2 * time.Hour),
		}
		if err := clihealth.NewStore(root, Now).Bench(entry); err != nil {
			t.Fatalf("bench %s: %v", v.BenchedFamily, err)
		}
	}
	return root
}

func (v Variant) Policy(t *testing.T, root string) policy.Policy {
	t.Helper()
	p, err := policy.Load(filepath.Join(root, ".evolve", "policy.json"))
	if err != nil {
		t.Fatalf("load variant %s policy: %v", v.Name, err)
	}
	return p
}

func (v Variant) policyDoc(phases []string) map[string]any {
	doc := map[string]any{}
	if v.Pin != nil {
		pins := make(map[string]policy.Pin, len(phases))
		for _, ph := range phases {
			pins[ph] = *v.Pin
		}
		doc["pins"] = pins
	}
	if v.ExcludeNoFamily {
		doc["workflow"] = map[string]any{"universal_fallback_exclude": []string{}}
	}
	if v.Router != nil {
		doc["router"] = v.Router
	}
	if len(doc) == 0 {
		return nil
	}
	return doc
}

func writePolicy(t *testing.T, root string, doc map[string]any) {
	t.Helper()
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal policy: %v", err)
	}
	dir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "policy.json"), raw, 0o644); err != nil {
		t.Fatalf("write policy: %v", err)
	}
}

func PhaseOf(agent string) string {
	var phases []string
	for _, c := range phasecontract.Contracts() {
		if c.AgentName == agent {
			phases = append(phases, c.Phase)
		}
	}
	sort.Strings(phases)
	for _, ph := range phases {
		if ph == agent {
			return ph
		}
	}
	if len(phases) > 0 {
		return phases[0]
	}
	return agent
}

func PhasesOf(agents []string) []string {
	out := make([]string, 0, len(agents))
	for _, a := range agents {
		out = append(out, PhaseOf(a))
	}
	return out
}

func RepoRoot(t *testing.T) string {
	t.Helper()
	_, self, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(self), "..", "..", "..", ".."))
}

func TrackedProfiles(t *testing.T) (string, []string) {
	t.Helper()
	root := RepoRoot(t)
	dir := filepath.Join(root, ".evolve", "profiles")
	names, err := profiles.NewFromDir(dir).List()
	if err != nil {
		t.Fatalf("list profiles in %s: %v", dir, err)
	}
	tracked, err := repostate.TrackedSet(root, ".evolve/profiles", ".json")
	if err != nil || len(tracked) == 0 {
		t.Fatalf("tracked profiles under %s: %v (set of %d) — the golden binds only tracked profiles", root, err, len(tracked))
	}
	kept := make([]string, 0, len(names))
	for _, n := range names {
		if tracked[n] {
			kept = append(kept, n)
		}
	}
	return dir, kept
}

type Record struct {
	Resolver      string              `json:"resolver"`
	Variant       string              `json:"variant"`
	Agent         string              `json:"agent"`
	Phase         string              `json:"phase,omitempty"`
	Candidates    []string            `json:"candidates,omitempty"`
	Triggers      []int               `json:"triggers,omitempty"`
	PrimarySource string              `json:"primary_source,omitempty"`
	Model         string              `json:"model,omitempty"`
	Tiers         []string            `json:"tiers,omitempty"`
	TierCeiling   map[string][]string `json:"tier_ceiling,omitempty"`
	Error         string              `json:"error,omitempty"`
	Healthy       *bool               `json:"healthy,omitempty"`
}

func PlanRecord(resolver, variant, agent, phase string, plan llmroute.Plan) Record {
	return Record{
		Resolver: resolver, Variant: variant, Agent: agent, Phase: phase,
		Candidates: plan.Candidates, Triggers: plan.Triggers, PrimarySource: plan.PrimarySource,
		Model: plan.Model, Tiers: plan.Tiers, TierCeiling: plan.TierCeiling,
	}
}

func ErrorRecord(resolver, variant, agent, phase, message string) Record {
	return Record{Resolver: resolver, Variant: variant, Agent: agent, Phase: phase, Error: message}
}

func GoldenPath(t *testing.T) string {
	t.Helper()
	return testdataPath(t, "legacy-plans.golden.jsonl")
}

func AdvisorGoldenPath(t *testing.T) string {
	t.Helper()
	return testdataPath(t, "legacy-advisor.golden.jsonl")
}

func testdataPath(t *testing.T, name string) string {
	t.Helper()
	_, self, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(self), "..", "testdata", name)
}

func Encode(records []Record) ([]byte, error) {
	var buf bytes.Buffer
	for _, r := range records {
		line, err := json.Marshal(r)
		if err != nil {
			return nil, err
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}
	return buf.Bytes(), nil
}

func ReadGolden(t *testing.T) []Record {
	t.Helper()
	raw, err := os.ReadFile(GoldenPath(t))
	if err != nil {
		t.Fatalf("read golden: %v (regenerate with go test ./internal/phases/runner -run TestLegacyDispatchPlans -update)", err)
	}
	var out []Record
	for _, line := range bytes.Split(bytes.TrimSuffix(raw, []byte("\n")), []byte("\n")) {
		var r Record
		if err := json.Unmarshal(line, &r); err != nil {
			t.Fatalf("golden line %q: %v", line, err)
		}
		out = append(out, r)
	}
	return out
}

func AssertGolden(t *testing.T, records []Record, update bool) {
	t.Helper()
	AssertGoldenAt(t, GoldenPath(t), records, update)
}

func AssertGoldenAt(t *testing.T, path string, records []Record, update bool) {
	t.Helper()
	got, err := Encode(records)
	if err != nil {
		t.Fatalf("encode records: %v", err)
	}
	if update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden: %v (regenerate with -update)", err)
	}
	if line, ok := firstDifference(got, want); ok {
		t.Fatalf("dispatch plans drifted from %s; first differing record:\n got: %s\nwant: %s", path, line[0], line[1])
	}
}

func firstDifference(got, want []byte) ([2]string, bool) {
	g, w := bytes.Split(got, []byte("\n")), bytes.Split(want, []byte("\n"))
	for i := 0; i < len(g) || i < len(w); i++ {
		gl, wl := lineAt(g, i), lineAt(w, i)
		if gl != wl {
			return [2]string{gl, wl}, true
		}
	}
	return [2]string{}, false
}

func lineAt(lines [][]byte, i int) string {
	if i < len(lines) {
		return string(lines[i])
	}
	return "<missing>"
}

var ErrDeclineAuto = errors.New("cliroutetest: auto expansion declined")
