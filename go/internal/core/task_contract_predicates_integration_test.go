//go:build integration

package core

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestListACSPredicates_ModuleAtTheWorktreeRootIsInventoried(t *testing.T) {
	t.Parallel()

	wt := t.TempDir()
	writeModuleFiles(t, wt, map[string]string{
		"go.mod":               "module example.com/rootmod\n\ngo 1.22\n",
		"acs/cycle3/p_test.go": "//go:build acs\n\npackage cycle3\n\nimport \"testing\"\n\nfunc TestC3_001(t *testing.T) {}\n",
	})

	got := listACSPredicates(context.Background(), wt, 3)

	if got.note != "" || strings.Join(got.names, ",") != "TestC3_001" {
		t.Fatalf("predicates = %+v, want TestC3_001 from the root module", got)
	}
}

func TestListACSPredicates_NoResolvableModuleIsALoudNote(t *testing.T) {
	saved := predicateModuleDir
	t.Cleanup(func() { predicateModuleDir = saved })
	var asked string
	predicateModuleDir = func(root string) string { asked = root; return "" }

	got := listACSPredicates(context.Background(), "/some/worktree", 5)

	if asked != "/some/worktree" || len(got.names) != 0 || got.note != "no Go module under the worktree — ACS predicates could not be listed" {
		t.Fatalf("listACSPredicates = %+v (asked %q), want the no-module note", got, asked)
	}
}

func TestListACSPredicates_LongCompilerOutputKeepsOnlyTheBoundedTail(t *testing.T) {
	t.Parallel()

	wt := t.TempDir()
	ident := "undefinedIdentifier" + strings.Repeat("Z", 600) + "End"
	writeModuleFiles(t, wt, map[string]string{
		"go/go.mod":               "module example.com/tailmod\n\ngo 1.22\n",
		"go/acs/cycle9/p_test.go": "//go:build acs\n\npackage cycle9\n\nimport \"testing\"\n\nfunc TestC9_001(t *testing.T) { " + ident + "() }\n",
	})

	got := listACSPredicates(context.Background(), wt, 9)

	_, tail, ok := strings.Cut(got.note, "): ")
	if !ok || len(got.names) != 0 || !strings.HasPrefix(got.note, "`go test -list` failed for ./acs/cycle9") {
		t.Fatalf("listACSPredicates = %+v, want the failed-list note", got)
	}
	body, truncated := strings.CutPrefix(tail, "…")
	if !truncated || len(body) != predicateNoteTailMax || !utf8.ValidString(tail) {
		t.Fatalf("tail = %d bytes (truncated %v), want \"…\" + the last %d bytes", len(body), truncated, predicateNoteTailMax)
	}
	if strings.Contains(body, "undefinedIdentifier") {
		t.Fatalf("tail %q kept the head of the output, want only its end", body)
	}
}

func TestListACSPredicates_CanceledContextIsAFailedListNote(t *testing.T) {
	t.Parallel()

	wt := t.TempDir()
	writeModuleFiles(t, wt, map[string]string{
		"go/go.mod":               "module example.com/cancelmod\n\ngo 1.22\n",
		"go/acs/cycle4/p_test.go": "//go:build acs\n\npackage cycle4\n\nimport \"testing\"\n\nfunc TestC4_001(t *testing.T) {}\n",
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got := listACSPredicates(ctx, wt, 4)

	if len(got.names) != 0 || !strings.HasPrefix(got.note, "`go test -list` failed for ./acs/cycle4 (context canceled)") {
		t.Fatalf("listACSPredicates = %+v, want the canceled failed-list note", got)
	}
}
