package wave_test

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

func TestFacts_Outcome(t *testing.T) {
	t.Parallel()
	cases := []struct {
		cycles []wave.Cycle
		want   string
	}{
		{nil, "no cycle ran"},
		{[]wave.Cycle{{ID: 1, Shipped: true}, {ID: 2, Verdict: "FAIL"}}, "1 of 2 cycles shipped"},
		{[]wave.Cycle{{ID: 1, Shipped: true}}, "1 of 1 cycles shipped"},
		{[]wave.Cycle{{ID: 1}, {ID: 2}, {ID: 3}}, "0 of 3 cycles shipped"},
	}
	for _, c := range cases {
		if got := (wave.Facts{Cycles: c.cycles}).Outcome(); got != c.want {
			t.Errorf("Outcome(%+v) = %q, want %q", c.cycles, got, c.want)
		}
	}
}

func TestFacts_Section(t *testing.T) {
	t.Parallel()
	f := wave.Facts{
		Wave: 81, RunID: "20261008T120000Z",
		Cycles: []wave.Cycle{
			{ID: 1835, Verdict: "WARN", Shipped: true},
			{ID: 1836, Phase: "scout", QuotaPauses: 1},
		},
		MergedPRs: []string{"814", "816"},
	}

	got := f.Section()

	want := strings.Join([]string{
		"Wave 81 facts (run 20261008T120000Z): 1 of 2 cycles shipped.",
		"- Cycle 1835: final verdict WARN. It shipped.",
		"- Cycle 1836: no final verdict. Last phase: scout. It did not ship. Quota pauses: 1.",
		"- Pull requests merged at this boundary: #814, #816.",
	}, "\n")
	if got != want {
		t.Errorf("Section() =\n%s\nwant\n%s", got, want)
	}
}

func TestFacts_SectionWithNoPullRequest(t *testing.T) {
	t.Parallel()
	got := wave.Facts{Wave: 3, RunID: "r"}.Section()

	want := "Wave 3 facts (run r): no cycle ran.\n- Pull requests merged at this boundary: none."
	if got != want {
		t.Errorf("Section() = %q, want %q", got, want)
	}
}

func TestCompose(t *testing.T) {
	t.Parallel()
	last := wave.Facts{Wave: 81, RunID: "r81", Cycles: []wave.Cycle{{ID: 1835, Verdict: "PASS", Shipped: true}}}
	in := wave.GoalInput{
		Next:     82,
		Standing: "Work the inbox.\n\n",
		Last:     &last,
		Notes:    []wave.Note{{Name: "a", Text: "watch PR 814"}, {Name: "b", Text: "keep every cycle shipping"}},
		History: []wave.Record{
			{Number: 79, RunID: "r79", Outcome: "2 of 2 cycles shipped"},
			{Number: 80, RunID: "r80", Outcome: "1 of 1 cycles shipped"},
		},
	}

	got := wave.Compose(in)

	want := strings.Join([]string{
		"Wave 82.",
		"",
		"Work the inbox.",
		"",
		last.Section(),
		"",
		"Operator notes for wave 82:",
		"- watch PR 814",
		"- keep every cycle shipping",
		"",
		"Earlier waves:",
		"- Wave 79 (run r79): 2 of 2 cycles shipped.",
		"- Wave 80 (run r80): 1 of 1 cycles shipped.",
		"",
	}, "\n")
	if got != want {
		t.Errorf("Compose() =\n%s\nwant\n%s", got, want)
	}
}

func TestCompose_FirstWaveHasNoFactsNotesOrHistory(t *testing.T) {
	t.Parallel()
	got := wave.Compose(wave.GoalInput{Next: 1, Standing: "Work the inbox."})

	want := "Wave 1.\n\nWork the inbox.\n\nNo earlier wave is recorded.\n"
	if got != want {
		t.Errorf("Compose() = %q, want %q", got, want)
	}
}

func TestCompose_AnOpenRecordShowsItsOutcomeAsUnknown(t *testing.T) {
	t.Parallel()
	got := wave.Compose(wave.GoalInput{Next: 5, Standing: "S", History: []wave.Record{{Number: 3, RunID: "r3"}}})

	if !strings.Contains(got, "- Wave 3 (run r3): outcome not recorded.\n") {
		t.Errorf("Compose() = %q, want the open wave 3 with an unknown outcome", got)
	}
}

func TestFacts_SectionBoundsTheCycleList(t *testing.T) {
	t.Parallel()
	var cycles []wave.Cycle
	for id := 1; id <= 150; id++ {
		cycles = append(cycles, wave.Cycle{ID: id, Verdict: "PASS", Shipped: id%2 == 0})
	}

	got := wave.Facts{Wave: 9, RunID: "r9", Cycles: cycles, Limit: 20}.Section()

	lines := strings.Split(got, "\n")
	if len(lines) != 23 || len(got) > 2048 {
		t.Fatalf("Section() has %d lines and %d bytes, want 23 lines (header, omitted line, 20 cycles, PRs) under 2 KB:\n%s", len(lines), len(got), got)
	}
	if lines[0] != "Wave 9 facts (run r9): 75 of 150 cycles shipped." || lines[1] != "- 130 older cycles are not shown." || !strings.HasPrefix(lines[2], "- Cycle 131:") || !strings.HasPrefix(lines[21], "- Cycle 150:") {
		t.Errorf("Section() head and tail = %q, %q, %q, %q", lines[0], lines[1], lines[2], lines[21])
	}
}

func TestFacts_SectionWithoutALimitShowsEveryCycle(t *testing.T) {
	t.Parallel()
	got := wave.Facts{Wave: 1, RunID: "r", Cycles: []wave.Cycle{{ID: 1}, {ID: 2}}}.Section()

	if strings.Contains(got, "not shown") || strings.Count(got, "- Cycle ") != 2 {
		t.Errorf("Section() = %q, want both cycles and no omitted line", got)
	}
}
