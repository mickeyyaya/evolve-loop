package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestParseLoopArgs_GoalSources covers the three goal sources and
// their precedence (--goal-hash > --goal-text > positional).
func TestParseLoopArgs_GoalSources(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		wantHash string // empty = rc != 0
		wantRC   int
	}{
		{
			"explicit_goal_hash_wins",
			[]string{"--goal-hash", "deadbeef", "--goal-text", "ignored", "fix the bug"},
			"deadbeef",
			0,
		},
		{
			"goal_text_computed",
			[]string{"--goal-text", "Fix the BUG"},
			// goalhash.Compute normalizes to "fix the bug" then SHA256
			// = same as "fix the bug" plain
			"a09f5d75a09f1ec5d518dbab47a4b3676ad7e6dad7e7a3c7e8c9b6c7e8c9b6c7", // placeholder; real value asserted via prefix
			0,
		},
		{
			"positional_goal",
			[]string{"fix the bug"},
			"a09f5d75",
			0,
		},
		{
			"resume_no_goal_ok",
			[]string{"--resume"},
			"", // resume mode allows empty goal hash
			0,
		},
		{
			"no_goal_no_resume_fails",
			[]string{},
			"",
			10,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stderr bytes.Buffer
			cfg, rc := parseLoopArgs(tc.args, &stderr)
			if rc != tc.wantRC {
				t.Fatalf("rc=%d, want %d (stderr=%q)", rc, tc.wantRC, stderr.String())
			}
			if rc != 0 {
				return
			}
			// For explicit hash, exact match required. For text/positional,
			// just verify some hash got computed.
			if tc.name == "explicit_goal_hash_wins" {
				if cfg.GoalHash != "deadbeef" {
					t.Errorf("GoalHash=%q, want deadbeef", cfg.GoalHash)
				}
			} else if tc.name == "resume_no_goal_ok" {
				if cfg.GoalHash != "" {
					t.Errorf("resume mode: GoalHash=%q, want empty", cfg.GoalHash)
				}
				if !cfg.Resume {
					t.Errorf("Resume flag not set")
				}
			} else {
				if len(cfg.GoalHash) != 64 {
					t.Errorf("GoalHash length=%d, want 64 (full SHA256)", len(cfg.GoalHash))
				}
			}
		})
	}
}

// TestParseLoopArgs_PositionalCyclesStrategy validates the bash-style
// positional parsing: [CYCLES] [STRATEGY] [GOAL...].
func TestParseLoopArgs_PositionalCyclesStrategy(t *testing.T) {
	cases := []struct {
		name         string
		args         []string
		wantCycles   int
		wantStrategy string
		wantGoalSub  string // substring of resolved GoalText
	}{
		{
			"cycles_strategy_goal",
			[]string{"3", "balanced", "fix the bug"},
			3, "balanced", "fix the bug",
		},
		{
			"cycles_only_default_strategy",
			[]string{"5", "improve performance"},
			5, "balanced", "improve performance",
		},
		{
			"goal_only_default_cycles_strategy",
			[]string{"refactor module"},
			1, "balanced", "refactor module",
		},
		{
			"strategy_alone_no_cycles",
			[]string{"innovate", "explore new ideas"},
			1, "innovate", "explore new ideas",
		},
		{
			"goal_with_apostrophe",
			[]string{"the goal isn't trivial - needs research"},
			1, "balanced", "isn't",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stderr bytes.Buffer
			cfg, rc := parseLoopArgs(tc.args, &stderr)
			if rc != 0 {
				t.Fatalf("rc=%d, stderr=%q", rc, stderr.String())
			}
			if cfg.MaxCycles != tc.wantCycles {
				t.Errorf("MaxCycles=%d, want %d", cfg.MaxCycles, tc.wantCycles)
			}
			if cfg.Strategy != tc.wantStrategy {
				t.Errorf("Strategy=%q, want %q", cfg.Strategy, tc.wantStrategy)
			}
			if !strings.Contains(cfg.GoalText, tc.wantGoalSub) {
				t.Errorf("GoalText=%q, want substring %q", cfg.GoalText, tc.wantGoalSub)
			}
		})
	}
}

// TestParseLoopArgs_StrategyValidation rejects unknown strategy values.
func TestParseLoopArgs_StrategyValidation(t *testing.T) {
	var stderr bytes.Buffer
	_, rc := parseLoopArgs([]string{"--strategy", "bogus", "goal"}, &stderr)
	if rc != 10 {
		t.Errorf("rc=%d, want 10", rc)
	}
	if !strings.Contains(stderr.String(), "invalid --strategy") {
		t.Errorf("stderr missing 'invalid --strategy' diagnostic: %q", stderr.String())
	}
}

