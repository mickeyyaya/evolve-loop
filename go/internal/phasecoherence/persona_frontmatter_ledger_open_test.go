package phasecoherence

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestPersonaFrontmatter(t *testing.T) {
	agents := fstest.MapFS{
		"agents/evolve-builder.md":      {Data: []byte("---\nname: builder\ntools: [Read, Edit]\n---\nbody\n")},
		"agents/evolve-scout.md/nested": {Data: []byte("x")},
	}
	override := filepath.Join(t.TempDir(), "builder.md")
	if err := os.WriteFile(override, []byte("---\nname: overridden\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	missingOverride := filepath.Join(t.TempDir(), "absent.md")
	cases := []struct {
		name      string
		persona   string
		overrides map[string]string
		wantName  any
		wantNil   bool
		wantErr   error
	}{
		{name: "persona in the agents tree", persona: "builder", wantName: "builder"},
		{name: "override file wins", persona: "builder", overrides: map[string]string{"builder": override}, wantName: "overridden"},
		{name: "absent persona without override is silent", persona: "auditor", wantNil: true},
		{name: "empty override path for an absent file is silent", persona: "auditor", overrides: map[string]string{"auditor": ""}, wantNil: true},
		{name: "missing override file is an error", persona: "auditor", overrides: map[string]string{"auditor": missingOverride}, wantErr: fs.ErrNotExist},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fm, err := personaFrontmatter(Options{AgentsFS: agents, Overrides: tc.overrides}, tc.persona)

			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || fm != nil {
					t.Fatalf("= (%v, %v), want (nil, %v)", fm, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v", err)
			}
			if tc.wantNil != (fm == nil) {
				t.Fatalf("fm = %v, want nil=%v", fm, tc.wantNil)
			}
			if !tc.wantNil && fm["name"] != tc.wantName {
				t.Errorf("fm[name] = %v, want %v", fm["name"], tc.wantName)
			}
		})
	}
}

func TestPersonaFrontmatter_UnreadablePersonaIsAnError(t *testing.T) {
	agents := fstest.MapFS{"agents/evolve-scout.md/nested": {Data: []byte("x")}}

	fm, err := personaFrontmatter(Options{AgentsFS: agents}, "scout")

	if err == nil || errors.Is(err, fs.ErrNotExist) || fm != nil {
		t.Errorf("= (%v, %v), want (nil, a non-not-exist read error)", fm, err)
	}
}

func TestScanLedger_UnopenableLedgerIsAWrappedError(t *testing.T) {
	file := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	ledger := filepath.Join(file, "ledger.jsonl")

	scan, err := scanLedger(ledger, ProvenanceFields{Phase: "build", Cycle: 1})

	if err == nil || errors.Is(err, os.ErrNotExist) || !strings.HasPrefix(err.Error(), "open ledger "+ledger+":") {
		t.Fatalf("err = %v, want a wrapped open error that is not ErrNotExist", err)
	}
	if scan != (ledgerScan{}) {
		t.Errorf("scan = %+v, want zero", scan)
	}
}

func TestCheckProvenance_UnopenableLedgerFailsTheCheck(t *testing.T) {
	file := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVOLVE_LEDGER_OVERRIDE", filepath.Join(file, "ledger.jsonl"))

	_, err := CheckProvenance("<!-- evolve:provenance phase=build cycle=1 tree_sha=abc -->", ProvenanceFields{Phase: "build", Cycle: 1})

	if err == nil || !strings.Contains(err.Error(), "open ledger") {
		t.Errorf("err = %v, want the open-ledger failure surfaced", err)
	}
}
