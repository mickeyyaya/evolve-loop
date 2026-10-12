package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"maps"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunLoop_AChainedResumeWarnsAndRunsTheSingleBatch(t *testing.T) {
	root := t.TempDir()
	var stdout, stderr bytes.Buffer

	code := runLoop([]string{"--project-root", root, "--goal-text", "g", "--until-inbox-empty", "--resume", "--dry-run"}, nil, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "[chain] --resume runs a single batch; chaining is not applied to a resumed cycle") {
		t.Fatalf("stderr=%q, want the chained-resume warning", stderr.String())
	}
	var dry map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &dry); err != nil {
		t.Fatalf("the single batch's dry run must print its config: %v\n%s", err, stdout.String())
	}
}

func TestRunLoop_ResumeWithoutChainingStaysQuiet(t *testing.T) {
	root := t.TempDir()
	var stdout, stderr bytes.Buffer

	code := runLoop([]string{"--project-root", root, "--goal-text", "g", "--resume", "--dry-run"}, nil, &stdout, &stderr)

	if code != 0 || strings.Contains(stderr.String(), "[chain]") {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
}

func TestRegisterPerAgentOverrides_ParsesAndRefusesPairs(t *testing.T) {
	cases := []struct {
		name      string
		args      []string
		wantCLI   map[string]string
		wantModel map[string]string
		wantErr   string
	}{
		{"none", nil, map[string]string{}, map[string]string{}, ""},
		{"trimmed pairs", []string{"--cli", " auditor = claude-tmux ", "--model", "auditor= opus"}, map[string]string{"auditor": "claude-tmux"}, map[string]string{"auditor": "opus"}, ""},
		{"a repeated agent keeps the last", []string{"--model", "a=x", "--model", "a=y"}, map[string]string{}, map[string]string{"a": "y"}, ""},
		{"a value with an equals sign", []string{"--model", "a=b=c"}, map[string]string{}, map[string]string{"a": "b=c"}, ""},
		{"a model without equals", []string{"--model", "opus"}, nil, nil, `--model expects agent=model (e.g. --model auditor=opus); got "opus"`},
		{"a model with a blank agent", []string{"--model", " =opus"}, nil, nil, `--model expects agent=model`},
		{"a model with a blank value", []string{"--model", "auditor= "}, nil, nil, `--model expects agent=model`},
		{"a cli with a blank value", []string{"--cli", "auditor="}, nil, nil, `--cli expects agent=cli`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fs := flag.NewFlagSet("t", flag.ContinueOnError)
			fs.SetOutput(io.Discard)
			cli, model := registerPerAgentOverrides(fs)

			err := fs.Parse(tc.args)

			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err=%v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil || !maps.Equal(cli, tc.wantCLI) || !maps.Equal(model, tc.wantModel) {
				t.Fatalf("cli=%v model=%v err=%v", cli, model, err)
			}
		})
	}
}

func TestAbsOrWarn_ResolvesOrWarnsAndKeepsThePath(t *testing.T) {
	var stderr bytes.Buffer
	if got := absOrWarn(&stderr, "--project-root", "/a/./b/.."); got != "/a" || stderr.Len() != 0 {
		t.Fatalf("got %q stderr=%q", got, stderr.String())
	}

	previous := loopAbsoluteRoot
	loopAbsoluteRoot = func(label, p string, warn func(string)) string {
		warn("could not resolve " + label + " " + p)
		return p
	}
	t.Cleanup(func() { loopAbsoluteRoot = previous })
	stderr.Reset()

	got := absOrWarn(&stderr, "--evolve-dir", "rel")

	if got != "rel" || stderr.String() != "evolve loop: WARN: could not resolve --evolve-dir rel\n" {
		t.Fatalf("got %q stderr=%q", got, stderr.String())
	}
}

