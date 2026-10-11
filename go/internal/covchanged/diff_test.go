package covchanged

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseDiff_ReturnsTheAddedLinesAndTheAnchorsOfRemovedCode(t *testing.T) {
	none := AddedLines{}
	for name, tc := range map[string]struct {
		diff string
		want Diff
	}{
		"an empty diff adds nothing": {"", Diff{none, none}},
		"a pure addition keeps every new line": {
			"diff --git a/p/a.go b/p/a.go\n--- a/p/a.go\n+++ b/p/a.go\n@@ -4,0 +5,3 @@ func F() {\n+\tx := 1\n+\ty := 2\n+\t_ = x + y\n",
			Diff{AddedLines{"p/a.go": {5, 6, 7}}, none},
		},
		"a hunk without counts adds one line": {
			"diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -3 +3 @@\n-old\n+new\n",
			Diff{AddedLines{"a.go": {3}}, AddedLines{"a.go": {3}}},
		},
		"a deletion-only hunk anchors the gap it leaves": {
			"diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -3,2 +2,0 @@\n-x\n-y\n",
			Diff{none, AddedLines{"a.go": {2}}},
		},
		"a deleted file adds nothing": {
			"diff --git a/gone.go b/gone.go\ndeleted file mode 100644\n--- a/gone.go\n+++ /dev/null\n@@ -1,2 +0,0 @@\n-package p\n-func F() {}\n",
			Diff{none, none},
		},
		"an added line that starts with +++ stays content": {
			"diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1,0 +2,2 @@\n+++ b/fake.go\n+x\n",
			Diff{AddedLines{"a.go": {2, 3}}, none},
		},
		"a quoted path is unquoted": {
			"diff --git \"a/sp ace.go\" \"b/sp ace.go\"\n--- \"a/sp ace.go\"\n+++ \"b/sp ace.go\"\n@@ -0,0 +1 @@\n+package p\n",
			Diff{AddedLines{"sp ace.go": {1}}, none},
		},
		"context and no-newline markers keep the count": {
			"diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1,3 +1,3 @@\n keep\n-old\n+new\n\\ No newline at end of file\n keep\n",
			Diff{AddedLines{"a.go": {2}}, AddedLines{"a.go": {2}}},
		},
		"a removed comment anchors nothing": {
			"+++ b/a.go\n@@ -4,2 +3,0 @@\n-\t// a note\n-\t/* and a block */\n",
			Diff{none, none},
		},
		"a removal at the top of the file anchors nothing": {
			"+++ b/a.go\n@@ -1 +0,0 @@\n-x := 1\n",
			Diff{none, none},
		},
		"two hunks and two files stay apart": {
			"diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1,0 +2 @@\n+a\n@@ -9,0 +11,2 @@\n+b\n+c\n" +
				"diff --git a/b.go b/b.go\nnew file mode 100644\n--- /dev/null\n+++ b/b.go\n@@ -0,0 +1 @@\n+d\n",
			Diff{AddedLines{"a.go": {2, 11, 12}, "b.go": {1}}, none},
		},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := ParseDiff([]byte(tc.diff))
			if err != nil {
				t.Fatalf("ParseDiff: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ParseDiff = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestParseDiff_RefusesAMalformedHunk(t *testing.T) {
	for name, diff := range map[string]string{
		"a header without a new side":     "+++ b/a.go\n@@ -1,2 @@\n",
		"a header with a huge start":      "+++ b/a.go\n@@ -1 +99999999999999999999 @@\n+x\n",
		"a header with a huge count":      "+++ b/a.go\n@@ -1 +1,99999999999999999999 @@\n+x\n",
		"a header with a huge old count":  "+++ b/a.go\n@@ -1,99999999999999999999 +1 @@\n+x\n",
		"a line the hunk did not count":   "+++ b/a.go\n@@ -1 +1 @@\n+x\n+y\n",
		"a line of no known kind":         "+++ b/a.go\n@@ -1 +1 @@\n?x\n",
		"a diff that ends inside a hunk":  "+++ b/a.go\n@@ -1 +1,3 @@\n+x\n",
		"a quoted path with a bad escape": "+++ \"b/a\\q.go\"\n",
	} {
		t.Run(name, func(t *testing.T) {
			if got, err := ParseDiff([]byte(diff)); err == nil {
				t.Errorf("ParseDiff = %v, nil; want an error", got)
			}
		})
	}
}

func FuzzParseDiff(f *testing.F) {
	f.Add("diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -4,0 +5,3 @@\n+x\n+y\n+z\n")
	f.Add("+++ b/a.go\n@@ -3 +3 @@\n-old\n+new\n")
	f.Add("+++ \"b/sp ace.go\"\n@@ -0,0 +1 @@\n+package p\n")
	f.Add("+++ /dev/null\n@@ -1,2 +0,0 @@\n-a\n-b\n")
	f.Add("+++ b/a.go\n@@ -1,3 +1,3 @@\n keep\n-old\n+new\n\\ No newline\n keep\n")
	f.Fuzz(func(t *testing.T, diff string) {
		got, err := ParseDiff([]byte(diff))
		if err != nil {
			return
		}
		for _, set := range []AddedLines{got.Added, got.Removed} {
			for file, lines := range set {
				if file == "" {
					t.Fatalf("an empty path carries lines %v", lines)
				}
				for _, l := range lines {
					if l < 1 {
						t.Fatalf("%s lines %v hold a line below 1", file, lines)
					}
				}
			}
		}
		if !strings.Contains(diff, "\n+") && len(got.Added) != 0 {
			t.Fatalf("a diff without an added line returned %v", got.Added)
		}
		if !strings.Contains(diff, "\n-") && len(got.Removed) != 0 {
			t.Fatalf("a diff without a removed line returned %v", got.Removed)
		}
	})
}
