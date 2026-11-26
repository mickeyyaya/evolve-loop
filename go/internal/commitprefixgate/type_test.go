package commitprefixgate

import "testing"

func TestType_ReadsTheConventionalTypeOfASubject(t *testing.T) {
	tests := map[string]string{
		"fix: a":             "fix",
		"fix(scope): a":      "fix",
		"fix!: a":            "fix",
		"fix(scope)!: a":     "fix",
		"  feat: a":          "feat",
		"docs-site: a":       "docs-site",
		"no prefix here":     "",
		"Fix: capitalised":   "",
		"fixed the thing: x": "",
		"":                   "",
	}
	for subject, want := range tests {
		if got := Type(subject); got != want {
			t.Errorf("Type(%q) = %q, want %q", subject, got, want)
		}
	}
}
