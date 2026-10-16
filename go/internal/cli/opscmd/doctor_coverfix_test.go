package opscmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/doctor"
	"github.com/mickeyyaya/evolve-loop/go/internal/releaseconsistency"
)

type coverfixFailingWriter struct{}

func (coverfixFailingWriter) Write([]byte) (int, error) { return 0, errors.New("write boom") }

func TestCoverfixRunDoctorDispatchesPlane(t *testing.T) {
	root := planePrimaryRoot(t, false)
	var out, errb bytes.Buffer
	if rc := RunDoctor([]string{"plane", root}, nil, &out, &errb); rc != exitOK {
		t.Fatalf("rc = %d: %s%s", rc, out.String(), errb.String())
	}
	if !strings.Contains(out.String(), "PRIMARY checkout") {
		t.Errorf("plane subcommand did not run: %q", out.String())
	}
}

func TestCoverfixDoctorProbeRefusesBadInvocations(t *testing.T) {
	cases := map[string][]string{
		"noTool":  {"probe"},
		"badFlag": {"probe", "--nope", "go"},
	}
	for name, args := range cases {
		var out, errb bytes.Buffer
		if rc := RunDoctor(args, nil, &out, &errb); rc != exitUsage {
			t.Errorf("%s: rc = %d, want %d", name, rc, exitUsage)
		}
		if out.Len() != 0 {
			t.Errorf("%s: stdout must stay empty, got %q", name, out.String())
		}
	}
}

func TestCoverfixDoctorProbeReportsProbeAndEmitFailures(t *testing.T) {
	savedProbe, savedEmit := doctorProbe, doctorEmitJSON
	t.Cleanup(func() { doctorProbe, doctorEmitJSON = savedProbe, savedEmit })

	doctorProbe = func(string) (doctor.Result, error) { return doctor.Result{}, errors.New("probe boom") }
	var out, errb bytes.Buffer
	if rc := runDoctorProbe([]string{"go"}, &out, &errb); rc != exitUsage || !strings.Contains(errb.String(), "probe boom") {
		t.Errorf("probe failure: rc=%d stderr=%q", rc, errb.String())
	}

	doctorProbe = func(tool string) (doctor.Result, error) { return doctor.Result{Tool: tool, Found: true}, nil }
	doctorEmitJSON = func(doctor.Result) ([]byte, error) { return nil, errors.New("emit boom") }
	out.Reset()
	errb.Reset()
	if rc := runDoctorProbe([]string{"--json", "go"}, &out, &errb); rc != exitUsage || !strings.Contains(errb.String(), "emit boom") {
		t.Errorf("emit failure: rc=%d stderr=%q", rc, errb.String())
	}
	if out.Len() != 0 {
		t.Errorf("no payload may reach stdout when emit fails: %q", out.String())
	}
}

func TestCoverfixDoctorPlaneReportsLinkedWorktreeAndDetachedHead(t *testing.T) {
	root := t.TempDir()
	gitdir := filepath.Join(t.TempDir(), "wt-gitdir")
	if err := os.MkdirAll(gitdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitdir, "HEAD"), []byte("0123456789abcdef0123456789abcdef01234567\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: "+gitdir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "state.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if rc := runDoctorPlane([]string{root}, &out, &errb); rc != exitOK {
		t.Fatalf("a linked worktree owning loop state is the healthy shape, rc = %d: %s", rc, errb.String())
	}
	for _, want := range []string{"linked worktree (runtime plane)", "branch: (detached)"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("report missing %q: %s", want, out.String())
		}
	}
}

func TestCoverfixDoctorPlaneFailsWhenRootCannotBeMadeAbsolute(t *testing.T) {
	saved := absPath
	t.Cleanup(func() { absPath = saved })
	absPath = func(string) (string, error) { return "", errors.New("abs boom") }
	var out, errb bytes.Buffer
	if rc := runDoctorPlane(nil, &out, &errb); rc != exitFail || !strings.Contains(errb.String(), "abs boom") {
		t.Errorf("rc=%d stderr=%q", rc, errb.String())
	}
}

func TestCoverfixDoctorVersionsRefusesPositionalsAndUnwritableJSON(t *testing.T) {
	stubVersions(t, map[string]string{"claude": "2.1.3"}, nil)
	var out, errb bytes.Buffer
	if rc := runDoctorVersions([]string{"extra"}, &out, &errb); rc != exitUsage || !strings.Contains(errb.String(), "unexpected arguments") {
		t.Errorf("positional: rc=%d stderr=%q", rc, errb.String())
	}
	errb.Reset()
	if rc := runDoctorVersions([]string{"--json"}, coverfixFailingWriter{}, &errb); rc != exitFail || !strings.Contains(errb.String(), "write boom") {
		t.Errorf("json write failure: rc=%d stderr=%q", rc, errb.String())
	}
}

