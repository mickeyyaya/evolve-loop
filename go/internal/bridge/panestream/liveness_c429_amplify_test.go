package panestream

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

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

func TestExtractResponseTokens_MixedKFormAndPlainPeak(t *testing.T) {
	cases := []struct {
		name string
		pane string
		want int
	}{
		{
			name: "plain_wins_over_k_form",
			pane: "↓ 0.2k tokens\n↓ 300 tokens",
			want: 300,
		},
		{
			name: "k_form_wins_over_plain",
			pane: "↓ 200 tokens\n↓ 1.5k tokens",
			want: 1500,
		},
		{
			name: "equal_values_both_forms",
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
		// "↓5.2k tokens" (no space) is deliberately absent: the regex accepts it.
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

func TestExtractResponseTokens_MultipleIdenticalValues(t *testing.T) {
	pane := "↓ 500 tokens\n↓ 500 tokens\n↓ 500 tokens"
	got := ExtractResponseTokens(pane)
	want := 500
	if got != want {
		t.Errorf("three identical lines: got %d, want %d (must not accumulate)", got, want)
	}
}

func TestExtractResponseTokens_DecreasingSequence(t *testing.T) {
	pane := "↓ 2.0k tokens\n↓ 1.5k tokens\n↓ 1.0k tokens\n↓ 0.5k tokens"
	got := ExtractResponseTokens(pane)
	want := 2000
	if got != want {
		t.Errorf("decreasing sequence: got %d, want %d", got, want)
	}
}

func TestExtractResponseTokens_WindowsCRLF(t *testing.T) {
	pane := "some pane output\r\n↓ 2.5k tokens\r\nmore output\r\n"
	got := ExtractResponseTokens(pane)
	want := 2500
	if got != want {
		t.Errorf("CRLF pane: got %d, want %d", got, want)
	}
}
