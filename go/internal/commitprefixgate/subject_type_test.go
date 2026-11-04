package commitprefixgate

import "testing"

func TestSubjectType_IsTheTypeOfTheConventionalSubject(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"fix: a\n\nbody": "fix", "fix(gate): a": "fix", "fix!: a": "fix", "  feat(x-1)!: a": "feat",
		"fixes: a": "fixes", "Fix: a": "", "fix(A B): a": "", "no prefix": "", "": "",
	}
	for msg, want := range cases {
		if got := SubjectType(msg); got != want {
			t.Errorf("SubjectType(%q) = %q, want %q", msg, got, want)
		}
	}
}
