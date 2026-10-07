package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/ledgerartifacts"
)

func TestGC_TheEnforceHookAndTheOperatorRunKeepTheLedgerEvidenceStore(t *testing.T) {
	f := newEnforceSafetyFixture(t, checkedInGCPolicy(t))
	t.Setenv("TMPDIR", t.TempDir())
	body := []byte("diff --git a/lane.txt b/lane.txt\n")
	digest, err := ledgerartifacts.Open(f.evolveDir).Put(body)
	if err != nil {
		t.Fatal(err)
	}
	object, err := ledgerartifacts.Open(f.evolveDir).Path(digest)
	if err != nil {
		t.Fatal(err)
	}
	longAgo := time.Now().Add(-90 * 24 * time.Hour)
	for p := object; p != f.evolveDir; p = filepath.Dir(p) {
		if err := os.Chtimes(p, longAgo, longAgo); err != nil {
			t.Fatal(err)
		}
	}

	var hookErr bytes.Buffer
	runGCHook(loopConfig{EvolveDir: f.evolveDir, ProjectRoot: f.projectRoot}, f.workspace, &hookErr)
	var stdout, stderr bytes.Buffer
	if rc := runGC([]string{"--project-root", f.projectRoot}, nil, &stdout, &stderr); rc == 10 {
		t.Fatalf("runGC rejected its arguments: %s", stderr.String())
	}

	if got, err := ledgerartifacts.Open(f.evolveDir).Get(digest); err != nil || !bytes.Equal(got, body) {
		t.Fatalf("gc released a stored carry diff (%v): every later carry would be refused\nhook:\n%s\ngc:\n%s%s", err, hookErr.String(), stdout.String(), stderr.String())
	}
}
