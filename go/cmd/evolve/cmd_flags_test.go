// cmd_flags_test.go — L2.2 (concurrency-factory plan): `evolve flags
// generate|check` projects the flagregistry SSOT into the marker region of
// docs/architecture/control-flags.md; check exits 2 on drift so a flag can
// no longer ship undocumented.
package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
		"Hand-written prose ABOVE",   // prose preserved
		"GENERATED:flag-index BEGIN", // markers present
		"`EVOLVE_TRIAGE_CAP_GATE`",   // registry content rendered
	} {
		if !strings.Contains(string(doc), want) {
			t.Errorf("generated doc missing %q", want)
		}
	}
	if rc := runFlags([]string{"check"}, nil, io.Discard, os.Stderr); rc != 0 {
		t.Errorf("check after generate rc=%d, want 0 (round-trip)", rc)
	}
	// Idempotency: a second generate must not change the file.
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
	tampered := strings.Replace(string(doc), "`EVOLVE_TRIAGE_CAP_GATE`", "`EVOLVE_TAMPERED_FLAG`", 1)
	if err := os.WriteFile(p, []byte(tampered), 0o644); err != nil {
		t.Fatal(err)
	}
	if rc := runFlags([]string{"check"}, nil, io.Discard, io.Discard); rc != 2 {
		t.Errorf("check on drift rc=%d, want 2", rc)
	}
}

// TestSpliceMarkedRegion_EmptyAnchorAppendsAtEOF pins the flags-path splice
// contract directly: no markers + no fallback anchor ⇒ block appended at EOF.
func TestSpliceMarkedRegion_EmptyAnchorAppendsAtEOF(t *testing.T) {
	out, err := spliceMarkedRegion("# Doc\n\nprose\n", "BEGIN\nblock\nEND", "BEGIN", "END", "")
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
