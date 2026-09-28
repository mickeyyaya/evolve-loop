package main

import (
	"reflect"
	"testing"

	gobridge "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
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