func TestWithDetachLaunch_ResolvesTheLogAndTheChildArgv(t *testing.T) {
	t.Run("not detached is a no-op", func(t *testing.T) {
		in := loopConfig{ProjectRoot: "/p"}
		got, rc := withDetachLaunch(in, &loopArgFlags{logPath: "x"}, []string{"--log", "x"}, 2, io.Discard)
		if rc != 0 || got.LogPath != "" || got.ProjectRoot != "/p" || got.DetachArgv != nil {
			t.Fatalf("got %+v rc=%d", got, rc)
		}
	})
	t.Run("a relative log becomes absolute", func(t *testing.T) {
		dir := t.TempDir()
		chdirForTest(t, dir)
		got, rc := withDetachLaunch(loopConfig{}, &loopArgFlags{detach: true, logPath: "loop.log"}, []string{"--detach", "--log", "loop.log", "--cycles", "2"}, 5, io.Discard)
		if rc != 0 || !filepath.IsAbs(got.LogPath) || filepath.Base(got.LogPath) != "loop.log" {
			t.Fatalf("got %+v rc=%d", got, rc)
		}
		if strings.Contains(strings.Join(got.DetachArgv, " "), "--detach") {
			t.Fatalf("the child argv must drop --detach: %q", got.DetachArgv)
		}
	})
	t.Run("an unresolvable log is a usage error", func(t *testing.T) {
		previous := loopDetachLogAbs
		loopDetachLogAbs = func(string) (string, error) { return "", errors.New("cwd gone") }
		t.Cleanup(func() { loopDetachLogAbs = previous })
		var stderr bytes.Buffer
		got, rc := withDetachLaunch(loopConfig{ProjectRoot: "/p"}, &loopArgFlags{detach: true, logPath: "loop.log"}, nil, 0, &stderr)
		if rc != 10 || got.ProjectRoot != "" || stderr.String() != "evolve loop: --log \"loop.log\": cwd gone\n" {
			t.Fatalf("got %+v rc=%d stderr=%q", got, rc, stderr.String())
		}
	})
}

func TestValidateLoopModes_EveryConflictIsAUsageError(t *testing.T) {
	cases := []struct {
		name string
		lf   loopArgFlags
		want string
	}{
		{"nothing set", loopArgFlags{}, ""},
		{"detach with a log", loopArgFlags{detach: true, logPath: "l"}, ""},
		{"preflight only", loopArgFlags{preflightOnly: true}, ""},
		{"detach and preflight", loopArgFlags{detach: true, preflightOnly: true, logPath: "l"}, "--detach and --preflight-only are mutually exclusive"},
		{"preflight and skip", loopArgFlags{preflightOnly: true, skipPreflight: true}, "--preflight-only and --skip-preflight are mutually exclusive (nothing would be checked)"},
		{"preflight and dry run", loopArgFlags{preflightOnly: true, dryRun: true}, "--preflight-only and --dry-run are mutually exclusive"},
		{"detach and dry run", loopArgFlags{detach: true, dryRun: true, logPath: "l"}, "--detach and --dry-run are mutually exclusive"},
		{"detach without a log", loopArgFlags{detach: true}, "--detach requires --log F"},
		{"a log without detach", loopArgFlags{logPath: "l"}, "--log is only valid with --detach"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stderr bytes.Buffer
			lf := tc.lf

			rc := validateLoopModes(&lf, &stderr)

			if tc.want == "" {
				if rc != 0 || stderr.Len() != 0 {
					t.Fatalf("rc=%d stderr=%q, want acceptance", rc, stderr.String())
				}
				return
			}
			if rc != 10 || stderr.String() != "evolve loop: "+tc.want+"\n" {
				t.Fatalf("rc=%d stderr=%q, want %q", rc, stderr.String(), tc.want)
			}
		})
	}
}

func TestRunLedger_RoutesSealAndEvidence(t *testing.T) {
	t.Run("seal", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		if rc := runLedger([]string{"seal", "--keep-it"}, nil, &stdout, &stderr); rc != 10 || !strings.Contains(stderr.String(), "flag provided but not defined: -keep-it") {
			t.Fatalf("rc=%d stderr=%q, want the seal flag set's refusal", rc, stderr.String())
		}
	})
	t.Run("evidence bad flag", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		if rc := runLedger([]string{"evidence", "restore", "--wet"}, nil, &stdout, &stderr); rc != 10 || !strings.Contains(stderr.String(), "flag provided but not defined: -wet") {
			t.Fatalf("rc=%d stderr=%q", rc, stderr.String())
		}
	})
}

func TestRunLedgerEvidence_AnUnreadableLedgerFailsTheWalk(t *testing.T) {
	for _, dryRun := range []bool{false, true} {
		evolveDir := t.TempDir()
		writeE2File(t, filepath.Join(evolveDir, "ledger.jsonl", "blocker"), "a dir where the ledger goes")
		args := []string{"restore", "--evolve-dir", evolveDir}
		if dryRun {
			args = append(args, "--dry-run")
		}
		var stdout, stderr bytes.Buffer

		rc := runLedgerEvidence(args, &stdout, &stderr)

		if rc != 1 || !strings.HasPrefix(stderr.String(), "[ledger] evidence restore failed: ledger evidence restore: ") || stdout.Len() != 0 {
			t.Fatalf("dryRun=%v rc=%d stdout=%q stderr=%q", dryRun, rc, stdout.String(), stderr.String())
		}
	}
}
