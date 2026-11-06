package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const rootNextStep = "pass --project-root"

func absentRoot(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "nowhere")
}

func assertRootSignal(t *testing.T, op string, rc int, stderr, root string) {
	t.Helper()
	if rc != 2 {
		t.Errorf("%s: rc = %d, want 2", op, rc)
	}
	for _, want := range []string{op, root, "no such file", rootNextStep} {
		if !strings.Contains(stderr, want) {
			t.Errorf("%s: stderr %q lacks %q", op, stderr, want)
		}
	}
}

func TestProjectRootCheck_NamesTheOperationTheRootTheCauseAndTheNextStep(t *testing.T) {
	t.Parallel()
	absent := absentRoot(t)
	var stderr bytes.Buffer
	rc := checkRootHasEvolve("some op", absent, &stderr)
	assertRootSignal(t, "some op", rc, stderr.String(), absent)

	bare := t.TempDir()
	stderr.Reset()
	if rc := checkRootHasEvolve("some op", bare, &stderr); rc != 2 || !strings.Contains(stderr.String(), filepath.Join(bare, ".evolve")) {
		t.Errorf("no .evolve: rc %d %q", rc, stderr.String())
	}
	stderr.Reset()
	if rc := checkRootDir("some op", bare, &stderr); rc != 0 || stderr.Len() != 0 {
		t.Errorf("a root with no .evolve passes when .evolve is not needed: rc %d %q", rc, stderr.String())
	}
	file := filepath.Join(bare, "afile")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	stderr.Reset()
	if rc := checkRootDir("some op", file, &stderr); rc != 2 || !strings.Contains(stderr.String(), "not a directory") {
		t.Errorf("a file as root: rc %d %q", rc, stderr.String())
	}
}

