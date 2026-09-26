package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/internal/skillcheck"
)

// anyRenderedFlagToken returns the backticked name of the first registered flag,
// for assertions that "some registry content was rendered" without hardcoding a
// specific flag name, since the flag-reduction campaign continually deletes
// flags and a pinned name would eventually 404. Skips the test when the
// registry is empty.
func anyRenderedFlagToken(t *testing.T) string {
	t.Helper()
	if len(flagregistry.All) == 0 {
		t.Skip("flagregistry is empty; no rendered flag to assert against")
	}
	return "`" + flagregistry.All[0].Name + "`"
}

const flagsDocSeed = `# Control Flags Reference

Hand-written prose ABOVE the generated region must survive generation.

## Some Cluster

Narrative kept verbatim.
`

func seedFlagsProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "architecture")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "control-flags.md"), []byte(flagsDocSeed), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestFlagsGenerateThenCheck_RoundTrip(t *testing.T) {
	root := seedFlagsProject(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)

	if rc := runFlags([]string{"generate"}, nil, io.Discard, os.Stderr); rc != 0 {
		t.Fatalf("generate rc=%d, want 0", rc)
	}
	doc, err := os.ReadFile(filepath.Join(root, "docs", "architecture", "control-flags.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Hand-written prose ABOVE",
		"GENERATED:flag-index BEGIN",
		anyRenderedFlagToken(t),
	} {
		if !strings.Contains(string(doc), want) {
			t.Errorf("generated doc missing %q", want)
		}
	}
	if rc := runFlags([]string{"check"}, nil, io.Discard, os.Stderr); rc != 0 {
		t.Errorf("check after generate rc=%d, want 0 (round-trip)", rc)
	}
	before := string(doc)
	if rc := runFlags([]string{"generate"}, nil, io.Discard, os.Stderr); rc != 0 {
		t.Fatalf("second generate rc=%d", rc)
	}
	after, err := os.ReadFile(filepath.Join(root, "docs", "architecture", "control-flags.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != before {
		t.Error("generate is not idempotent")
	}
}

func TestFlagsCheck_DriftExitsTwo(t *testing.T) {
	root := seedFlagsProject(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	if rc := runFlags([]string{"generate"}, nil, io.Discard, os.Stderr); rc != 0 {
		t.Fatalf("generate rc=%d", rc)
	}
	p := filepath.Join(root, "docs", "architecture", "control-flags.md")
	doc, _ := os.ReadFile(p)
	flagToken := anyRenderedFlagToken(t)
	tampered := strings.Replace(string(doc), flagToken, "`EVOLVE_TAMPERED_FLAG`", 1)
	if tampered == string(doc) {
		t.Fatalf("tamper no-op: %q not found in generated doc", flagToken)
	}
	if err := os.WriteFile(p, []byte(tampered), 0o644); err != nil {
		t.Fatal(err)
	}
	if rc := runFlags([]string{"check"}, nil, io.Discard, io.Discard); rc != 2 {
		t.Errorf("check on drift rc=%d, want 2", rc)
	}
}

func TestSpliceMarkedRegion_EmptyAnchorAppendsAtEOF(t *testing.T) {
	out, err := skillcheck.SpliceMarkedRegion("# Doc\n\nprose\n", "BEGIN\nblock\nEND", "BEGIN", "END", "")
	if err != nil {
		t.Fatal(err)
	}
	if want := "# Doc\n\nprose\n\nBEGIN\nblock\nEND\n"; out != want {
		t.Errorf("EOF-append mismatch:\n%q\nwant\n%q", out, want)
	}
}

func TestFlagsUnknownSubcommand(t *testing.T) {
	if rc := runFlags([]string{"frobnicate"}, nil, io.Discard, io.Discard); rc != 10 {
		t.Errorf("unknown subcommand rc=%d, want 10", rc)
	}
}

func TestFlagsCheck_ResolvesWorktreeRootOverProjectRoot(t *testing.T) {
	worktree := seedFlagsProject(t) // brought in sync with the registry
	mainRoot := seedFlagsProject(t) // stays the bare seed → stale vs registry

	t.Setenv("EVOLVE_WORKTREE_ROOT", "")
	t.Setenv("EVOLVE_PROJECT_ROOT", worktree)
	if rc := runFlags([]string{"generate"}, nil, io.Discard, io.Discard); rc != 0 {
		t.Fatalf("seed generate rc=%d, want 0", rc)
	}

	t.Setenv("EVOLVE_PROJECT_ROOT", mainRoot)
	t.Setenv("EVOLVE_WORKTREE_ROOT", worktree)
	if rc := runFlags([]string{"check"}, nil, io.Discard, io.Discard); rc != 0 {
		t.Fatalf("check rc=%d, want 0 — flags check must resolve EVOLVE_WORKTREE_ROOT "+
			"(in-sync worktree), not EVOLVE_PROJECT_ROOT (stale main)", rc)
	}

	t.Setenv("EVOLVE_WORKTREE_ROOT", "")
	if rc := runFlags([]string{"check"}, nil, io.Discard, io.Discard); rc != 2 {
		t.Fatalf("check rc=%d with WORKTREE unset + stale PROJECT_ROOT, want 2 (drift); "+
			"test is not exercising the worktree redirect", rc)
	}
}
