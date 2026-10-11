//go:build integration

package core

import (
	"errors"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

const compositionPatchIDTwoFileDiff = `diff --git a/a.txt b/a.txt
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

const compositionPatchIDTwoFileDiffReordered = `diff --git a/b.txt b/b.txt
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

func TestCompositionPatchID_IsAFortyHexIdentityIndependentOfFileOrder(t *testing.T) {
	inOrder, err := compositionPatchID([]byte(compositionPatchIDTwoFileDiff))
	if err != nil {
		t.Fatalf("compositionPatchID(in order): %v", err)
	}
	reordered, err := compositionPatchID([]byte(compositionPatchIDTwoFileDiffReordered))
	if err != nil {
		t.Fatalf("compositionPatchID(reordered): %v", err)
	}
	if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(inOrder) || inOrder == strings.Repeat("0", 40) {
		t.Fatalf("PatchID = %q, want a non-zero 40-hex patch identity", inOrder)
	}
	if inOrder != reordered {
		t.Fatalf("--stable identity depends on file order: %s vs %s", inOrder, reordered)
	}
}

func TestCompositionPatchID_NonDiffInputHasNoIdentity(t *testing.T) {
	for _, input := range []string{"", "\n", "not a diff at all\n", "--- a/x\n+++ b/x\n"} {
		got, err := compositionPatchID([]byte(input))
		if err == nil || !strings.Contains(err.Error(), "empty output") || got != "" {
			t.Fatalf("compositionPatchID(%q) = %q, %v; want the empty-output error", input, got, err)
		}
	}
}

func TestCompositionPatchID_MissingGitIsAWrappedError(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	got, err := compositionPatchID([]byte(compositionPatchIDOneFileDiff))

	if !errors.Is(err, exec.ErrNotFound) || !strings.HasPrefix(err.Error(), "git patch-id --stable: ") {
		t.Fatalf("err = %v, want a git patch-id error wrapping exec.ErrNotFound", err)
	}
	if got != "" {
		t.Fatalf("got = %q, want empty on error", got)
	}
}

const compositionPatchIDOneFileDiff = `diff --git a/fixture.txt b/fixture.txt
--- a/fixture.txt
+++ b/fixture.txt
@@ -1 +1,2 @@
 fixture line 1
+lane change
`
