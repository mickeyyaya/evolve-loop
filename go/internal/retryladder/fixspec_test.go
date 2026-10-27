package retryladder_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
)

const validSpec = `fix-spec: 1
expected: the guard refuses a negative span
edit: go/internal/foo/a.go:10-12
> func guard(n int) error {
> 	if n < 0 {
> 		return errNegative
> 	}
edit: go/internal/foo/b.go:5-5
>
> var errNegative = errors.New("negative")
`

func TestParseFixSpec_ReadsLocationBehaviorAndReplacement(t *testing.T) {
	got, err := retryladder.ParseFixSpec(validSpec)
	if err != nil {
		t.Fatalf("ParseFixSpec(valid) error = %v", err)
	}
	want := retryladder.FixSpec{
		Expected: "the guard refuses a negative span",
		Edits: []retryladder.FixEdit{
			{File: "go/internal/foo/a.go", FromLine: 10, ToLine: 12, Replacement: []string{
				"func guard(n int) error {", "\tif n < 0 {", "\t\treturn errNegative", "\t}",
			}},
			{File: "go/internal/foo/b.go", FromLine: 5, ToLine: 5, Replacement: []string{"", `var errNegative = errors.New("negative")`}},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseFixSpec(valid) = %+v, want %+v", got, want)
	}
}

func TestParseFixSpec_IgnoresBlankSeparatorsAndCarriageReturns(t *testing.T) {
	text := "\r\nfix-spec: 1\r\n\r\nexpected: ok\r\n\r\nedit: a.go:1-1\r\n> x\r\n\r\n"
	got, err := retryladder.ParseFixSpec(text)
	if err != nil {
		t.Fatalf("ParseFixSpec(crlf) error = %v", err)
	}
	if len(got.Edits) != 1 || !reflect.DeepEqual(got.Edits[0].Replacement, []string{"x"}) {
		t.Fatalf("ParseFixSpec(crlf) edits = %+v, want one edit replacing with [x]", got.Edits)
	}
}

func TestParseFixSpec_RefusesMalformedSpecs(t *testing.T) {
	cases := []struct {
		name string
		text string
	}{
		{"empty", ""},
		{"blank only", "\n\n"},
		{"missing header", "expected: ok\nedit: a.go:1-1\n> x\n"},
		{"unknown version", "fix-spec: 2\nexpected: ok\nedit: a.go:1-1\n> x\n"},
		{"missing expected", "fix-spec: 1\nedit: a.go:1-1\n> x\n"},
		{"empty expected", "fix-spec: 1\nexpected:   \nedit: a.go:1-1\n> x\n"},
		{"duplicate expected", "fix-spec: 1\nexpected: a\nexpected: b\nedit: a.go:1-1\n> x\n"},
		{"no edit", "fix-spec: 1\nexpected: ok\n"},
		{"edit without replacement", "fix-spec: 1\nexpected: ok\nedit: a.go:1-1\n"},
		{"edit without replacement before next edit", "fix-spec: 1\nexpected: ok\nedit: a.go:1-1\nedit: b.go:2-2\n> x\n"},
		{"range without end", "fix-spec: 1\nexpected: ok\nedit: a.go:3\n> x\n"},
		{"range reversed", "fix-spec: 1\nexpected: ok\nedit: a.go:9-3\n> x\n"},
		{"range starts at zero", "fix-spec: 1\nexpected: ok\nedit: a.go:0-3\n> x\n"},
		{"range not a number", "fix-spec: 1\nexpected: ok\nedit: a.go:one-3\n> x\n"},
		{"file missing", "fix-spec: 1\nexpected: ok\nedit: :1-1\n> x\n"},
		{"absolute path", "fix-spec: 1\nexpected: ok\nedit: /etc/passwd:1-1\n> x\n"},
		{"parent traversal", "fix-spec: 1\nexpected: ok\nedit: go/../../etc/passwd:1-1\n> x\n"},
		{"stray line", "fix-spec: 1\nexpected: ok\nnote: hi\nedit: a.go:1-1\n> x\n"},
		{"unprefixed replacement line", "fix-spec: 1\nexpected: ok\nedit: a.go:1-1\n> x\nreturn nil\n"},
		{"replacement before any edit", "fix-spec: 1\nexpected: ok\n> x\nedit: a.go:1-1\n> y\n"},
		{"expected after an edit without replacement", "fix-spec: 1\nedit: a.go:1-1\nexpected: ok\n"},
		{"edit location without a colon", "fix-spec: 1\nexpected: ok\nedit: a.go\n> x\n"},
		{"overlapping edits in one file", "fix-spec: 1\nexpected: ok\nedit: a.go:1-5\n> x\nedit: a.go:5-8\n> y\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := retryladder.ParseFixSpec(tc.text)
			if !errors.Is(err, retryladder.ErrMalformedFixSpec) {
				t.Fatalf("ParseFixSpec error = %v, want ErrMalformedFixSpec", err)
			}
			if !reflect.DeepEqual(got, retryladder.FixSpec{}) {
				t.Fatalf("ParseFixSpec returned %+v with an error, want the zero spec", got)
			}
		})
	}
}

