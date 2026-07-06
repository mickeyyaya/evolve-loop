package deliverable

import (
	"strings"
	"testing"
)

// reportsize_test.go — RED contract for cycle-565 Slice S1 of
// report-size-contracts-jit-artifacts: a per-artifact token/size budget check
// on the never-evict "## Handoff Summary" section (phasecontract.HandoffSummary
// — see handoffsummary_test.go in go/internal/phasecontract). No tokenizer
// dependency exists in this repo (go.mod has none), so EstimateTokens uses the
// common ~4-chars-per-token heuristic already implicit in this codebase's other
// byte-length budgets (e.g. core.salvageMaxBytes) rather than inventing a real
// tokenizer.
//
// RED today: EstimateTokens, HandoffSectionContent, CheckHandoffBudget, and
// CodeHandoffBudgetExceeded do not exist (compile failure).

func TestEstimateTokens(t *testing.T) {
	cases := []struct {
		name string
		s    string
		want int
	}{
		{"empty", "", 0},
		{"exact multiple of 4 chars/token", strings.Repeat("x", 400), 100},
		{"non-multiple rounds down", strings.Repeat("x", 401), 100},
	}
	for _, c := range cases {
		if got := EstimateTokens(c.s); got != c.want {
			t.Errorf("%s: EstimateTokens(len=%d) = %d, want %d", c.name, len(c.s), got, c.want)
		}
	}
	// Monotonic: more content never yields fewer estimated tokens (anti-no-op —
	// a stub that always returns a constant fails this).
	if EstimateTokens(strings.Repeat("y", 4000)) <= EstimateTokens(strings.Repeat("y", 40)) {
		t.Error("EstimateTokens must grow with content length (monotonic); got a flat/constant result")
	}
}

func TestHandoffSectionContent(t *testing.T) {
	t.Run("absent heading", func(t *testing.T) {
		_, ok := HandoffSectionContent("## Changes\n- x\n")
		if ok {
			t.Error("HandoffSectionContent must report ok=false when no Handoff Summary heading exists")
		}
	})
	t.Run("present, stops at next heading", func(t *testing.T) {
		content := "## Handoff Summary\ndecision: ship it\n## Detail\nbody that must be excluded\n"
		got, ok := HandoffSectionContent(content)
		if !ok {
			t.Fatal("HandoffSectionContent must report ok=true when the heading is present")
		}
		if !strings.Contains(got, "decision: ship it") {
			t.Errorf("section content = %q, want it to contain the handoff body", got)
		}
		if strings.Contains(got, "body that must be excluded") {
			t.Errorf("section content = %q, must stop at the next ## heading, not bleed into Detail", got)
		}
	})
	t.Run("present, runs to EOF", func(t *testing.T) {
		content := "## Changes\n- x\n## Handoff Summary\nverdict: PASS\n"
		got, ok := HandoffSectionContent(content)
		if !ok {
			t.Fatal("HandoffSectionContent must report ok=true")
		}
		if !strings.Contains(got, "verdict: PASS") {
			t.Errorf("section content = %q, want it to contain the trailing body to EOF", got)
		}
	})
}

func TestCheckHandoffBudget(t *testing.T) {
	t.Run("under budget: not violated", func(t *testing.T) {
		content := "## Handoff Summary\nshort decision\n"
		violated, estimated := CheckHandoffBudget(content, 2000)
		if violated {
			t.Errorf("short handoff section must not violate a 2000-token budget (estimated=%d)", estimated)
		}
	})
	t.Run("over budget: violated", func(t *testing.T) {
		big := strings.Repeat("word ", 5000) // ~25000 chars => ~6250 estimated tokens
		content := "## Handoff Summary\n" + big
		violated, estimated := CheckHandoffBudget(content, 2000)
		if !violated {
			t.Errorf("oversized handoff section must violate a 2000-token budget (estimated=%d)", estimated)
		}
		if estimated <= 2000 {
			t.Errorf("estimated=%d, want > budget 2000 to justify the violation", estimated)
		}
	})
	t.Run("missing section: never violated (CodeMissingSection's job, not this check's)", func(t *testing.T) {
		violated, estimated := CheckHandoffBudget("## Changes\n- x\n", 1)
		if violated {
			t.Errorf("an absent Handoff Summary section must not be reported as a budget violation (estimated=%d)", estimated)
		}
	})
}

func TestCodeHandoffBudgetExceeded_IsStableIdentifier(t *testing.T) {
	if CodeHandoffBudgetExceeded != "handoff_budget_exceeded" {
		t.Errorf("CodeHandoffBudgetExceeded = %q, want stable snake_case %q", CodeHandoffBudgetExceeded, "handoff_budget_exceeded")
	}
}
