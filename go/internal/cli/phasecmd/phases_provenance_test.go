package phasecmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func provenanceFixture(t *testing.T, buildReport string) {
	t.Helper()
	root := t.TempDir()
	registry, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "docs", "architecture", "phase-registry.json"))
	if err != nil {
		t.Fatal(err)
	}
	for rel, body := range map[string]string{
		"docs/architecture/phase-registry.json": string(registry),
		".evolve/runs/cycle-5/build-report.md":  buildReport,
	} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	t.Setenv("EVOLVE_LEDGER_OVERRIDE", "")
}

func TestPhasesCheckProvenance_MissingHeaderWarnsAndPasses(t *testing.T) {
	provenanceFixture(t, "# Build Report\n")
	var stdout, stderr bytes.Buffer
	if rc := RunPhases([]string{"check-provenance", "--cycle", "5"}, nil, &stdout, &stderr); rc != 0 {
		t.Fatalf("rc = %d, want 0 for a WARN-only finding; stdout=%q stderr=%q", rc, stdout.String(), stderr.String())
	}
	want := "WARN: .evolve/runs/cycle-5/build-report.md: missing evolve:provenance header"
	if !strings.Contains(stdout.String(), want) || !strings.Contains(stdout.String(), "1 artifact(s) checked, 1 provenance finding(s)") {
		t.Errorf("stdout = %q, want the WARN line and the count line", stdout.String())
	}
}

func TestPhasesCheckProvenance_RejectsStrayPositional(t *testing.T) {
	provenanceFixture(t, "# Build Report\n")
	var stdout, stderr bytes.Buffer
	if rc := RunPhases([]string{"check-provenance", "--cycle", "5", "extra"}, nil, &stdout, &stderr); rc != 10 {
		t.Errorf("rc = %d, want 10 for a stray positional; stderr=%q", rc, stderr.String())
	}
}

func TestParsePersonaOverride(t *testing.T) {
	rows := []struct {
		value   string
		want    map[string]string
		wantErr bool
	}{
		{"", map[string]string{}, false},
		{"/tmp/p.md:widget", map[string]string{"widget": "/tmp/p.md"}, false},
		{"/tmp/p.md", nil, true},
		{":widget", nil, true},
		{"/tmp/p.md:", nil, true},
	}
	for _, row := range rows {
		got, err := parsePersonaOverride(row.value)
		if (err != nil) != row.wantErr {
			t.Errorf("parsePersonaOverride(%q) err = %v, wantErr %v", row.value, err, row.wantErr)
			continue
		}
		if !row.wantErr && (len(got) != len(row.want) || got["widget"] != row.want["widget"]) {
			t.Errorf("parsePersonaOverride(%q) = %v, want %v", row.value, got, row.want)
		}
	}
}
