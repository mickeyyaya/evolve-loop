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

func TestRefuseSelector_FocusIDsThatNoLaneCanTakeWarnAndLetTheLaunchProceed(t *testing.T) {
	t.Parallel()
	evolveDir := filepath.Join(t.TempDir(), ".evolve")
	if err := os.MkdirAll(filepath.Join(evolveDir, "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer

	rc := refuseSelector(loopConfig{EvolveDir: evolveDir, Focus: []string{"ghost-id"}}, &stderr)

	if rc != 0 || !strings.Contains(stderr.String(), "--focus names an item that no lane can take") || !strings.Contains(stderr.String(), "ghost-id: not in the inbox") {
		t.Fatalf("rc=%d stderr=%q, want rc 0 and one WARN for ghost-id", rc, stderr.String())
	}
	stderr.Reset()
	writeInboxFile(t, evolveDir, "c.json", `{"id":"console-only","weight":0.5,"route":"console"}`)
	if err := os.MkdirAll(filepath.Join(evolveDir, "inbox", "consumed"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "inbox", "consumed", "gone.json"), []byte(`{"id":"gone"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	rc = refuseSelector(loopConfig{EvolveDir: evolveDir, Focus: []string{"console-only", "gone"}}, &stderr)
	for _, want := range []string{"console-only: route:console", "gone: "} {
		if rc != 0 || !strings.Contains(stderr.String(), want) {
			t.Fatalf("rc=%d stderr=%q, want rc 0 and a WARN line naming %q", rc, stderr.String(), want)
		}
	}
	stderr.Reset()
	broken := filepath.Join(t.TempDir(), ".evolve")
	if err := os.MkdirAll(broken, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(broken, "inbox"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	rc = refuseSelector(loopConfig{EvolveDir: broken, Focus: []string{"ghost-id"}}, &stderr)
	if rc != 0 || !strings.Contains(stderr.String(), "cannot check the items --focus names") {
		t.Fatalf("rc=%d stderr=%q, want rc 0 and the unreadable-inbox warning", rc, stderr.String())
	}
	stderr.Reset()
	if rc = refuseSelector(loopConfig{EvolveDir: evolveDir, Focus: []string{"ghost-id"}, Resume: true}, &stderr); rc != 0 || stderr.Len() != 0 {
		t.Fatalf("resume: rc=%d stderr=%q, want silence", rc, stderr.String())
	}
}
