package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
)

// `evolve dashboard` is a READ-ONLY render of the loop's on-disk state
// (ADR-0095). --snapshot is the scriptable form; the served form goes through
// the dashboardServe seam so the wiring is provable without binding a port.

func writeDashboardDossier(t *testing.T, root string, cycle int) {
	t.Helper()
	dir := filepath.Join(root, "knowledge-base", "cycles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	d := dossier.Dossier{Cycle: cycle, Goal: "g", FinalVerdict: dossier.VerdictPass, CommitSHA: "abc",
		Phases: []dossier.PhaseRecord{{Name: "ship", Verdict: "PASS"}}}
	buf, err := dossier.RenderJSON(&d)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cycle-"+strconv.Itoa(cycle)+".json"), buf, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDashboard_SnapshotPrintsJSONWithCycles(t *testing.T) {
	root := t.TempDir()
	writeDashboardDossier(t, root, 41)
	writeDashboardDossier(t, root, 42)
	var out, errb bytes.Buffer
	code := runDashboard([]string{"--project-root", root, "--snapshot"}, nil, &out, &errb)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errb.String())
	}
	var snap struct {
		Root   string `json:"root"`
		Cycles []struct {
			ID    int    `json:"id"`
			State string `json:"state"`
		} `json:"cycles"`
		Trend struct {
			Closed  int `json:"closed"`
			Shipped int `json:"shipped"`
		} `json:"trend"`
	}
	if err := json.Unmarshal(out.Bytes(), &snap); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, out.String())
	}
	if len(snap.Cycles) != 2 || snap.Cycles[0].ID != 42 || snap.Cycles[0].State != "pass" || snap.Trend.Shipped != 2 {
		t.Fatalf("snapshot = %+v", snap)
	}
	if !filepath.IsAbs(snap.Root) {
		t.Fatalf("root must be absolute: %q", snap.Root)
	}
}

func TestDashboard_ServeGoesThroughSeamWithResolvedRootAndAddr(t *testing.T) {
	saved := dashboardServe
	t.Cleanup(func() { dashboardServe = saved })
	var gotRoot, gotAddr string
	dashboardServe = func(_ context.Context, root, addr string) error {
		gotRoot, gotAddr = root, addr
		return nil
	}
	root := t.TempDir()
	var out, errb bytes.Buffer
	if code := runDashboard([]string{"--project-root", root, "--addr", "127.0.0.1:0"}, nil, &out, &errb); code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errb.String())
	}
	if gotRoot != root || gotAddr != "127.0.0.1:0" {
		t.Fatalf("seam got root=%q addr=%q", gotRoot, gotAddr)
	}
	if !bytes.Contains(errb.Bytes(), []byte("serving "+root+" on http://127.0.0.1:0")) {
		t.Fatalf("banner missing: %s", errb.String())
	}
}

func TestDashboard_ServeErrorIsExit1(t *testing.T) {
	saved := dashboardServe
	t.Cleanup(func() { dashboardServe = saved })
	dashboardServe = func(context.Context, string, string) error {
		return errors.New("listen 127.0.0.1:8090: address already in use")
	}
	var out, errb bytes.Buffer
	if code := runDashboard([]string{"--project-root", t.TempDir()}, nil, &out, &errb); code != 1 || !bytes.Contains(errb.Bytes(), []byte("address already in use")) {
		t.Fatalf("exit=%d stderr=%s", code, errb.String())
	}
}

func TestDashboard_BadFlagIsUsageError(t *testing.T) {
	var out, errb bytes.Buffer
	if code := runDashboard([]string{"--nope"}, nil, &out, &errb); code != 10 {
		t.Fatalf("exit=%d, want 10", code)
	}
}

func TestDashboard_DefaultRootFallsBackToEnvThenCwd(t *testing.T) {
	root := t.TempDir()
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	var out, errb bytes.Buffer
	if code := runDashboard([]string{"--snapshot"}, nil, &out, &errb); code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errb.String())
	}
	if !bytes.Contains(out.Bytes(), []byte(`"root": "`+root+`"`)) {
		t.Fatalf("env root not honoured: %s", out.String())
	}
}