func TestProjectRootCheck_AnEvolveEntryThatIsAFileIsRefused(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".evolve"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	if rc := checkRootHasEvolve("some op", root, &stderr); rc != 2 || !strings.Contains(stderr.String(), "not a directory") {
		t.Errorf("rc %d %q", rc, stderr.String())
	}
	stderr.Reset()
	if rc := checkRootHasEvolve("some op", t.TempDir(), &stderr); rc != 2 {
		t.Errorf("a root with no .evolve: rc %d", rc)
	}
	good := t.TempDir()
	if err := os.MkdirAll(filepath.Join(good, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	stderr.Reset()
	if rc := checkRootHasEvolve("some op", good, &stderr); rc != 0 || stderr.Len() != 0 {
		t.Errorf("a project with .evolve/: rc %d %q", rc, stderr.String())
	}
}

func TestWaveWatch_AnAbsentRootExitsTwoAndNamesWaveWatch(t *testing.T) {
	t.Parallel()
	h := newWaveHarness()
	absent := absentRoot(t)
	rc := h.run("watch", "--project-root", absent)
	assertRootSignal(t, "wave watch", rc, h.stderr.String(), absent)
}

func TestWaveReads_AnUnexpectedArgumentIsAUsageError(t *testing.T) {
	t.Parallel()
	for _, verb := range []string{"status", "watch"} {
		h := newWaveHarness()
		if rc := h.run(verb, "stray"); rc != cliExitUsage || h.stdout.Len() != 0 {
			t.Errorf("%s: rc %d stdout %q, want usage", verb, rc, h.stdout.String())
		}
	}
}

func TestWaveStatus_AnAbsentRootOrNoEvolveDirectoryExitsTwoWithTheSignal(t *testing.T) {
	t.Parallel()
	h := newWaveHarness()
	absent := absentRoot(t)
	rc := h.run("status", "--project-root", absent)
	assertRootSignal(t, "wave status", rc, h.stderr.String(), absent)
	if h.stdout.Len() != 0 {
		t.Errorf("stdout = %q, want none", h.stdout.String())
	}
	bare := t.TempDir()
	if rc := h.run("status", "--project-root", bare); rc != 2 || !strings.Contains(h.stderr.String(), rootNextStep) {
		t.Errorf("no .evolve: rc %d %q", rc, h.stderr.String())
	}
}

func TestWaveStatus_AnEmptyEvolveDirectoryStaysSuccessAndTheMessageNamesTheRoot(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	h := newWaveHarness()
	rc := h.run("status", "--project-root", p.root)
	if rc != 0 || h.stdout.String() != "wave: no wave is recorded under "+p.root+"\n" {
		t.Fatalf("rc %d stdout %q", rc, h.stdout.String())
	}
	if rc := h.run("status", "--json", "--project-root", p.root); rc != 0 {
		t.Fatalf("json rc %d", rc)
	}
	var rep struct {
		ProjectRoot string `json:"project_root"`
	}
	if err := json.Unmarshal(h.stdout.Bytes(), &rep); err != nil || rep.ProjectRoot != p.root {
		t.Errorf("project_root = %q (%v), want %q\n%s", rep.ProjectRoot, err, p.root, h.stdout.String())
	}
}

func TestLoopStatusAndStatus_PrintTheRootInJSONAndGiveTheNextStepOnAnAbsentRoot(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", "")
	p := newWavePlane(t)
	var out, errb bytes.Buffer
	if rc := runLoopStatus([]string{"--json", "--project-root", p.root}, &out, &errb); rc != 0 {
		t.Fatalf("loop status rc %d %s", rc, errb.String())
	}
	var loop struct {
		ProjectRoot string          `json:"project_root"`
		Loop        json.RawMessage `json:"loop"`
	}
	if err := json.Unmarshal(out.Bytes(), &loop); err != nil || loop.ProjectRoot != p.root || len(loop.Loop) == 0 {
		t.Errorf("loop status: %+v (%v)\n%s", loop, err, out.String())
	}
	out.Reset()
	if rc := runStatus([]string{"--json", "--project-root", p.root}, nil, &out, &errb); rc != 0 {
		t.Fatalf("status rc %d %s", rc, errb.String())
	}
	var st struct {
		ProjectRoot string `json:"project_root"`
	}
	if err := json.Unmarshal(out.Bytes(), &st); err != nil || st.ProjectRoot != p.root {
		t.Errorf("status project_root = %q (%v)", st.ProjectRoot, err)
	}
	absent := absentRoot(t)
	errb.Reset()
	rc := runLoopStatus([]string{"--project-root", absent}, &out, &errb)
	assertRootSignal(t, "loop status", rc, errb.String(), absent)
	errb.Reset()
	rc = runStatus([]string{"--project-root", absent}, nil, &out, &errb)
	assertRootSignal(t, "status", rc, errb.String(), absent)
}

func TestInboxList_AnAbsentRootOrNoEvolveDirectoryExitsTwoWithTheSignalAndAnEmptyOneStaysSuccess(t *testing.T) {
	t.Parallel()
	absent := absentRoot(t)
	var out, errb bytes.Buffer
	rc := runInboxList(absent, []string{"--json"}, &out, &errb)
	assertRootSignal(t, "inbox list", rc, errb.String(), absent)
	if out.Len() != 0 {
		t.Errorf("stdout = %q, want none", out.String())
	}
	errb.Reset()
	if rc := runInboxList(t.TempDir(), nil, &out, &errb); rc != 2 || !strings.Contains(errb.String(), rootNextStep) {
		t.Errorf("no .evolve: rc %d %q", rc, errb.String())
	}
	empty := t.TempDir()
	if err := os.MkdirAll(filepath.Join(empty, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errb.Reset()
	if rc := runInboxList(empty, []string{"--json"}, &out, &errb); rc != 0 || strings.TrimSpace(out.String()) != "[]" {
		t.Errorf("empty .evolve: rc %d out %q err %q", rc, out.String(), errb.String())
	}
}

func TestLedgerVerify_AnAbsentRootOrNoEvolveDirectoryFailsAndAnAbsentLedgerFileIsSaid(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", "")
	absent := absentRoot(t)
	var out, errb bytes.Buffer
	rc := runLedgerVerify([]string{"--project-root", absent}, &out, &errb)
	assertRootSignal(t, "ledger verify", rc, errb.String(), absent)
	errb.Reset()
	if rc := runLedgerVerify([]string{"--project-root", t.TempDir()}, &out, &errb); rc != 2 || !strings.Contains(errb.String(), rootNextStep) {
		t.Errorf("no .evolve: rc %d %q", rc, errb.String())
	}
	empty := t.TempDir()
	if err := os.MkdirAll(filepath.Join(empty, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	errb.Reset()
	if rc := runLedgerVerify([]string{"--project-root", empty}, &out, &errb); rc != 0 || !strings.Contains(errb.String(), "no ledger file exists") || strings.Contains(errb.String(), "verified strictly from genesis") {
		t.Errorf("no ledger: rc %d %q", rc, errb.String())
	}
	out.Reset()
	if rc := runLedgerVerify([]string{"--json", "--project-root", empty}, &out, &errb); rc != 0 || !strings.Contains(out.String(), "no ledger file exists") {
		t.Errorf("json: rc %d %q", rc, out.String())
	}
}

func TestLedgerVerify_AnExplicitEvolveDirStillWinsOverTheRootCheck(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", "")
	dir := filepath.Join(t.TempDir(), ".evolve")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if rc := runLedgerVerify([]string{"--evolve-dir", dir, "--project-root", absentRoot(t)}, &out, &errb); rc != 0 {
		t.Errorf("rc %d %q", rc, errb.String())
	}
}
