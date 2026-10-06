package main

import (
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"testing"

	gobridge "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func doctorResult(cli string, present bool, verdict string) gobridge.DoctorResult {
	return gobridge.DoctorResult{CLI: cli, Binary: gobridge.BinaryInfo{Present: present}, Verdict: verdict}
}

func TestUniversalFallbackTail_KeepsOnlyPresentFamiliesThatCanRunAWorktreePhase(t *testing.T) {
	results := []gobridge.DoctorResult{
		doctorResult("claude-p", false, "blocked"),
		doctorResult("claude-tmux", true, "ready"),
		doctorResult("claude-p", true, "ready"),
		doctorResult("ollama-tmux", true, "ready"),
		doctorResult("agy-tmux", true, "ready"),
		doctorResult("codex-tmux", true, "warning"),
		doctorResult("codex", false, "blocked"),
	}

	got := universalFallbackTail(results)

	if want := []string{"claude-tmux", "agy-tmux", "codex-tmux"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("tail = %v, want %v: ollama cannot write a worktree artifact; the family ban is the router's, not discovery's", got, want)
	}
}

func checkedInPolicy(t *testing.T) (policy.Policy, string) {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	pol, err := policy.Load(filepath.Join(root, ".evolve", "policy.json"))
	if err != nil {
		t.Fatalf("load the checked-in policy: %v", err)
	}
	return pol, root
}

func scoutChainUnder(t *testing.T, pol policy.Policy, root string, results []gobridge.DoctorResult) []string {
	t.Helper()
	r, _, err := cliroute.Build(cliroute.Setup{
		Policy: pol, Profiles: profiles.NewFromDir(filepath.Join(root, ".evolve", "profiles")),
		Host: cliroute.Host{LookPath: func(string) (string, error) { return "/fake", nil }, Discover: func() []string { return universalFallbackTail(results) }},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	d, err := r.Resolve(cliroute.Request{Agent: "scout", Phase: "scout", ProjectRoot: t.TempDir()})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	return d.Plan.Candidates
}

func TestUniversalFallbackTail_LeavesTheExclusionToTheRouter(t *testing.T) {
	_, root := checkedInPolicy(t)
	results := []gobridge.DoctorResult{doctorResult("claude-tmux", true, "ready"), doctorResult("agy-tmux", true, "ready")}
	if got := scoutChainUnder(t, policy.Policy{}, root, results); slices.Contains(got, "agy-tmux") {
		t.Fatalf("the compiled-default ban keeps agy out of the tail, applied once by the router: %v", got)
	}
}

func TestTheCheckedInPolicyPutsAgyInTheLastResortTail(t *testing.T) {
	pol, root := checkedInPolicy(t)
	if !pol.WorkflowConfig().UniversalFallback {
		t.Fatal("the checked-in policy turns the last-resort tail off, so agy would join no chain")
	}
	results := []gobridge.DoctorResult{
		doctorResult("claude-tmux", true, "ready"),
		doctorResult("agy-tmux", true, "ready"),
		doctorResult("codex-tmux", true, "ready"),
	}
	if got := scoutChainUnder(t, pol, root, results); !slices.Contains(got, "agy-tmux") {
		t.Fatalf("scout chain = %v: the operator added the Antigravity CLI to the pipeline (2026-10-05), so the checked-in policy keeps agy in every launch's last-resort tail", got)
	}
}
