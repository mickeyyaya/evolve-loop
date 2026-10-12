//go:build integration

package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
)

func TestDefaultBootRefreshRebuild_RunsTheCanonicalBuildWithChatterOnStderr(t *testing.T) {
	root := bfRealDir(t, t.TempDir())
	record := bfStub(t, "make", "echo build-out; echo build-err >&2")
	var stderr bytes.Buffer

	err := defaultBootRefreshRebuild(root, &stderr)

	if err != nil || bfCalls(t, record) != root+"|-C go build\n" {
		t.Fatalf("defaultBootRefreshRebuild = %v, make calls %q; want one `make -C go build` in the root", err, bfCalls(t, record))
	}
	if got := stderr.String(); got != "build-out\nbuild-err\n" {
		t.Fatalf("stderr = %q, want both build streams", got)
	}
}

func TestDefaultBootRefreshRebuild_ABuildFailureIsItsExit(t *testing.T) {
	bfStub(t, "make", "exit 2")

	err := defaultBootRefreshRebuild(t.TempDir(), &bytes.Buffer{})

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 2 {
		t.Fatalf("defaultBootRefreshRebuild = %v, want make's exit 2", err)
	}
}

func TestDefaultChainRebuild_SucceedsQuietlyOrCarriesTheBuildOutput(t *testing.T) {
	root := bfRealDir(t, t.TempDir())
	record := bfStub(t, "make", `[ -n "$FAIL_BUILD" ] && { echo "compile error in x.go"; echo "  " >&2; exit 2; }; echo ok`)

	if err := defaultChainRebuild(root); err != nil || bfCalls(t, record) != root+"|-C go build\n" {
		t.Fatalf("defaultChainRebuild = %v, calls %q; want success from `make -C go build` in the root", err, bfCalls(t, record))
	}
	t.Setenv("FAIL_BUILD", "1")
	err := defaultChainRebuild(root)
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || err.Error() != "make -C go build: exit status 2: compile error in x.go" {
		t.Fatalf("defaultChainRebuild = %v, want the trimmed build output in the error", err)
	}
}

func TestMainCIRedForSHA_JudgesOnlyCompletedRuns(t *testing.T) {
	sha := "0123456789abcdef0123456789abcdef01234567"
	for _, tc := range []struct {
		name, response string
		red            bool
		failed         []string
		wantErr        string
	}{
		{name: "green with pending", response: `{"check_runs":[{"name":"a","status":"in_progress","conclusion":"failure"},{"name":"b","status":"completed","conclusion":"success"},{"name":"c","status":"completed","conclusion":"neutral"},{"name":"d","status":"completed","conclusion":"skipped"}]}`},
		{name: "red", response: `{"check_runs":[{"name":"a","status":"completed","conclusion":"success"},{"name":"b","status":"completed","conclusion":"failure"},{"name":"c","status":"completed","conclusion":"cancelled"},{"name":"d","status":"completed","conclusion":""}]}`, red: true, failed: []string{"b=failure", "c=cancelled", "d="}},
		{name: "nothing completed", response: `{"check_runs":[{"name":"a","status":"queued"}]}`, wantErr: "no completed check-runs visible for 0123456789ab"},
		{name: "no runs", response: `{"check_runs":[]}`, wantErr: "no completed check-runs visible for 0123456789ab"},
		{name: "not json", response: `<html>rate limited</html>`, wantErr: "decode check-runs: "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			installCheckRunsGH(t, tc.response, false)

			red, failed, err := mainCIRedForSHA(context.Background(), t.TempDir(), sha)

			gotErr := ""
			if err != nil {
				gotErr = err.Error()
			}
			if red != tc.red || !reflect.DeepEqual(failed, tc.failed) || tc.wantErr == "" && gotErr != "" || !strings.HasPrefix(gotErr, tc.wantErr) {
				t.Fatalf("mainCIRedForSHA = %v, %q, %q; want %v, %q, %q", red, failed, gotErr, tc.red, tc.failed, tc.wantErr)
			}
		})
	}
}

func TestGhIn_RunsInTheRootAndReturnsStdout(t *testing.T) {
	root := bfRealDir(t, t.TempDir())
	record := bfStub(t, "gh", `[ "$1" = fail ] && { echo 'gh: boom ' >&2; exit 1; }; printf '{"number":12}'; echo noise >&2`)
	run := ghIn(root)

	out, err := run(context.Background(), "pr", "view", "12")
	if err != nil || string(out) != `{"number":12}` {
		t.Fatalf("ghIn = %q, %v; want stdout only", out, err)
	}
	if calls := bfCalls(t, record); calls != root+"|pr view 12\n" {
		t.Fatalf("gh calls %q, want one call in the root", calls)
	}
	out, err = run(context.Background(), "fail", "x y")
	var exitErr *exec.ExitError
	if out != nil || !errors.As(err, &exitErr) || err.Error() != "gh fail x y: exit status 1: gh: boom" {
		t.Fatalf("ghIn failure = %q, %v; want nil and the args, exit and trimmed stderr", out, err)
	}
}

