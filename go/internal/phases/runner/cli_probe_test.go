package runner

import (
	"errors"
	"reflect"
	"testing"
)

// Workstream G3 — capability probe tests.
//
// probeAvailableCLIChain demotes (not deletes) CLIs whose binary isn't on
// PATH: missing CLIs go to the END of the chain so an available fallback
// runs first, but if ALL candidates are missing the original primary still
// gets attempted (so the bridge surfaces a real ExitMissingBinary 127 to
// the classifier rather than a silent skip).

// fakeLookPath builds a LookPath stub where binaries in `present` succeed.
func fakeLookPath(present ...string) func(string) (string, error) {
	set := make(map[string]struct{}, len(present))
	for _, p := range present {
		set[p] = struct{}{}
	}
	return func(name string) (string, error) {
		if _, ok := set[name]; ok {
			return "/usr/local/bin/" + name, nil
		}
		return "", errors.New("not found")
	}
}

func TestProbeAvailableCLIChain_AllAvailable_NoReorder(t *testing.T) {
	chain := cliChain{
		candidates: []string{"codex-tmux", "claude-tmux", "agy-tmux"},
		triggers:   []int{80, 127},
	}
	got := probeAvailableCLIChain(chain, fakeLookPath("codex", "claude", "agy"))
	if !reflect.DeepEqual(got.candidates, chain.candidates) {
		t.Errorf("all-available chain reordered: %v vs %v", got.candidates, chain.candidates)
	}
}

func TestProbeAvailableCLIChain_PrimaryMissing_Demoted(t *testing.T) {
	// codex-tmux primary but codex binary missing → demote to end so
	// claude-tmux runs first; codex stays in the chain so the fallback
	// loop has a final-resort attempt.
	chain := cliChain{
		candidates: []string{"codex-tmux", "claude-tmux", "agy-tmux"},
		triggers:   []int{80, 127},
	}
	got := probeAvailableCLIChain(chain, fakeLookPath("claude", "agy"))
	want := []string{"claude-tmux", "agy-tmux", "codex-tmux"}
	if !reflect.DeepEqual(got.candidates, want) {
		t.Errorf("candidates=%v, want %v (codex demoted to end)", got.candidates, want)
	}
}

func TestProbeAvailableCLIChain_MultipleMissing_PreservesRelativeOrder(t *testing.T) {
	// Two missing in different positions — the survivors keep their
	// relative order, the missing ones keep theirs at the tail.
	chain := cliChain{
		candidates: []string{"codex-tmux", "ollama-tmux", "claude-tmux", "agy-tmux"},
		triggers:   []int{80, 127},
	}
	got := probeAvailableCLIChain(chain, fakeLookPath("claude", "agy"))
	want := []string{"claude-tmux", "agy-tmux", "codex-tmux", "ollama-tmux"}
	if !reflect.DeepEqual(got.candidates, want) {
		t.Errorf("candidates=%v, want %v", got.candidates, want)
	}
}

func TestProbeAvailableCLIChain_AllMissing_FallsBackToOriginal(t *testing.T) {
	// All missing → keep original chain so the bridge can surface a
	// real ExitMissingBinary (127) to the classifier. A silent
	// reordering would hide the misconfiguration.
	chain := cliChain{
		candidates: []string{"codex-tmux", "claude-tmux"},
		triggers:   []int{80, 127},
	}
	got := probeAvailableCLIChain(chain, fakeLookPath()) // nothing available
	if !reflect.DeepEqual(got.candidates, chain.candidates) {
		t.Errorf("all-missing should fall back to original; got %v", got.candidates)
	}
}

func TestProbeAvailableCLIChain_SingleCandidate_NoOp(t *testing.T) {
	// Single-candidate chain has nothing to demote against; probe is a no-op
	// even if the binary is missing (no fallback to skip to anyway).
	chain := cliChain{candidates: []string{"codex-tmux"}, triggers: []int{80, 127}}
	got := probeAvailableCLIChain(chain, fakeLookPath())
	if !reflect.DeepEqual(got.candidates, chain.candidates) {
		t.Errorf("single-candidate chain modified: %v", got.candidates)
	}
}

func TestProbeAvailableCLIChain_UnknownCLI_KeepsPosition(t *testing.T) {
	// An operator-authored CLI name not in cliBinaryFor is conservatively
	// left at its original position — the bridge's driver registry may
	// still resolve it (e.g., a future custom driver).
	chain := cliChain{
		candidates: []string{"future-cli", "claude-tmux"},
		triggers:   []int{80, 127},
	}
	got := probeAvailableCLIChain(chain, fakeLookPath("claude"))
	want := []string{"future-cli", "claude-tmux"}
	if !reflect.DeepEqual(got.candidates, want) {
		t.Errorf("unknown CLI position not preserved; got %v want %v", got.candidates, want)
	}
}

func TestProbeAvailableCLIChain_NilLookPath_DefaultsToExecLookPath(t *testing.T) {
	// Pass nil — the function must not panic; behavior depends on host
	// PATH but we just assert it returns SOMETHING with the same number
	// of candidates (no candidates were dropped).
	chain := cliChain{
		candidates: []string{"claude-tmux", "codex-tmux"},
		triggers:   []int{80, 127},
	}
	got := probeAvailableCLIChain(chain, nil)
	if len(got.candidates) != len(chain.candidates) {
		t.Errorf("nil lookPath should preserve candidate COUNT; got %d want %d",
			len(got.candidates), len(chain.candidates))
	}
}

func TestSameCandidates(t *testing.T) {
	cases := []struct {
		a, b []string
		want bool
	}{
		{[]string{"a", "b"}, []string{"a", "b"}, true},
		{[]string{"a", "b"}, []string{"b", "a"}, false},
		{[]string{"a"}, []string{"a", "b"}, false},
		{nil, nil, true},
		{nil, []string{}, true},
	}
	for _, c := range cases {
		if got := sameCandidates(c.a, c.b); got != c.want {
			t.Errorf("sameCandidates(%v, %v)=%v want %v", c.a, c.b, got, c.want)
		}
	}
}
