package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWarnSelectorIgnored_NamesTheSequentialPathOnlyWhenASelectorIsSet(t *testing.T) {
	t.Parallel()
	var none bytes.Buffer
	warnSelectorIgnored(loopConfig{}, &none)
	if none.Len() != 0 {
		t.Fatalf("no selector: warning %q, want none", none.String())
	}
	for _, cfg := range []loopConfig{{Only: []string{"a-b"}}, {Exclude: []string{"a-b"}}} {
		var out bytes.Buffer
		warnSelectorIgnored(cfg, &out)
		if !strings.Contains(out.String(), "sequential") {
			t.Errorf("selector %+v: warning %q, want one naming the sequential path", cfg, out.String())
		}
	}
}

func TestRefuseSelector_AnUnreadableInboxWarnsAndLetsTheLaunchProceed(t *testing.T) {
	t.Parallel()
	evolveDir := filepath.Join(t.TempDir(), ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "inbox"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer

	rc := refuseSelector(loopConfig{EvolveDir: evolveDir, Only: []string{"a-b"}}, &stderr)

	if rc != 0 || !strings.Contains(stderr.String(), "cannot check the items --only names") {
		t.Fatalf("rc=%d stderr=%q, want rc 0 and the unreadable-inbox warning", rc, stderr.String())
	}
}
