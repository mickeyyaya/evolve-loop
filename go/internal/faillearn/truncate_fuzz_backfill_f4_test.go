package faillearn

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func FuzzTruncateRunes(f *testing.F) {
	for _, seed := range []string{"héllo", "日本語", "", "plain ascii text", "a\xffb"} {
		f.Add(seed, 3)
	}
	f.Fuzz(func(t *testing.T, s string, max int) {
		if max < 0 || max > 1<<12 {
			return
		}
		got := truncateRunes(s, max)
		if !strings.HasPrefix(s, got) {
			t.Fatalf("truncateRunes(%q,%d) = %q is not a prefix", s, max, got)
		}
		if len(s) <= max && got != s {
			t.Fatalf("a string within the byte budget is returned whole: %q -> %q", s, got)
		}
		if utf8.ValidString(s) && utf8.RuneCountInString(got) > max {
			t.Fatalf("truncateRunes(%q,%d) = %q exceeds the rune budget", s, max, got)
		}
	})
}
