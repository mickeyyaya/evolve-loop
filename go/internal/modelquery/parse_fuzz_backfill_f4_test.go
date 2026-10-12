package modelquery

import "testing"

func FuzzParseDate(f *testing.F) {
	for _, seed := range []string{"claude-opus-2025-07-31", "m-2025-07", "m-2025", "latest", "", "2025-99-99", "x-1999-01-01-y"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, id string) {
		got := parseDate(id)
		if !got.ok && got.value != 0 {
			t.Fatalf("parseDate(%q) = %+v: a failed parse must be the zero value", id, got)
		}
		if got.ok && (got.value < 19000000 || got.value > 20999999) {
			t.Fatalf("parseDate(%q) = %+v: the year is pinned to 19xx or 20xx", id, got)
		}
	})
}

func FuzzEnumTokens(f *testing.F) {
	for _, seed := range []string{"low, medium, high", "low|high", "low", "", ",|,", "low, 9bad"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, inner string) {
		got := enumTokens(inner)
		if got != nil && len(got) < minEnumTokens {
			t.Fatalf("enumTokens(%q) = %v: an enum has at least %d tokens", inner, got, minEnumTokens)
		}
		for _, tok := range got {
			if !effortToken.MatchString(tok) {
				t.Fatalf("enumTokens(%q) returned the malformed token %q", inner, tok)
			}
		}
	})
}