func TestStatusOpenPRs_ReportsEachOpenPRWithItsCheckState(t *testing.T) {
	root := bfRealDir(t, t.TempDir())
	record := bfStub(t, "gh", `printf '[{"number":7,"title":"t7","headRefName":"cycle-7","url":"u7","statusCheckRollup":[{"name":"build","conclusion":"FAILURE"}]},{"number":8,"title":"t8","headRefName":"cycle-8","url":"u8","statusCheckRollup":[]}]'`)

	got := statusOpenPRs(context.Background(), root)

	want := statusRemote{Available: true, Items: []statusPR{
		{Number: 7, Title: "t7", HeadRefName: "cycle-7", URL: "u7", Checks: "failing", FailingChecks: []string{"build"}},
		{Number: 8, Title: "t8", HeadRefName: "cycle-8", URL: "u8", Checks: "none"},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("statusOpenPRs = %+v, want %+v", got, want)
	}
	if calls := bfCalls(t, record); calls != root+"|pr list --state open --json number,title,headRefName,url,statusCheckRollup\n" {
		t.Fatalf("gh calls %q, want the open-PR listing in the root", calls)
	}
}

func TestStatusOpenPRs_NoOpenPRsIsAvailableAndEmpty(t *testing.T) {
	bfStub(t, "gh", `printf '[]'`)

	got := statusOpenPRs(context.Background(), t.TempDir())

	if !got.Available || got.Error != "" || !reflect.DeepEqual(got.Items, []statusPR{}) {
		t.Fatalf("statusOpenPRs = %+v, want available with an empty list", got)
	}
}

func TestStatusOpenPRs_FailuresAreUnavailableWithTheReason(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"gh fails", "exit 1", "gh pr list: exit status 1"},
		{"not json", "printf 'not json'", "gh pr list: invalid character"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bfStub(t, "gh", tc.body)

			got := statusOpenPRs(context.Background(), t.TempDir())

			if got.Available || got.Items != nil || !strings.HasPrefix(got.Error, tc.want) {
				t.Fatalf("statusOpenPRs = %+v, want unavailable with %q", got, tc.want)
			}
		})
	}
}

func TestGhMergedPRHeads_ListsMergedHeadsOfTheBranch(t *testing.T) {
	dir := bfRealDir(t, t.TempDir())
	record := bfStub(t, "gh", `printf 'aaa\nbbb\n'`)

	out, err := ghMergedPRHeads(context.Background(), dir, "dev-task")

	if err != nil || out != "aaa\nbbb\n" {
		t.Fatalf("ghMergedPRHeads = %q, %v; want the raw head list", out, err)
	}
	if calls := bfCalls(t, record); calls != dir+"|pr list --head dev-task --state merged --json headRefOid --jq .[].headRefOid\n" {
		t.Fatalf("gh calls %q, want the merged-head query in the dir", calls)
	}
}

func TestGhMergedPRHeads_FailuresAreReturned(t *testing.T) {
	t.Run("gh missing", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		out, err := ghMergedPRHeads(context.Background(), t.TempDir(), "b")
		if out != "" || !errors.Is(err, exec.ErrNotFound) {
			t.Fatalf("ghMergedPRHeads = %q, %v; want exec.ErrNotFound", out, err)
		}
	})
	t.Run("gh fails", func(t *testing.T) {
		bfStub(t, "gh", "printf partial; exit 1")
		out, err := ghMergedPRHeads(context.Background(), t.TempDir(), "b")
		var exitErr *exec.ExitError
		if out != "partial" || !errors.As(err, &exitErr) {
			t.Fatalf("ghMergedPRHeads = %q, %v; want the partial output and the exit", out, err)
		}
	})
	t.Run("canceled", func(t *testing.T) {
		bfStub(t, "gh", "printf heads")
		out, err := ghMergedPRHeads(canceledCtx(), t.TempDir(), "b")
		if out != "" || !errors.Is(err, context.Canceled) {
			t.Fatalf("ghMergedPRHeads = %q, %v; want context.Canceled", out, err)
		}
	})
}

func TestBinaryAnswersSubcommandDep_OnlyAnUnknownCommandIsARefusal(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body, want string
	}{
		{"answers", "echo usage", ""},
		{"exits non-zero without refusing", "echo 'flag provided but not defined' >&2; exit 2", ""},
		{"another subcommand unknown", `echo 'unknown command "other"' >&2; exit 1`, ""},
		{"refuses", `echo 'Error: unknown command "wave" for "evolve"' >&2; exit 1`, `binary does not recognize subcommand "wave"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bin := filepath.Join(t.TempDir(), "evolve")
			fakeclitest.Install(t, bin, "#!/bin/sh\n[ \"$1 $2\" = \"wave --help\" ] || exit 9\n"+tc.body+"\n")

			err := binaryAnswersSubcommandDep(bin, "wave")

			gotErr := ""
			if err != nil {
				gotErr = err.Error()
			}
			if gotErr != tc.want {
				t.Fatalf("binaryAnswersSubcommandDep = %q, want %q", gotErr, tc.want)
			}
		})
	}
}

func TestBinaryAnswersSubcommandDep_AMissingBinaryIsALaunchError(t *testing.T) {
	t.Parallel()
	missing := filepath.Join(t.TempDir(), "evolve")

	err := binaryAnswersSubcommandDep(missing, "wave")

	if !errors.Is(err, os.ErrNotExist) || !strings.HasPrefix(err.Error(), "run \""+missing+"\" wave: ") {
		t.Fatalf("binaryAnswersSubcommandDep = %v, want the wrapped launch error", err)
	}
}
