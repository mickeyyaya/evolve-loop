package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestWaveFocus_NextDryRunPrintsTheFocusIdsInThePlan(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	p.seedWave81()
	h := newWaveHarness()

	rc := h.run("next", "--dry-run", "--json", "--focus", "id-x,id-y", "--project-root", p.root)

	if rc != 0 {
		t.Fatalf("rc = %d, want 0; stderr:\n%s", rc, h.stderr.String())
	}
	e := decodeWaveEnvelope(t, h.stdout.Bytes())
	launch := e.Steps[len(e.Steps)-1].Name
	if !strings.Contains(launch, "--focus id-x,id-y") {
		t.Errorf("launch step = %q, want it to carry --focus id-x,id-y", launch)
	}
	if !strings.Contains(e.Goal, "Focus items for wave 82: id-x, id-y.") {
		t.Errorf("goal does not name the focus ids:\n%s", e.Goal)
	}

	plain := newWaveHarness()
	if prc := plain.run("next", "--dry-run", "--focus", "id-x,id-y", "--project-root", p.root); prc != 0 {
		t.Fatalf("plain dry-run rc = %d, want 0; stderr:\n%s", prc, plain.stderr.String())
	}
	if out := plain.stdout.String(); !strings.Contains(out, "Focus items for wave 82: id-x, id-y.") || !strings.Contains(out, "--focus id-x,id-y") {
		t.Errorf("plain dry-run stdout does not print the focus ids:\n%s", out)
	}
}

func TestWaveFocus_NextRefusesFocusWithOnlyAsAUsageError(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	p.seedWave81()
	h := newWaveHarness()

	rc := h.run("next", "--dry-run", "--focus", "id-x", "--only", "id-y", "--project-root", p.root)

	if rc != exitUsage {
		t.Fatalf("rc = %d, want %d; stderr:\n%s", rc, exitUsage, h.stderr.String())
	}
	if !strings.Contains(h.stderr.String(), "--focus and --only are mutually exclusive") {
		t.Errorf("stderr = %q, want the conflict named", h.stderr.String())
	}
}

func TestWaveFocus_LaunchRecordsTheFocusIdsAndStatusNamesThem(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	p.seedWave81()
	h := newWaveHarness()

	if rc := h.run("next", "--json", "--focus", "id-x,id-y", "--project-root", p.root); rc != 0 {
		t.Fatalf("next rc = %d; stderr:\n%s", rc, h.stderr.String())
	}
	records, _ := p.store().Records()
	if got := records[len(records)-1].Focus; !slices.Equal(got, []string{"id-x", "id-y"}) {
		t.Fatalf("recorded focus = %v, want [id-x id-y]", got)
	}

	text, js := newWaveHarness(), newWaveHarness()
	text.run("status", "--project-root", p.root)
	js.run("status", "--json", "--project-root", p.root)
	if out := text.stdout.String(); !strings.Contains(out, "focus:   id-x, id-y\n") {
		t.Errorf("status text:\n%s\nwant a focus line", out)
	}
	var got waveStatusReport
	if err := json.Unmarshal(js.stdout.Bytes(), &got); err != nil || !slices.Equal(got.Focus, []string{"id-x", "id-y"}) {
		t.Errorf("status json focus = %v, %v, want [id-x id-y]", got.Focus, err)
	}
}

func TestWaveFocus_StatusOfAWaveWithoutFocusPrintsNoFocusLine(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	p.seedWave81()
	h := newWaveHarness()

	h.run("status", "--project-root", p.root)

	if strings.Contains(h.stdout.String(), "focus:") {
		t.Errorf("status text names focus for a wave with none:\n%s", h.stdout.String())
	}
}

