//go:build integration

package ledger

import (
	"errors"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

const patchIDTwoFileDiff = `diff --git a/a.txt b/a.txt
--- a/a.txt
+++ b/a.txt
@@ -1 +1,2 @@
 a
+a2
diff --git a/b.txt b/b.txt
--- a/b.txt
+++ b/b.txt
@@ -1 +1,2 @@
 b
+b2
`

const patchIDTwoFileDiffReordered = `diff --git a/b.txt b/b.txt
--- a/b.txt
+++ b/b.txt
@@ -1 +1,2 @@
 b
+b2
diff --git a/a.txt b/a.txt
--- a/a.txt
+++ b/a.txt
@@ -1 +1,2 @@
 a
+a2
`

func TestPatchID_IsAFortyHexIdentityIndependentOfFileOrder(t *testing.T) {
	inOrder, err := PatchID([]byte(patchIDTwoFileDiff))
	if err != nil {
		t.Fatalf("PatchID(in order): %v", err)
	}
	reordered, err := PatchID([]byte(patchIDTwoFileDiffReordered))
	if err != nil {
		t.Fatalf("PatchID(reordered): %v", err)
	}
	if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(inOrder) || inOrder == strings.Repeat("0", 40) {
		t.Fatalf("PatchID = %q, want a non-zero 40-hex patch identity", inOrder)
	}
	if inOrder != reordered {
		t.Fatalf("--stable identity depends on file order: %s vs %s", inOrder, reordered)
	}
}

func TestPatchID_NonDiffInputHasNoIdentity(t *testing.T) {
	for _, input := range []string{"", "\n", "not a diff at all\n", "--- a/x\n+++ b/x\n"} {
		got, err := PatchID([]byte(input))
		if err == nil || !strings.Contains(err.Error(), "empty output") || got != "" {
			t.Fatalf("PatchID(%q) = %q, %v; want the empty-output error", input, got, err)
		}
	}
}

func TestPatchID_MissingGitIsAWrappedError(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	got, err := PatchID([]byte(compTestDiff))

	if !errors.Is(err, exec.ErrNotFound) || !strings.HasPrefix(err.Error(), "git patch-id --stable: ") {
		t.Fatalf("err = %v, want a git patch-id error wrapping exec.ErrNotFound", err)
	}
	if got != "" {
		t.Fatalf("got = %q, want empty on error", got)
	}
}
