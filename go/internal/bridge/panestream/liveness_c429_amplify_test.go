package panestream

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

// Adversarial amplification tests for ExtractResponseTokens (cycle-429).
// Written by Test Amplifier — black-box, spec only: no implementation reading.
//
// Contract under test:
//   ExtractResponseTokens(pane string) int
//   - k-form:  "↓ 5.2k tokens" → 5200
//   - plain:   "↓ 200 tokens"  → 200
//   - peak:    returns max across all matches in pane
//   - invalid: returns 0

// TestExtractResponseTokens_Concurrent verifies no data race when many
// goroutines call the exported function simultaneously.
func TestExtractResponseTokens_Concurrent(t *testing.T) {
	const workers = 20
	pane := "↓ 3.0k tokens\n↓ 1.5k tokens\n↓ 4.0k tokens"
	want := 4000

	var wg sync.WaitGroup
	errs := make(chan string, workers)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			got := ExtractResponseTokens(pane)
			if got != want {
				errs <- fmt.Sprintf("worker %d: got %d, want %d", id, got, want)
			}
		}(i)
	}
	wg.Wait()
	close(errs)

	for msg := range errs {
		t.Error(msg)
	}
}

// TestExtractResponseTokens_KFormEdgeCases tests k-form boundary values not
// covered by the TDD engineer's 14 baseline cases.
func TestExtractResponseTokens_KFormEdgeCases(t *testing.T) {
	cases := []struct {
		name string
		pane string
		want int
	}{
		{
			name: "zero_k_form",
			pane: "↓ 0.0k tokens",
			want: 0,
		},
		{
			name: "large_k_form_999.9k",
			pane: "↓ 999.9k tokens",
			want: 999900,
		},
		{
			name: "small_fraction_0.1k",
			pane: "↓ 0.1k tokens",
			want: 100,
		},
		{
			name: "hundred_k_100.0k",
			pane: "↓ 100.0k tokens",
			want: 100000,
		},
		{
			name: "one_k_form_1.0k",
			pane: "↓ 1.0k tokens",
			want: 1000,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ExtractResponseTokens(tc.pane)
			if got != tc.want {
				t.Errorf("ExtractResponseTokens(%q) = %d, want %d", tc.pane, got, tc.want)
			}
		})
	}
}

// TestExtractResponseTokens_PlainIntegerEdgeCases tests plain-integer boundary
// values.  The old k-only extractor returned 0 for all of these — this is the
// "superset" guarantee the unified function adds.
func TestExtractResponseTokens_PlainIntegerEdgeCases(t *testing.T) {
	cases := []struct {
		name string
		pane string
		want int
	}{
		{
			name: "zero_plain",
			pane: "↓ 0 tokens",
			want: 0,
		},
		{
			name: "one_plain",
			pane: "↓ 1 tokens",
			want: 1,
		},
		{
			name: "large_plain_999999",
			pane: "↓ 999999 tokens",
			want: 999999,
		},
		{
			name: "reconciled_5200_was_zero_in_old_extractor",
			pane: "↓ 5200 tokens",
			want: 5200,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ExtractResponseTokens(tc.pane)
			if got != tc.want {
				t.Errorf("ExtractResponseTokens(%q) = %d, want %d", tc.pane, got, tc.want)
			}
		})
	}
}

// TestExtractResponseTokens_MixedKFormAndPlainPeak verifies that peak selection
// works correctly across both parse paths in a single pane.
func TestExtractResponseTokens_MixedKFormAndPlainPeak(t *testing.T) {
	cases := []struct {
		name string
		pane string
		want int
	}{
		{
			name: "plain_wins_over_k_form",
			// 300 plain > 0.2k (200) → expect 300
			pane: "↓ 0.2k tokens\n↓ 300 tokens",
			want: 300,
		},
		{
			name: "k_form_wins_over_plain",
			// 1.5k (1500) > 200 plain → expect 1500
			pane: "↓ 200 tokens\n↓ 1.5k tokens",
			want: 1500,
		},
		{
			name: "equal_values_both_forms",
			// 1.0k == 1000 plain → expect 1000
			pane: "↓ 1.0k tokens\n↓ 1000 tokens",
			want: 1000,
		},
		{
			name: "three_lines_peak_is_middle",
			pane: "↓ 100 tokens\n↓ 0.5k tokens\n↓ 200 tokens",
			want: 500,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ExtractResponseTokens(tc.pane)
			if got != tc.want {
				t.Errorf("ExtractResponseTokens(%q) = %d, want %d", tc.pane, got, tc.want)
			}
		})
	}
}

