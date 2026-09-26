package faillearn

import (
	"reflect"
	"testing"
)

func TestTokenizeObservation_SplitsOnNonASCIIAlphanumericsAndDropsBareNumbers(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want []string
	}{
		{"cycle-mid-execution-FAIL", []string{"cycle", "mid", "execution", "fail"}},
		{"cycle mid execution fail", []string{"cycle", "mid", "execution", "fail"}},
		{"cycle 1694 failed 3 times", []string{"cycle", "failed", "times"}},
		{"gpt5 a1b2", []string{"gpt5", "a1b2"}},
		{"fizz 10x 9lives", []string{"fizz", "10x", "9lives"}},
		{"café_au-lait", []string{"caf", "au", "lait"}},
		{"", []string{}},
	} {
		if got := tokenizeObservation(tc.in); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("tokenizeObservation(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