func TestWaveFocus_BoundaryLaunchCarriesFocusAndRefusesItWithOnly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	goal := filepath.Join(dir, "goal.txt")
	if err := os.WriteFile(goal, []byte("work\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	a, err := parseBoundaryArgs([]string{"--goal-text-file", goal, "--focus", "id-x"})
	if err != nil || a.focus != "id-x" {
		t.Fatalf("parse = %+v, %v, want focus id-x", a, err)
	}
	launch := boundaryLaunchSteps(a, dir)
	if got := launch[len(launch)-1].args; !slices.Contains(got, "--focus") || !slices.Contains(got, "id-x") {
		t.Errorf("launch args = %v, want --focus id-x", got)
	}
	if _, err := parseBoundaryArgs([]string{"--goal-text-file", goal, "--focus", "id-x", "--only", "id-y"}); !errors.Is(err, errFocusWithOnly) {
		t.Errorf("parse error = %v, want errFocusWithOnly", err)
	}
}

func TestWaveFocus_LoopFlagFillsTheConfigAndRefusesOnlyWithExit10(t *testing.T) {
	t.Parallel()
	root := selectorFixture(t)
	var stderr bytes.Buffer

	cfg, rc := parseLoopArgs(selectorLaunch(root, "--focus", "delta-ready,charlie-ready"), &stderr)
	if rc != 0 || !reflect.DeepEqual(cfg.Focus, []string{"delta-ready", "charlie-ready"}) {
		t.Fatalf("parse rc=%d focus=%v, want 0 and [delta-ready charlie-ready]\n%s", rc, cfg.Focus, stderr.String())
	}
	stderr.Reset()
	if _, rc := parseLoopArgs(selectorLaunch(root, "--focus", "delta-ready", "--only", "alpha-ready"), &stderr); rc != exitUsage {
		t.Fatalf("parse with --only rc=%d, want %d\n%s", rc, exitUsage, stderr.String())
	}
	if !strings.Contains(stderr.String(), "--focus and --only are mutually exclusive") {
		t.Errorf("stderr = %q, want the conflict named", stderr.String())
	}
}

func TestWaveFocus_LoopWaveLeadsWithTheFocusIdsAndKeepsTheOthers(t *testing.T) {
	t.Parallel()
	root := selectorFixture(t)

	got := selectorPlan(t, selectorLaunch(root, "--focus", "delta-ready,charlie-ready"))

	if want := []string{"charlie-ready", "delta-ready"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("wave plan = %v, want %v", got, want)
	}
	if wide := selectorPlanCount(t, selectorLaunch(root, "--focus", "delta-ready"), 3); !reflect.DeepEqual(wide, []string{"alpha-ready", "bravo-ready", "delta-ready"}) {
		t.Fatalf("3-lane plan = %v, want delta-ready plus the two top items", wide)
	}
	if excluded := selectorPlanCount(t, selectorLaunch(root, "--focus", "delta-ready", "--exclude", "delta-ready"), 2); !reflect.DeepEqual(excluded, []string{"alpha-ready", "bravo-ready"}) {
		t.Fatalf("plan with the focus id excluded = %v, want the two top items", excluded)
	}
}

func TestWaveFocus_ASequentialBatchStillWarnsThatFocusIsIgnored(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer

	warnSelectorIgnored(loopConfig{Focus: []string{"id-x"}}, &stderr)

	if !strings.Contains(stderr.String(), "--focus") {
		t.Errorf("stderr = %q, want a note that --focus does not apply", stderr.String())
	}
}

func selectorPlanCount(t *testing.T, args []string, count int) []string {
	t.Helper()
	var stderr bytes.Buffer
	cfg, rc := parseLoopArgs(args, &stderr)
	if rc != 0 {
		t.Fatalf("parseLoopArgs(%q) rc=%d, want 0\n%s", args, rc, clipSelectorStderr(stderr.String()))
	}
	data, _, err := productionWavePlanFn(cfg, &erroringReadStorage{}, count, io.Discard)(context.Background(), 0)
	if err != nil {
		t.Fatalf("plan for %q: %v", args, err)
	}
	return plannedTopNIDs(t, data)
}
