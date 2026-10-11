package inboxbatch

import (
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestFieldRewritesClean(t *testing.T) {
	cases := []struct {
		name          string
		in            string
		max           int
		want          string
		stripped      bool
		truncated     bool
		changedRecord bool
	}{
		{"untouched", "plain", 10, "plain", false, false, false},
		{"empty", "", 0, "", false, false, false},
		{"exactly at the bound", "abcde", 5, "abcde", false, false, false},
		{"one over the bound", "abcdef", 5, "abcde", false, true, true},
		{"cut lands inside a two-byte rune", "aé", 2, "a", false, true, true},
		{"cut lands inside a three-byte rune", "ab中", 4, "ab", false, true, true},
		{"cut lands inside a four-byte rune", "😀😀", 7, "😀", false, true, true},
		{"cut on a rune start keeps it whole", "é中", 2, "é", false, true, true},
		{"only a partial rune fits", "中", 1, "", false, true, true},
		{"control chars become spaces", "a\tb\x7f", 10, "a b ", true, false, true},
		{"strip then truncate", "\n\n\nabc", 4, "   a", true, true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var r fieldRewrites

			got := r.clean("summary", tc.in, tc.max)

			if got != tc.want {
				t.Errorf("clean(%q, %d) = %q, want %q", tc.in, tc.max, got, tc.want)
			}
			if has := reflect.DeepEqual(r.stripped, []string{"summary"}); has != tc.stripped {
				t.Errorf("stripped = %v, want recorded=%v", r.stripped, tc.stripped)
			}
			if has := reflect.DeepEqual(r.truncated, []string{"summary"}); has != tc.truncated {
				t.Errorf("truncated = %v, want recorded=%v", r.truncated, tc.truncated)
			}
			if has := reflect.DeepEqual(r.changed, []string{"summary"}); has != tc.changedRecord {
				t.Errorf("changed = %v, want recorded=%v", r.changed, tc.changedRecord)
			}
		})
	}
}

func TestFieldRewritesClean_RecordsAFieldOnceAcrossCalls(t *testing.T) {
	var r fieldRewrites

	r.clean("title", "a\x01", 1)
	r.clean("title", "b\x02", 1)
	r.clean("id", "ok", 5)

	want := fieldRewrites{changed: []string{"title"}, stripped: []string{"title"}, truncated: []string{"title"}}
	if !reflect.DeepEqual(r, want) {
		t.Errorf("rewrites = %+v, want %+v", r, want)
	}
}

func FuzzFieldRewritesClean_StaysValidBoundedAndAPrefix(f *testing.F) {
	f.Add("aé", 2)
	f.Add("😀😀", 7)
	f.Add("\x80\x80\x80", 2)
	f.Add("ctrl\x00chars", 3)
	f.Fuzz(func(t *testing.T, s string, max int) {
		if max < 0 || max > 1<<12 {
			return
		}
		var r fieldRewrites

		got := r.clean("f", s, max)

		if len(got) > max {
			t.Fatalf("len(%q) = %d > max %d", got, len(got), max)
		}
		if !utf8.ValidString(got) {
			t.Fatalf("clean(%q) = %q is not valid UTF-8", s, got)
		}
		if !strings.HasPrefix(StripControl(s), got) {
			t.Fatalf("clean(%q) = %q is not a prefix of the stripped input", s, got)
		}
		if (got != s) != (len(r.changed) == 1) {
			t.Fatalf("changed = %v for %q -> %q", r.changed, s, got)
		}
	})
}
