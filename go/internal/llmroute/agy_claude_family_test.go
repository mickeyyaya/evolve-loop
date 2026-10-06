package llmroute

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func TestFamily_AgyClaudeTmuxIsItsOwnFamilyOnTheAgyBinary(t *testing.T) {
	cases := []struct{ driver, family, binary string }{
		{"agy-claude-tmux", "agy-claude", "agy"},
		{"agy-tmux", "agy", "agy"},
		{"agy", "agy", "agy"},
		{"claude-p", "claude", "claude"},
		{"claude-tmux", "claude", "claude"},
		{"codex", "codex", "codex"},
		{"codex-tmux", "codex", "codex"},
		{"ollama-tmux", "ollama", "ollama"},
		{"gpt-cli", "gpt-cli", ""},
	}
	for _, tc := range cases {
		if got := Family(tc.driver); got != tc.family {
			t.Errorf("Family(%q) = %q, want %q", tc.driver, got, tc.family)
		}
		if got := Binary(tc.driver); got != tc.binary {
			t.Errorf("Binary(%q) = %q, want %q", tc.driver, got, tc.binary)
		}
	}
}

func TestFamily_AgreesWithTheProfilesFamilyOfEveryRegisteredDriver(t *testing.T) {
	for driver := range cliBinaryFor {
		if Family(driver) != profiles.BaseCLI(driver) {
			t.Errorf("Family(%q) = %q but profiles.BaseCLI = %q: allowed_clis, the catalog key and the bench key must name one family", driver, Family(driver), profiles.BaseCLI(driver))
		}
	}
}

func TestDefaultDriverForFamily_AgyClaudeRunsOnItsTmuxTarget(t *testing.T) {
	if got := DefaultDriverForFamily("agy-claude"); got != "agy-claude-tmux" {
		t.Fatalf("DefaultDriverForFamily(agy-claude) = %q, want agy-claude-tmux", got)
	}
	if !KnownDriver("agy-claude-tmux") {
		t.Fatal("agy-claude-tmux is a registered bridge driver")
	}
}

func TestProbe_AgyClaudeTmuxIsPresentWhenTheAgyBinaryIs(t *testing.T) {
	var looked []string
	lookPath := func(bin string) (string, error) {
		looked = append(looked, bin)
		if bin == "agy" {
			return "/usr/local/bin/agy", nil
		}
		return "", errors.New("not found")
	}
	out := Probe(Plan{Candidates: []string{"claude-tmux", "agy-claude-tmux"}}, lookPath)
	if want := []string{"agy-claude-tmux", "claude-tmux"}; !slices.Equal(out.Candidates, want) {
		t.Fatalf("Probe = %v, want %v: agy-claude-tmux runs the agy binary", out.Candidates, want)
	}
	if slices.Contains(looked, "agy-claude") {
		t.Fatalf("Probe looked up %v; there is no agy-claude binary", looked)
	}
}

func TestApplyBench_AnAgyClaudeWallLeavesAgyGeminiUnbenchedAndTheReverse(t *testing.T) {
	at := time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC)
	plan := Plan{Candidates: []string{"agy-claude-tmux", "agy-tmux", "claude-tmux"}}
	cases := []struct {
		benched string
		want    []string
	}{
		{"agy-claude", []string{"agy-tmux", "claude-tmux", "agy-claude-tmux"}},
		{"agy", []string{"agy-claude-tmux", "claude-tmux", "agy-tmux"}},
	}
	for _, tc := range cases {
		out := ApplyBench(plan, map[string]time.Time{tc.benched: at})
		if !slices.Equal(out.Candidates, tc.want) {
			t.Errorf("bench %s: chain = %v, want %v", tc.benched, out.Candidates, tc.want)
		}
	}
}

func TestDrivers_ListsEveryKnownDriverSortedWithItsBinary(t *testing.T) {
	drivers := Drivers()
	if !slices.IsSorted(drivers) || !slices.Contains(drivers, "agy-claude-tmux") || len(drivers) != len(cliBinaryFor) {
		t.Fatalf("Drivers() = %v, want every registered driver, sorted", drivers)
	}
	for _, d := range drivers {
		if !KnownDriver(d) || Binary(d) == "" {
			t.Errorf("Drivers() lists %q, but KnownDriver = %v and Binary = %q", d, KnownDriver(d), Binary(d))
		}
	}
}
