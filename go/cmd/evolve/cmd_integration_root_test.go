package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var errCwdGone = errors.New("cwd gone")

func TestChooseRoot_FlagBeatsEnvBeatsFallback(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, flag, env, fallback, want string }{
		{"flag", "/f", "/e", "/c", "/f"},
		{"env", "", "/e", "/c", "/e"},
		{"fallback", "", "", "/c", "/c"},
	} {
		if got := chooseRoot(tc.flag, tc.env, tc.fallback); got != tc.want {
			t.Errorf("%s: chooseRoot = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestLoopStopRoot_ReadsTheWorkingDirectoryOnlyWhenNeitherTheFlagNorTheVariableIsSet(t *testing.T) {
	prev := processGetwd
	t.Cleanup(func() { processGetwd = prev })
	processGetwd = func() (string, error) { return "", errCwdGone }
	t.Setenv("EVOLVE_PROJECT_ROOT", "")
	var stderr bytes.Buffer
	if got, err := loopStopRoot("/flag", &stderr); err != nil || got != "/flag" {
		t.Errorf("flag with a broken cwd = %q, %v", got, err)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", "/env")
	if got, err := loopStopRoot("", &stderr); err != nil || got != "/env" {
		t.Errorf("env with a broken cwd = %q, %v", got, err)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", "")
	if _, err := loopStopRoot("", &stderr); err == nil {
		t.Error("no flag, no env and a broken cwd must fail")
	}
}

func TestProductionIntegrationEnv_TakesTheRootFromTheEnvironmentVariable(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", "/from/env")
	env := productionIntegrationEnv("/repo", time.Unix(0, 0))
	if env.ProjectRootEnv != "/from/env" || env.Root != "/repo" {
		t.Fatalf("env = %+v, want the variable and the working directory kept apart", env)
	}
}

func (r statusRig) runIn(env integrationEnv, args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := statusVerb(env, args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestStatusVerb_ProjectRootFlagReadsThatRootFromAnotherDirectoryAndPrintsIt(t *testing.T) {
	t.Parallel()
	r := newStatusRig(t)
	r.writePool(t, 5, nil)
	elsewhere := t.TempDir()

	code, stdout, stderr := r.runIn(integrationEnv{Root: elsewhere}, "--project-root", r.root, "--json")

	var rep integrationStatusReport
	if err := json.Unmarshal([]byte(stdout), &rep); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, stdout)
	}
	if code != 0 || stderr != "" || rep.Pool != 5 || rep.ProjectRoot != r.root {
		t.Fatalf("code %d stderr %q report %+v, want pool 5 read from %s", code, stderr, rep, r.root)
	}
}

func TestStatusVerb_TheEnvironmentStepSitsBetweenTheFlagAndTheWorkingDirectory(t *testing.T) {
	t.Parallel()
	withRecord := newStatusRig(t)
	withRecord.writePool(t, 5, nil)
	other := newStatusRig(t)
	other.writePool(t, 9, nil)
	empty := t.TempDir()

	_, viaEnv, _ := withRecord.runIn(integrationEnv{Root: empty, ProjectRootEnv: withRecord.root}, "--json")
	_, viaFlag, _ := withRecord.runIn(integrationEnv{Root: empty, ProjectRootEnv: other.root}, "--project-root", withRecord.root, "--json")
	_, viaCwd, _ := withRecord.runIn(integrationEnv{Root: other.root}, "--json")

	for name, tc := range map[string]struct {
		out  string
		pool int
	}{"env beats cwd": {viaEnv, 5}, "flag beats env": {viaFlag, 5}, "cwd is last": {viaCwd, 9}} {
		var rep integrationStatusReport
		if err := json.Unmarshal([]byte(tc.out), &rep); err != nil || rep.Pool != tc.pool {
			t.Errorf("%s: pool %d (%v), want %d\n%s", name, rep.Pool, err, tc.pool, tc.out)
		}
	}
}

func TestStatusVerb_AnAbsentRootExitsTwoAndNamesTheOperationTheRootTheCauseAndTheNextStep(t *testing.T) {
	t.Parallel()
	r := newStatusRig(t)
	absent := filepath.Join(t.TempDir(), "nowhere")
	for name, args := range map[string][]string{"flag": {"--project-root", absent}, "flag and json": {"--project-root", absent, "--json"}} {
		code, stdout, stderr := r.runIn(integrationEnv{Root: r.root}, args...)
		if code != exitIO || stdout != "" {
			t.Errorf("%s: code %d stdout %q, want exit %d and no data", name, code, stdout, exitIO)
		}
		for _, want := range []string{"integration status", absent, "no such file", "pass --project-root"} {
			if !strings.Contains(stderr, want) {
				t.Errorf("%s: stderr %q lacks %q", name, stderr, want)
			}
		}
	}
	if code, _, stderr := r.runIn(integrationEnv{Root: absent}); code != exitIO || !strings.Contains(stderr, "pass --project-root") {
		t.Errorf("an absent working directory root = %d %q", code, stderr)
	}
}

func TestStatusVerb_ARootWithNoEvolveDirectoryStaysARefusalThatNamesTheRoot(t *testing.T) {
	t.Parallel()
	r := newStatusRig(t)
	code, _, stderr := r.runIn(integrationEnv{Root: t.TempDir()}, "--project-root", r.root)
	if code != exitRefused || !strings.Contains(stderr, "no wave record exists under "+r.root) {
		t.Fatalf("= %d, %q", code, stderr)
	}
}

func TestIntegration_TheFlagIsReadAfterTheSubVerbAndOnlyByStatus(t *testing.T) {
	t.Parallel()
	r := newStatusRig(t)
	r.writePool(t, 5, nil)
	var out, errb bytes.Buffer
	env := integrationEnv{Root: t.TempDir(), Wave: "w"}
	if rc := runIntegrationEnv(env, []string{"--project-root", r.root, "status"}, &out, &errb); rc != exitUsage {
		t.Errorf("flag before the sub-verb = %d, want usage", rc)
	}
	for _, args := range [][]string{
		{"record-pr", "--project-root", r.root, "--pr", "5", "--reviewed"},
		{"batch", "--project-root", r.root},
		{"watch", "--project-root", r.root, "--tip", strings.Repeat("a", 40)},
	} {
		out.Reset()
		errb.Reset()
		if rc := runIntegrationEnv(env, args, &out, &errb); rc != exitUsage {
			t.Errorf("%v = %d, want usage: only status reads the flag", args, rc)
		}
	}
}

func TestBatch_GhAndGitRunInTheWorkingDirectoryWithTheVariableSet(t *testing.T) {
	t.Parallel()
	cwd, other := t.TempDir(), filepath.Join(t.TempDir(), "other")
	var dirs []string
	env := integrationEnv{Root: cwd, Wave: "w", ProjectRootEnv: other,
		Gh: func(_ context.Context, dir string, _ ...string) (string, int, error) {
			dirs = append(dirs, dir)
			return "[]", 0, nil
		},
		Git: func(_ context.Context, dir, _ string, _ ...string) (string, error) {
			dirs = append(dirs, dir)
			return "", nil
		}}
	var out, errb bytes.Buffer
	runIntegrationEnv(env, []string{"batch"}, &out, &errb)
	if len(dirs) == 0 {
		t.Fatal("no gh or git call reached the spy")
	}
	for _, d := range dirs {
		if d != cwd {
			t.Errorf("dir %s, want %s", d, cwd)
		}
	}
}

func TestRecordPR_ReadsTheWorkingDirectoryWithTheVariableSet(t *testing.T) {
	t.Parallel()
	cwd, other := t.TempDir(), filepath.Join(t.TempDir(), "other")
	var out, errb bytes.Buffer
	env := integrationEnv{Root: cwd, Wave: "w", ProjectRootEnv: other}
	runIntegrationEnv(env, []string{"record-pr", "--pr", "5", "--reviewed"}, &out, &errb)
	if !strings.Contains(errb.String(), cwd) || strings.Contains(errb.String(), other) {
		t.Errorf("err %q, want %s and not %s", errb.String(), cwd, other)
	}
}

func TestWatch_NeverReadsTheVariable(t *testing.T) {
	t.Parallel()
	cwd, other := t.TempDir(), filepath.Join(t.TempDir(), "other")
	var dirs []string
	env := integrationEnv{Root: cwd, ProjectRootEnv: other,
		Gh: func(_ context.Context, dir string, _ ...string) (string, int, error) {
			dirs = append(dirs, dir)
			return "[]", 0, nil
		}}
	var out, errb bytes.Buffer
	runIntegrationEnv(env, []string{"watch", "--tip", strings.Repeat("a", 40)}, &out, &errb)
	if len(dirs) == 0 {
		t.Fatal("no gh call reached the spy")
	}
	for _, d := range dirs {
		if d == other {
			t.Errorf("dir %s is the variable, want it unused", d)
		}
	}
}

func TestRunIntegrationAt_TheEnvironmentVariableReachesStatus(t *testing.T) {
	r := newStatusRig(t)
	r.writePool(t, 5, nil)
	t.Setenv("EVOLVE_PROJECT_ROOT", r.root)
	var out, errb bytes.Buffer
	rc := runIntegrationAt(func() (string, error) { return t.TempDir(), nil }, []string{"status", "--json"}, &out, &errb)
	var rep integrationStatusReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil || rep.Pool != 5 || rep.ProjectRoot != r.root {
		t.Fatalf("rc %d rep %+v err %v stderr %s", rc, rep, err, errb.String())
	}
}

func TestStatusVerb_AWaveQueryPrintsTheRootAtTheTopOfTheJSON(t *testing.T) {
	t.Parallel()
	r := newStatusRig(t)
	r.seedWaves(t, 0, 20, 40)
	r.writePool(t, 5, nil)
	code, stdout, stderr := r.run("--wave", "1", "--json")
	var set integrationStatusWaves
	if err := json.Unmarshal([]byte(stdout), &set); err != nil || set.ProjectRoot != r.root {
		t.Fatalf("code %d stderr %q root %q err %v\n%s", code, stderr, set.ProjectRoot, err, stdout)
	}
}

func TestStatusVerb_ARelativeRootFromTheFlagOrTheVariableIsPrintedAbsolute(t *testing.T) {
	r := newStatusRig(t)
	r.writePool(t, 5, nil)
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(filepath.Dir(r.root)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
	rel := filepath.Base(r.root)
	for name, tc := range map[string]struct {
		env  integrationEnv
		args []string
	}{
		"flag":     {integrationEnv{Root: t.TempDir()}, []string{"--project-root", rel, "--json"}},
		"variable": {integrationEnv{Root: t.TempDir(), ProjectRootEnv: rel}, []string{"--json"}},
	} {
		code, stdout, stderr := r.runIn(tc.env, tc.args...)
		var rep integrationStatusReport
		if err := json.Unmarshal([]byte(stdout), &rep); err != nil || !filepath.IsAbs(rep.ProjectRoot) || filepath.Base(rep.ProjectRoot) != rel {
			t.Errorf("%s: code %d stderr %q root %q err %v", name, code, stderr, rep.ProjectRoot, err)
		}
	}
}

func TestIntegrationUsage_BothLinesStateTheFlagAndTheTruthfulExitCodes(t *testing.T) {
	t.Parallel()
	for name, text := range map[string]string{"family": integrationUsage, "status": integrationStatusUsage} {
		for _, want := range []string{"--project-root P", "2 = an absent root, an unreadable wave store, or a wave number that the store does not know"} {
			if !strings.Contains(text, want) {
				t.Errorf("%s usage lacks %q:\n%s", name, want, text)
			}
		}
	}
}