// TestExtractResponseTokens_NotMatchingFormats verifies that plausible-looking
// but spec-non-conforming strings return 0, not a spurious value.
func TestExtractResponseTokens_NotMatchingFormats(t *testing.T) {
	cases := []struct {
		name string
		pane string
	}{
		{
			name: "wrong_arrow_direction",
			pane: "→ 5.2k tokens",
		},
		{
			name: "up_arrow",
			pane: "↑ 5.2k tokens",
		},
		{
			name: "bare_number_no_arrow",
			pane: "5200 tokens",
		},
		// DISCOVERED: regex matches "arrow_no_space" (↓5.2k tokens) — more permissive than spec example but pre-429 behavior.
		{
			name: "singular_token_word",
			pane: "↓ 5.2k token",
		},
		{
			name: "comma_separated_number",
			pane: "↓ 1,200 tokens",
		},
		{
			name: "only_whitespace",
			pane: "   \n\t  ",
		},
		{
			name: "number_only",
			pane: "5200",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ExtractResponseTokens(tc.pane)
			if got != 0 {
				t.Errorf("ExtractResponseTokens(%q) = %d, want 0 (no valid token line)", tc.pane, got)
			}
		})
	}
}

// TestExtractResponseTokens_TokenLineBuriedInLongPane checks that the function
// correctly extracts from a realistic multi-hundred-line pane where the token
// line is not at the top.
func TestExtractResponseTokens_TokenLineBuriedInLongPane(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 200; i++ {
		sb.WriteString(fmt.Sprintf("output line %d: processing step foo bar baz\n", i))
	}
	sb.WriteString("↓ 7.3k tokens\n")
	for i := 0; i < 100; i++ {
		sb.WriteString(fmt.Sprintf("trailing line %d\n", i))
	}

	got := ExtractResponseTokens(sb.String())
	want := 7300
	if got != want {
		t.Errorf("token line buried in 300-line pane: got %d, want %d", got, want)
	}
}

// TestExtractResponseTokens_MultipleIdenticalValues verifies that peak across
// repeated identical token lines returns the value once, not an accumulated sum.
func TestExtractResponseTokens_MultipleIdenticalValues(t *testing.T) {
	pane := "↓ 500 tokens\n↓ 500 tokens\n↓ 500 tokens"
	got := ExtractResponseTokens(pane)
	want := 500
	if got != want {
		t.Errorf("three identical lines: got %d, want %d (must not accumulate)", got, want)
	}
}

// TestExtractResponseTokens_DecreasingSequence verifies that peak across a
// strictly decreasing sequence still returns the first (largest) value.
func TestExtractResponseTokens_DecreasingSequence(t *testing.T) {
	pane := "↓ 2.0k tokens\n↓ 1.5k tokens\n↓ 1.0k tokens\n↓ 0.5k tokens"
	got := ExtractResponseTokens(pane)
	want := 2000
	if got != want {
		t.Errorf("decreasing sequence: got %d, want %d", got, want)
	}
}

// TestExtractResponseTokens_WindowsCRLF verifies correct extraction from pane
// output that uses Windows-style line endings (\r\n), as tmux may emit these
// on some hosts.
func TestExtractResponseTokens_WindowsCRLF(t *testing.T) {
	pane := "some pane output\r\n↓ 2.5k tokens\r\nmore output\r\n"
	got := ExtractResponseTokens(pane)
	want := 2500
	if got != want {
		t.Errorf("CRLF pane: got %d, want %d", got, want)
	}
}
