package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const routingInitVerb = "init"

func contractUnitWrite(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPrintVerbJSON_AnUnencodableResultIsAnIOFailure(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	rc := printVerbJSON("gc", map[string]any{"bad": make(chan int)}, &stdout, &stderr)
	if rc != cliExitIO || stdout.Len() != 0 || !strings.HasPrefix(stderr.String(), "evolve gc: encode the JSON result: ") {
		t.Fatalf("rc %d stdout %q stderr %q, want exit 2 and the encode error", rc, stdout.String(), stderr.String())
	}
}

func TestRunBoundaryJSON_RecordsEachStepAndKeepsTheFailedStepsCode(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name          string
		failAt, code  int
		wantRC, steps int
	}{{"each step passes", 0, 0, cliExitOK, 10}, {"the sync step fails", 3, 22, 22, 3}} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			verbs := &fakeBoundaryVerbs{failAt: c.failAt, code: c.code}
			var stdout, stderr bytes.Buffer
			rc := runBoundaryWith(verbs.dispatch, []string{"run", "--json", "--goal-text-file", fakeBoundaryGoalFile(t), "--project-root", t.TempDir()}, &stdout, &stderr)
			var result boundaryRunResult
			if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
				t.Fatalf("stdout %q: %v", stdout.String(), err)
			}
			if rc != c.wantRC || result.DryRun || len(result.Steps) != c.steps || len(result.Plan) != 10 || result.Steps[len(result.Steps)-1].OK != (c.failAt == 0) {
				t.Fatalf("rc %d result %+v, want rc %d and %d steps", rc, result, c.wantRC, c.steps)
			}
			if strings.Contains(stdout.String(), "fake-step") {
				t.Errorf("the step output must go to stderr, not stdout: %q", stdout.String())
			}
		})
	}
}

func TestRunCLIRoutingExplain_JSONNamesARefusedRouteOnStderr(t *testing.T) {
	root := routingWriteProject(t, `{"cli_routing": {"clis": ["agy", "agy-claude", "claude"]}}`)
	var stdout, stderr bytes.Buffer
	rc := runCLIRouting([]string{"explain", "no-such-agent", "--static", "--json", "--project-root", root}, nil, &stdout, &stderr)
	var got routingExplanation
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil || rc != exitRoutingFinding || got.Route.Error == "" {
		t.Fatalf("rc %d route %+v (%v), want exit 1 and the route error in data", rc, got.Route, err)
	}
	if !strings.Contains(stderr.String(), "evolve cli-routing explain: refused: "+got.Route.Error) {
		t.Errorf("stderr %q must name the refusal for the envelope message", stderr.String())
	}
	if rc := runCLIRouting([]string{"explain", "no-such-agent", "--static", "--json", "--project-root", root}, nil, &failingWriter{}, &stderr); rc != cliExitIO {
		t.Errorf("an unwritable stdout: rc %d, want 2", rc)
	}
}

func TestRunCLIRoutingWrite_JSONKeepsTheExitOfAFailedWrite(t *testing.T) {
	root := routingWriteProject(t, `{"cli_routing": {"clis": ["agy", "claude"], "default": ["agy", "claude"]}}`)
	var stdout, stderr bytes.Buffer
	rc := runCLIRouting([]string{routingInitVerb, "--clis", "claude", "--json", "--project-root", root}, nil, &stdout, &stderr)
	if rc != exitRoutingFinding || stdout.Len() != 0 || !strings.Contains(stderr.String(), "a cli_routing block exists") {
		t.Fatalf("rc %d stdout %q stderr %q, want exit 1, no data and the refusal", rc, stdout.String(), stderr.String())
	}
}

