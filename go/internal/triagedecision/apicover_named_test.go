package triagedecision

import (
	"strings"
	"testing"
)

// TestAPICoverNamedExports names every exported symbol through the shapes its consumers use: the host
// derivation (Derive), ship's projection (Project), and triagecap's card scans (the parser).
func TestAPICoverNamedExports(t *testing.T) {
	raw, err := Derive([]byte(report1707), 1707, nil)
	if err != nil || len(raw) == 0 {
		t.Fatalf("Derive = (%d bytes, %v)", len(raw), err)
	}
	if raw, err = Project(report1707, 1707); err != nil || len(raw) == 0 {
		t.Fatalf("Project = (%d bytes, %v)", len(raw), err)
	}
	body, ok := SectionBody(personaReport, "top_n")
	if !ok {
		t.Fatal("SectionBody: no top_n")
	}
	var sec Section = ParseSection(body)
	var it Item = sec.Items[0]
	if ActionOf(it.Rest) != "Cover store.Write error branches" {
		t.Fatalf("ActionOf = %q", ActionOf(it.Rest))
	}
	if ReasonOf("x — reason=stale") != "stale" {
		t.Fatal("ReasonOf")
	}
	tokens, stripped := SplitDeclaredFiles(it.Rest)
	if len(tokens) != 2 || strings.Contains(stripped, "files=") {
		t.Fatalf("SplitDeclaredFiles = (%v, %q)", tokens, stripped)
	}
	if got := FilesOf(it.Rest); len(got) != 2 {
		t.Fatalf("FilesOf = %v", got)
	}
	if p, ok := DeclaredFilePath("[go/a.go],"); !ok || p != "go/a.go" {
		t.Fatalf("DeclaredFilePath = (%q, %v)", p, ok)
	}
}