func TestParseFixSpec_NamesTheOffendingLine(t *testing.T) {
	_, err := retryladder.ParseFixSpec("fix-spec: 1\nexpected: ok\nedit: a.go:9-3\n> x\n")
	if err == nil || !strings.Contains(err.Error(), "line 3") {
		t.Fatalf("ParseFixSpec error = %v, want it to name line 3", err)
	}
}

func TestParseFixSpec_AllowsTwoEditsInOneFileWhenDisjoint(t *testing.T) {
	text := "fix-spec: 1\nexpected: ok\nedit: a.go:1-2\n> x\nedit: a.go:3-4\n> y\n"
	got, err := retryladder.ParseFixSpec(text)
	if err != nil || len(got.Edits) != 2 {
		t.Fatalf("ParseFixSpec(disjoint) = %+v, %v, want two edits and no error", got, err)
	}
}

func scopeSpec(t *testing.T) retryladder.FixSpec {
	t.Helper()
	spec, err := retryladder.ParseFixSpec("fix-spec: 1\nexpected: ok\nedit: go/a.go:10-12\n> x\nedit: go/b.go:40-40\n> y\n")
	if err != nil {
		t.Fatalf("scope spec: %v", err)
	}
	return spec
}

func diffOf(file string, hunks ...string) string {
	return "diff --git a/" + file + " b/" + file + "\n--- a/" + file + "\n+++ b/" + file + "\n" + strings.Join(hunks, "\n") + "\n"
}

func TestFixScopeViolations_AcceptsAChangeInsideTheCitedLines(t *testing.T) {
	diff := diffOf("go/a.go", "@@ -10,3 +10,4 @@\n-old\n+new") + diffOf("go/b.go", "@@ -40 +40 @@\n-old\n+new")
	got, err := retryladder.FixScopeViolations(scopeSpec(t), diff)
	if err != nil || len(got) != 0 {
		t.Fatalf("FixScopeViolations = %+v, %v, want none", got, err)
	}
}

func TestFixScopeViolations_ReportsEachFileOutsideTheSpecOnce(t *testing.T) {
	diff := diffOf("go/c.go", "@@ -1 +1 @@\n-a\n+b", "@@ -9 +9 @@\n-a\n+b") + diffOf("go/a.go", "@@ -11 +11 @@\n-a\n+b")
	got, err := retryladder.FixScopeViolations(scopeSpec(t), diff)
	if err != nil {
		t.Fatalf("FixScopeViolations error = %v", err)
	}
	want := []retryladder.ScopeViolation{{Code: retryladder.ScopeFileOutside, File: "go/c.go"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FixScopeViolations = %+v, want %+v", got, want)
	}
}

func TestFixScopeViolations_ReportsAHunkOutsideTheCitedLines(t *testing.T) {
	cases := []struct {
		name string
		hunk string
		from int
		to   int
	}{
		{"before the span", "@@ -9 +9 @@\n-a\n+b", 9, 9},
		{"after the span", "@@ -13 +13 @@\n-a\n+b", 13, 13},
		{"straddles the end", "@@ -12,2 +12,2 @@\n-a\n-b\n+c\n+d", 12, 13},
		{"straddles the start", "@@ -9,2 +9,2 @@\n-a\n-b\n+c\n+d", 9, 10},
		{"insertion well after the span", "@@ -20,0 +21 @@\n+b", 20, 20},
		{"insertion well before the span", "@@ -3,0 +4 @@\n+b", 3, 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := retryladder.FixScopeViolations(scopeSpec(t), diffOf("go/a.go", tc.hunk))
			if err != nil {
				t.Fatalf("FixScopeViolations error = %v", err)
			}
			want := []retryladder.ScopeViolation{{Code: retryladder.ScopeLinesOutside, File: "go/a.go", FromLine: tc.from, ToLine: tc.to}}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("FixScopeViolations = %+v, want %+v", got, want)
			}
		})
	}
}