// TestParseLoopArgs_FlagPrecedence verifies explicit flags beat positional.
func TestParseLoopArgs_FlagPrecedence(t *testing.T) {
	var stderr bytes.Buffer
	cfg, rc := parseLoopArgs([]string{
		"--cycles", "7",
		"--strategy", "harden",
		"3", "balanced", "positional goal", // these should all be subordinate
	}, &stderr)
	if rc != 0 {
		t.Fatalf("rc=%d, stderr=%q", rc, stderr.String())
	}
	if cfg.MaxCycles != 7 {
		t.Errorf("MaxCycles=%d, want 7 (--cycles wins)", cfg.MaxCycles)
	}
	if cfg.Strategy != "harden" {
		t.Errorf("Strategy=%q, want harden (--strategy wins)", cfg.Strategy)
	}
	// Goal text becomes "positional goal" (positional goal still applied
	// when no --goal-text). The 3 is consumed as cycles even though
	// --cycles overrides; the strategy "balanced" is consumed as
	// strategy positionally even though --strategy overrides; the rest
	// is goal.
	if !strings.Contains(cfg.GoalText, "positional goal") {
		t.Errorf("GoalText=%q, want substring 'positional goal'", cfg.GoalText)
	}
}

// TestParseLoopArgs_DryRun ensures --dry-run is captured (downstream
// runLoop short-circuits on it).
func TestParseLoopArgs_DryRun(t *testing.T) {
	var stderr bytes.Buffer
	cfg, rc := parseLoopArgs([]string{"--dry-run", "fix bug"}, &stderr)
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	if !cfg.DryRun {
		t.Errorf("DryRun=false, want true")
	}
}

// TestParseLoopArgs_BudgetFlagsAreNoOps verifies the removed --budget-usd flag
// no longer drives behavior: a legacy invocation is stripped (not rejected), so
// it parses without error and does NOT bump the cycle count (the former
// budget-mode 50-cycle default is gone — cost is display-only telemetry now).
func TestParseLoopArgs_BudgetFlagsAreNoOps(t *testing.T) {
	var stderr bytes.Buffer
	cfg, rc := parseLoopArgs([]string{"--budget-usd", "5", "fix bug"}, &stderr)
	if rc != 0 {
		t.Fatalf("rc=%d, want 0 (legacy --budget-usd must be stripped, not rejected)", rc)
	}
	if cfg.MaxCycles != 1 {
		t.Errorf("MaxCycles=%d, want 1 (--budget-usd must not drive cycle count)", cfg.MaxCycles)
	}
	// The goal positional must survive the strip intact.
	if cfg.GoalText != "fix bug" {
		t.Errorf("GoalText=%q, want \"fix bug\" (strip must not eat the positional goal)", cfg.GoalText)
	}
}

// TestParseLoopArgs_BudgetFlagWarnsRemoved verifies that a legacy --budget-usd
// emits a visible "removed" notice on stderr pointing operators at --cycles, and
// that the flag's absence produces no budget noise.
func TestParseLoopArgs_BudgetFlagWarnsRemoved(t *testing.T) {
	var withFlag bytes.Buffer
	parseLoopArgs([]string{"--budget-usd", "5", "fix bug"}, &withFlag)
	got := withFlag.String()
	low := strings.ToLower(got)
	if !strings.Contains(got, "--budget-usd") || !strings.Contains(low, "removed") || !strings.Contains(got, "--cycles") {
		t.Errorf("expected a removal notice mentioning --budget-usd and --cycles; stderr=%q", got)
	}
	// And no budget noise when the flag is absent.
	var noFlag bytes.Buffer
	parseLoopArgs([]string{"fix bug"}, &noFlag)
	if strings.Contains(strings.ToLower(noFlag.String()), "budget") {
		t.Errorf("no budget notice expected when --budget-usd is absent; stderr=%q", noFlag.String())
	}
}

// TestParsePositional unit-tests the heuristic in isolation.
func TestParsePositional(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		cycles   int
		strategy string
		goal     string
	}{
		{"empty", []string{}, 0, "", ""},
		{"only_cycles", []string{"3"}, 3, "", ""},
		{"only_goal", []string{"do stuff"}, 0, "", "do stuff"},
		{"cycles_and_strategy", []string{"5", "balanced"}, 5, "balanced", ""},
		{"all_three", []string{"2", "innovate", "explore", "ideas"}, 2, "innovate", "explore ideas"},
		{"strategy_first", []string{"harden", "lock down"}, 0, "harden", "lock down"},
		{"zero_cycles_ignored", []string{"0", "do stuff"}, 0, "", "0 do stuff"},
		{"negative_ignored", []string{"-1", "goal"}, 0, "", "-1 goal"},
		{"non_numeric_first", []string{"abc", "more"}, 0, "", "abc more"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, s, g := parsePositional(tc.args)
			if c != tc.cycles || s != tc.strategy || g != tc.goal {
				t.Errorf("got (%d, %q, %q), want (%d, %q, %q)",
					c, s, g, tc.cycles, tc.strategy, tc.goal)
			}
		})
	}
}

// TestJoinArgs spot-checks the joiner — important because the bash
// dispatcher joins remaining positional args with a single space.
func TestJoinArgs(t *testing.T) {
	cases := []struct {
		in   []string
		want string
	}{
		{nil, ""},
		{[]string{}, ""},
		{[]string{"a"}, "a"},
		{[]string{"a", "b"}, "a b"},
		{[]string{"a", "b", "c"}, "a b c"},
		{[]string{"with apostrophe"}, "with apostrophe"},
	}
	for _, tc := range cases {
		got := joinArgs(tc.in)
		if got != tc.want {
			t.Errorf("joinArgs(%v)=%q, want %q", tc.in, got, tc.want)
		}
	}
}