func TestCoverfixReleaseConsistencyMapsRunOutcomesToExitCodes(t *testing.T) {
	root := t.TempDir()
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	saved := releaseConsistencyRun
	t.Cleanup(func() { releaseConsistencyRun = saved })

	cases := []struct {
		name    string
		err     error
		wantRC  int
		wantErr string
	}{
		{"consistent", nil, exitOK, ""},
		{"inconsistent", releaseconsistency.ErrInconsistent, exitFail, ""},
		{"operational", errors.New("disk boom"), exitFail, "[release-consistency] FAIL: disk boom"},
	}
	for _, tc := range cases {
		var got releaseconsistency.Options
		releaseConsistencyRun = func(o releaseconsistency.Options) (releaseconsistency.Result, error) {
			got = o
			return releaseconsistency.Result{}, tc.err
		}
		var out, errb bytes.Buffer
		if rc := RunReleaseConsistency([]string{"1.2.3"}, nil, &out, &errb); rc != tc.wantRC {
			t.Errorf("%s: rc = %d, want %d", tc.name, rc, tc.wantRC)
		}
		if got.ProjectRoot != root || got.Target != "1.2.3" {
			t.Errorf("%s: options = %+v, want root %q target 1.2.3", tc.name, got, root)
		}
		if tc.wantErr == "" && strings.Contains(errb.String(), "FAIL:") {
			t.Errorf("%s: unexpected FAIL line %q", tc.name, errb.String())
		}
		if tc.wantErr != "" && !strings.Contains(errb.String(), tc.wantErr) {
			t.Errorf("%s: stderr %q missing %q", tc.name, errb.String(), tc.wantErr)
		}
	}
}

func coverfixWriteBumpFixture(t *testing.T, root, version string) {
	t.Helper()
	files := map[string]string{
		".claude-plugin/plugin.json":      `{"name":"evo","version":"` + version + `"}`,
		".claude-plugin/marketplace.json": `{"plugins":[{"name":"evo","version":"` + version + `"}]}`,
		"skills/loop/SKILL.md":            "# Evolve Loop v" + strings.Join(strings.Split(version, ".")[:2], ".") + "\n",
		"README.md":                       "Current (v" + strings.Join(strings.Split(version, ".")[:2], ".") + ")\n\n| v" + strings.Join(strings.Split(version, ".")[:2], ".") + " | May 23 | shipped |\n",
	}
	for rel, body := range files {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCoverfixVersionBumpReportsModifiedFilesThenNoOp(t *testing.T) {
	root := t.TempDir()
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	coverfixWriteBumpFixture(t, root, "11.6.6")

	var out, errb bytes.Buffer
	if rc := RunVersionBump([]string{"11.7.0"}, nil, &out, &errb); rc != exitOK {
		t.Fatalf("rc = %d: %s", rc, errb.String())
	}
	if !strings.Contains(errb.String(), "OK: bumped .claude-plugin/plugin.json → 11.7.0") {
		t.Errorf("bump not reported: %q", errb.String())
	}
	if !strings.Contains(out.String(), `"target"`) || !strings.Contains(out.String(), "11.7.0") {
		t.Errorf("result JSON missing from stdout: %q", out.String())
	}
	body, err := os.ReadFile(filepath.Join(root, ".claude-plugin", "plugin.json"))
	if err != nil || !strings.Contains(string(body), `"version": "11.7.0"`) {
		t.Errorf("plugin.json not bumped: %s (%v)", body, err)
	}

	out.Reset()
	errb.Reset()
	if rc := RunVersionBump([]string{"11.7.0"}, nil, &out, &errb); rc != exitOK {
		t.Fatalf("second bump rc = %d: %s", rc, errb.String())
	}
	if !strings.Contains(errb.String(), "no-op: all files already at 11.7.0") {
		t.Errorf("idempotent bump must say no-op: %q", errb.String())
	}
}

func TestCoverfixVersionBumpFailsOnMissingMarkers(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", t.TempDir())
	var out, errb bytes.Buffer
	if rc := RunVersionBump([]string{"11.7.0"}, nil, &out, &errb); rc != exitFail || !strings.Contains(errb.String(), "[version-bump] FAIL:") {
		t.Errorf("rc=%d stderr=%q", rc, errb.String())
	}
}