func TestFixScopeViolations_AcceptsAnInsertionAtTheEdgesOfTheSpan(t *testing.T) {
	for _, hunk := range []string{"@@ -9,0 +10 @@\n+b", "@@ -12,0 +13 @@\n+b", "@@ -11,0 +12 @@\n+b"} {
		got, err := retryladder.FixScopeViolations(scopeSpec(t), diffOf("go/a.go", hunk))
		if err != nil || len(got) != 0 {
			t.Fatalf("FixScopeViolations(%q) = %+v, %v, want none", hunk, got, err)
		}
	}
}

func TestFixScopeViolations_ChecksEachEditOfAFileSeparately(t *testing.T) {
	spec, err := retryladder.ParseFixSpec("fix-spec: 1\nexpected: ok\nedit: a.go:1-2\n> x\nedit: a.go:10-11\n> y\n")
	if err != nil {
		t.Fatalf("spec: %v", err)
	}
	inside := diffOf("a.go", "@@ -2,2 +2,2 @@\n-a\n-b\n+c\n+d")
	got, err := retryladder.FixScopeViolations(spec, inside)
	if err != nil || len(got) != 1 || got[0].Code != retryladder.ScopeLinesOutside {
		t.Fatalf("a hunk across the gap between two edits = %+v, %v, want one lines_out_of_scope", got, err)
	}
	both := diffOf("a.go", "@@ -1,2 +1,2 @@\n-a\n-b\n+c\n+d", "@@ -11 +11 @@\n-a\n+b")
	got, err = retryladder.FixScopeViolations(spec, both)
	if err != nil || len(got) != 0 {
		t.Fatalf("one hunk in each edit = %+v, %v, want none", got, err)
	}
}

func TestFixScopeViolations_TreatsACreatedOrDeletedUnnamedFileAsOutside(t *testing.T) {
	created := "diff --git a/go/new.go b/go/new.go\nnew file mode 100644\n--- /dev/null\n+++ b/go/new.go\n@@ -0,0 +1,2 @@\n+a\n+b\n"
	deleted := "diff --git a/go/old.go b/go/old.go\ndeleted file mode 100644\n--- a/go/old.go\n+++ /dev/null\n@@ -1,2 +0,0 @@\n-a\n-b\n"
	for name, diff := range map[string]string{"created": created, "deleted": deleted} {
		got, err := retryladder.FixScopeViolations(scopeSpec(t), diff)
		if err != nil || len(got) != 1 || got[0].Code != retryladder.ScopeFileOutside {
			t.Fatalf("%s file = %+v, %v, want one file_out_of_scope", name, got, err)
		}
	}
}

func TestFixScopeViolations_RefusesARenameThatLeavesTheSpec(t *testing.T) {
	diff := "diff --git a/go/a.go b/go/moved.go\nsimilarity index 100%\nrename from go/a.go\nrename to go/moved.go\n"
	got, err := retryladder.FixScopeViolations(scopeSpec(t), diff)
	if err != nil || len(got) != 1 || got[0].Code != retryladder.ScopeFileOutside || got[0].File != "go/moved.go" {
		t.Fatalf("rename out of the spec = %+v, %v, want file_out_of_scope on go/moved.go", got, err)
	}
}

func TestFixScopeViolations_EmptyDiffHasNoViolations(t *testing.T) {
	got, err := retryladder.FixScopeViolations(scopeSpec(t), "")
	if err != nil || len(got) != 0 {
		t.Fatalf("empty diff = %+v, %v, want none", got, err)
	}
}

func TestFixScopeViolations_RefusesADiffItCannotRead(t *testing.T) {
	cases := map[string]string{
		"hunk without a file":         "@@ -1 +1 @@\n-a\n+b\n",
		"unreadable hunk range":       "diff --git a/go/a.go b/go/a.go\n@@ -x,y +1 @@\n-a\n+b\n",
		"file header without b/ path": "diff --git nothing\n",
		"hunk start overflows":        "diff --git a/go/a.go b/go/a.go\n@@ -99999999999999999999 +1 @@\n-a\n+b\n",
		"hunk length overflows":       "diff --git a/go/a.go b/go/a.go\n@@ -1,99999999999999999999 +1 @@\n-a\n+b\n",
	}
	for name, diff := range cases {
		got, err := retryladder.FixScopeViolations(scopeSpec(t), diff)
		if !errors.Is(err, retryladder.ErrMalformedFixDiff) || got != nil {
			t.Fatalf("%s = %+v, %v, want nil and ErrMalformedFixDiff", name, got, err)
		}
	}
}