func TestRunDossierVerify_JSONReportsEachVerdict(t *testing.T) {
	t.Parallel()
	enrolled := `{"floor":[{"id":"dossier-closeout"}]}`
	for _, c := range []struct {
		name, policy, cycles string
		dossiers             map[string]string
		wantRC               int
		wantErr              string
	}{
		{name: "a bad policy", policy: "{bad", wantRC: cliExitFailed, wantErr: "dossier verify: load policy: "},
		{name: "cycles is a file", cycles: "a file", wantRC: cliExitFailed, wantErr: "dossier verify: read dir: "},
		{name: "enrolled with no dossier", policy: enrolled, dossiers: map[string]string{"notes.txt": "x"}, wantRC: cliExitFailed, wantErr: "contains no dossiers"},
		{name: "an invalid dossier", dossiers: map[string]string{"cycle-1.json": `{}`}, wantRC: cliExitFailed, wantErr: "dossier verify: invalid cycle-1.json: "},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if c.policy != "" {
				contractUnitWrite(t, root, ".evolve/policy.json", c.policy)
			}
			if c.cycles != "" {
				contractUnitWrite(t, root, "knowledge-base/cycles", c.cycles)
			}
			for name, body := range c.dossiers {
				contractUnitWrite(t, root, "knowledge-base/cycles/"+name, body)
			}
			var stdout, stderr bytes.Buffer
			rc := runDossierVerify([]string{"--json", "--project-root", root}, &stdout, &stderr)
			if rc != c.wantRC || stdout.Len() != 0 || !strings.Contains(stderr.String(), c.wantErr) {
				t.Fatalf("rc %d stdout %q stderr %q, want %d and %q", rc, stdout.String(), stderr.String(), c.wantRC, c.wantErr)
			}
		})
	}
}

func TestVerifyDossierFile_AnUnreadableFileIsNamed(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	link := filepath.Join(dir, "cycle-2.json")
	if err := os.Symlink(filepath.Join(dir, "absent"), link); err != nil {
		t.Fatal(err)
	}
	if err := verifyDossierFile(link); err == nil || !strings.HasPrefix(err.Error(), "read cycle-2.json: ") {
		t.Fatalf("err = %v, want the read error naming the file", err)
	}
}

func TestRunDossierVerify_AnUnreadableCwdIsAnIOFailure(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", "")
	e2StubProcessGetwd(t, errors.New("cwd gone"))
	var stdout, stderr bytes.Buffer
	if rc := runDossierVerify(nil, &stdout, &stderr); rc != cliExitIO || stderr.String() != "dossier verify: cwd: cwd gone\n" {
		t.Fatalf("rc %d stderr %q, want 2 and the cwd error", rc, stderr.String())
	}
}

func TestRunLedgerVerify_AnUnreadableCwdIsAnIOFailure(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", "")
	e2StubProcessGetwd(t, errors.New("cwd gone"))
	var stdout, stderr bytes.Buffer
	if rc := runLedgerVerify(nil, &stdout, &stderr); rc != cliExitIO || stderr.String() != "[ledger] cwd: cwd gone\n" {
		t.Fatalf("rc %d stderr %q, want 2 and the cwd error", rc, stderr.String())
	}
}

func TestRunInboxLint_TextNamesTheCitedProtectedBasenamesAndFailsOnEachLoadOrWriteError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	contractUnitWrite(t, root, "go/internal/core/disposition_gate.go", "package core\n")
	contractUnitWrite(t, root, ".evolve/inbox/cites.json", `{"id":"cites","kind":"bug","weight":0.9,"priority_class":"correctness","title":"disposition_gate.go misreads a row","summary":"s"}`)
	contractUnitWrite(t, root, ".evolve/inbox/plain.json", `{"id":"plain","kind":"bug","weight":0.8,"priority_class":"correctness","title":"the queue renders slowly","summary":"s"}`)
	var stdout, stderr bytes.Buffer
	rc := runInboxLint(root, nil, &stdout, &stderr)
	want := []string{"cites: undeclared surface (cites protected basename: disposition_gate.go)", "plain: undeclared surface"}
	if rc != cliExitFailed || !strings.Contains(stdout.String(), want[0]) || !strings.Contains(stdout.String(), want[1]) {
		t.Fatalf("rc %d stdout %q, want exit 1 and %q", rc, stdout.String(), want)
	}
	if rc := runInboxLint(root, []string{"--json"}, &failingWriter{}, &stderr); rc != cliExitFailed || !strings.Contains(stderr.String(), "inbox lint: encode: ") {
		t.Errorf("an unwritable stdout: rc %d stderr %q", rc, stderr.String())
	}
	broken := t.TempDir()
	contractUnitWrite(t, broken, ".evolve/inbox", "a file\n")
	if rc := runInboxLint(broken, nil, &stdout, &stderr); rc != cliExitFailed || !strings.Contains(stderr.String(), "inbox lint: ") {
		t.Errorf("an unreadable inbox: rc %d stderr %q", rc, stderr.String())
	}
}

func TestRunSignals_ABareProjectRootFlagIsUsage(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if rc := runSignals([]string{"tail", "--project-root"}, nil, &stdout, &stderr); rc != cliExitUsage || !strings.Contains(stderr.String(), "usage: "+signalsTailUsage) {
		t.Fatalf("rc %d stderr %q, want 10 and the usage", rc, stderr.String())
	}
}
