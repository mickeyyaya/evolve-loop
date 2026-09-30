//go:build acs

package cycle534

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	_ "github.com/mickeyyaya/evolve-loop/go/internal/phases/audit"
	_ "github.com/mickeyyaya/evolve-loop/go/internal/phases/build"
	_ "github.com/mickeyyaya/evolve-loop/go/internal/phases/debugger"
	_ "github.com/mickeyyaya/evolve-loop/go/internal/phases/intent"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
	_ "github.com/mickeyyaya/evolve-loop/go/internal/phases/retro"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
	_ "github.com/mickeyyaya/evolve-loop/go/internal/phases/scout"
	_ "github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	_ "github.com/mickeyyaya/evolve-loop/go/internal/phases/tdd"
	_ "github.com/mickeyyaya/evolve-loop/go/internal/phases/triage"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

type promptComposer interface {
	ComposePrompt(body string, req core.PhaseRequest) string
}

const staticBody = "STATIC AGENT BODY: persona + rules + skill text + tool defs"

const minAuditedPhases = 7

func composers(t *testing.T) map[string]promptComposer {
	t.Helper()
	root := acsassert.RepoRoot(t)
	out := map[string]promptComposer{}
	for _, name := range registry.Names() {
		factory, ok := registry.For(name)
		if !ok {
			t.Fatalf("registry.Names() reported %q but registry.For(%q) missed it", name, name)
		}
		pr := factory(core.PhaseRequest{ProjectRoot: root})
		if pc, ok := pr.(promptComposer); ok {
			out[name] = pc
		}
	}
	if len(out) < minAuditedPhases {
		t.Fatalf("cache-stable audit covered only %d phases (%v); expected >= %d BaseRunner-based composers — the ComposePrompt seam is missing",
			len(out), keys(out), minAuditedPhases)
	}
	return out
}

func keys(m map[string]promptComposer) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestC534_001_StaticPrefixByteIdenticalAcrossCycles(t *testing.T) {
	reqA := core.PhaseRequest{Cycle: 100, GoalHash: "aaaa1111", ProjectRoot: acsassert.RepoRoot(t), Workspace: "/ws/cycle-100"}
	reqB := core.PhaseRequest{Cycle: 200, GoalHash: "bbbb2222", ProjectRoot: acsassert.RepoRoot(t), Workspace: "/ws/cycle-200"}
	for name, pc := range composers(t) {
		prefixA := runner.StaticPrefix(pc.ComposePrompt(staticBody, reqA))
		prefixB := runner.StaticPrefix(pc.ComposePrompt(staticBody, reqB))
		if prefixA != prefixB {
			t.Errorf("phase %q: static prefix drifted between cycle 100 and cycle 200\n cycle100: %q\n cycle200: %q", name, prefixA, prefixB)
		}
		if prefixA != staticBody {
			t.Errorf("phase %q: static prefix must equal the agent body exactly (cache-stable); got %q, want %q", name, prefixA, staticBody)
		}
	}
}

func TestC534_002_DynamicTokensNeverInPrefix(t *testing.T) {
	const (
		cycleTok = "987654"
		goalTok  = "DYNGOALHASHSENTINEL"
		wsTok    = "/DYN/WORKSPACE/SENTINEL"
	)
	req := core.PhaseRequest{Cycle: 987654, GoalHash: goalTok, ProjectRoot: acsassert.RepoRoot(t), Workspace: wsTok}
	for name, pc := range composers(t) {
		prefix := runner.StaticPrefix(pc.ComposePrompt(staticBody, req))
		for _, tok := range []string{cycleTok, goalTok, wsTok} {
			if strings.Contains(prefix, tok) {
				t.Errorf("phase %q: dynamic token %q leaked into the cache-stable prefix:\n%s", name, tok, prefix)
			}
		}
	}
}

func TestC534_003_GuardDetectsEarlyInjection(t *testing.T) {
	req := core.PhaseRequest{Cycle: 42, GoalHash: "cafe", ProjectRoot: acsassert.RepoRoot(t), Workspace: "/ws/42"}
	clean := runner.BaseCycleContext(staticBody, req)
	cleanPrefix := runner.StaticPrefix(clean)
	mutated := "- cycle: 200\n" + clean
	mutatedPrefix := runner.StaticPrefix(mutated)
	if mutatedPrefix == cleanPrefix {
		t.Fatalf("StaticPrefix is a tautology: an early-injected dynamic value produced the same prefix\n clean:   %q\n mutated: %q", cleanPrefix, mutatedPrefix)
	}
	if !strings.Contains(mutatedPrefix, "- cycle: 200") {
		t.Errorf("mutated prefix must retain the early-injected dynamic value; got %q", mutatedPrefix)
	}
}

func TestC534_004_EmptyBodyYieldsStableEmptyPrefix(t *testing.T) {
	reqA := core.PhaseRequest{Cycle: 1, GoalHash: "g1", ProjectRoot: "/p", Workspace: "/w1"}
	reqB := core.PhaseRequest{Cycle: 2, GoalHash: "g2", ProjectRoot: "/p", Workspace: "/w2"}
	prefixA := runner.StaticPrefix(runner.BaseCycleContext("", reqA))
	prefixB := runner.StaticPrefix(runner.BaseCycleContext("", reqB))
	if prefixA != "" {
		t.Errorf("empty body must yield an empty static prefix; got %q", prefixA)
	}
	if prefixA != prefixB {
		t.Errorf("empty-body prefix must be stable across cycles; cycle1=%q cycle2=%q", prefixA, prefixB)
	}
}

func TestC534_005_RunnerPackageVetClean(t *testing.T) {
	runnerPkg := filepath.Join(acsassert.RepoRoot(t), "go", "internal", "phases", "runner")
	_, stderr, code, err := acsassert.SubprocessOutput("go", "vet", runnerPkg)
	if err != nil {
		t.Fatalf("failed to launch go vet: %v", err)
	}
	if code != 0 {
		t.Errorf("go vet ./internal/phases/runner/... must be clean; exit=%d stderr:\n%s", code, stderr)
	}
}
