package main

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHostInboxClaimer_RefusesAnInboxOffTheShape(t *testing.T) {
	t.Parallel()
	_, existed := os.Stat(".evolve")
	t.Cleanup(func() {
		if _, err := os.Stat(".evolve"); err == nil && existed != nil {
			_ = os.RemoveAll(".evolve")
			t.Errorf("the claim wrote .evolve/ under the process directory")
		}
	})
	inbox := filepath.Join(t.TempDir(), "queue")
	item := filepath.Join(inbox, "item-a.json")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(item, []byte(`{"id":"item-a"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	err := hostInboxClaimer(nil, nil, nil)(inbox, 8, []string{"item-a"})

	if err == nil {
		t.Fatal("a claim on an inbox that is not <root>/.evolve/inbox must be refused")
	}
	if _, serr := os.Stat(item); serr != nil {
		t.Errorf("the item moved although the claim was refused: %v", serr)
	}
}

func TestResolveGCProjectRoot_AnExplicitRelativeRootBecomesAbsolute(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	root, code, ok := resolveGCProjectRoot(".", false, &stderr)
	if !ok || code != 0 || !filepath.IsAbs(root) {
		t.Fatalf("root=%q code=%d ok=%v, want an absolute root", root, code, ok)
	}
}

func TestContinuationRoot_AnExplicitRelativeRootBecomesAbsolute(t *testing.T) {
	t.Parallel()
	fs := flag.NewFlagSet("continuation", flag.ContinueOnError)
	rootFlag := fs.String("project-root", "", "")
	var stderr bytes.Buffer
	root, ok := continuationRoot(fs, []string{"--project-root", "."}, rootFlag, &stderr)
	if !ok || !filepath.IsAbs(root) {
		t.Fatalf("root=%q ok=%v, want an absolute root", root, ok)
	}
}

func TestRootWarner_NamesTheVerbAndTheMessage(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	rootWarner("evolve gc", &stderr)("could not resolve")
	if got, want := stderr.String(), "evolve gc: WARN: could not resolve\n"; got != want {
		t.Errorf("warning = %q, want %q", got, want)
	}
}

func TestContinuationRoot_AMissingWorkingDirectoryIsNotARoot(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", "")
	prev := processGetwd
	t.Cleanup(func() { processGetwd = prev })
	processGetwd = func() (string, error) { return "", errors.New("gone") }
	fs := flag.NewFlagSet("continuation", flag.ContinueOnError)
	rootFlag := fs.String("project-root", "", "")
	var stderr bytes.Buffer

	root, ok := continuationRoot(fs, nil, rootFlag, &stderr)

	if ok || root != "" || !strings.Contains(stderr.String(), "cannot resolve the project root") {
		t.Errorf("root=%q ok=%v stderr=%q, want a refusal that names the cause", root, ok, stderr.String())
	}
}

func TestContinuationForScope_ARelativeRootIsRefusedNotResolved(t *testing.T) {
	t.Parallel()
	if got := continuationForScope("relative-root", 3, []string{"absent-task"}); got != nil {
		t.Errorf("continuationForScope = %+v, want nil for a root that is not absolute", got)
	}
}
