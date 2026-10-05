package main

import (
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"testing"

	gobridge "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
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

	got := universalFallbackTail(results, []string{"agy"})

	if want := []string{"claude-tmux", "codex-tmux"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("tail = %v, want %v: ollama cannot write a worktree artifact, agy is the operator's ban", got, want)
	}
}

func TestTheCheckedInPolicyPutsAgyInTheLastResortTail(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	checkedIn := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", ".evolve", "policy.json")
	pol, err := policy.Load(checkedIn)
	if err != nil {
		t.Fatalf("load %s: %v", checkedIn, err)
	}
	if !pol.WorkflowConfig().UniversalFallback {
		t.Fatal("the checked-in policy turns the last-resort tail off, so agy would join no chain")
	}
	results := []gobridge.DoctorResult{
		doctorResult("claude-tmux", true, "ready"),
		doctorResult("agy-tmux", true, "ready"),
		doctorResult("codex-tmux", true, "ready"),
	}

	got := universalFallbackTail(results, pol.WorkflowConfig().UniversalFallbackExclude)

	if !slices.Contains(got, "agy-tmux") {
		t.Fatalf("tail = %v: the operator added the Antigravity CLI to the pipeline (2026-10-05), so the checked-in policy keeps agy in every launch's last-resort tail", got)
	}
}
