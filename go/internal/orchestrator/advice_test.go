package orchestrator

import "testing"

func TestConsoleAdvice_NamesTheCaseOfEachBoundary(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		admissible, window, openPRs int
		want                        string
	}{
		"starved supply feeds":                {1, 8, 0, AdviceFeed},
		"7 of 8 still feeds":                  {7, 8, 0, AdviceFeed},
		"full window builds":                  {8, 8, 3, AdviceBuild},
		"one batch of PRs still builds":       {8, 8, ConsoleBatchSize, AdviceBuild},
		"more than one batch finishes":        {8, 8, ConsoleBatchSize + 1, AdviceFinish},
		"finish outranks feed":                {1, 8, 11, AdviceFinish},
		"a window of zero never feeds":        {0, 0, 0, AdviceBuild},
		"a surplus over the window builds":    {9, 8, 0, AdviceBuild},
		"the advice strings are the contract": {8, 8, 0, "console: build"},
	} {
		if got := ConsoleAdvice(tc.admissible, tc.window, tc.openPRs); got != tc.want {
			t.Errorf("%s: ConsoleAdvice(%d, %d, %d) = %q, want %q", name, tc.admissible, tc.window, tc.openPRs, got, tc.want)
		}
	}
	if AdviceFeed != "console: feed" || AdviceFinish != "console: finish" {
		t.Errorf("advice strings = %q, %q", AdviceFeed, AdviceFinish)
	}
}
