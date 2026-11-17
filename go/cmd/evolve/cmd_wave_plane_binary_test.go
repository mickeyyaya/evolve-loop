package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestRunPlaneBinary_ARefreshWithoutAProjectRootIsAUsageErrorAndRunsNothing(t *testing.T) {
	t.Parallel()
	plane := &planeBinary{verdict: func() (int, string) { return 0, "" }}
	var stdout, stderr bytes.Buffer

	rc := runPlaneBinary(plane.run, "code", []string{"refresh"}, &stdout, &stderr)

	if rc != exitUsage || len(plane.calls) != 0 || !strings.Contains(stderr.String(), "needs --project-root") {
		t.Errorf("rc=%d calls=%v stderr=%q, want a usage error and no run", rc, plane.calls, stderr.String())
	}
}

func TestRunPlaneBinary_ABinaryThatCannotStartIsAnIOErrorThatNamesTheCause(t *testing.T) {
	t.Parallel()
	run := func(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
		return 0, errors.New("no such binary")
	}
	var stdout, stderr bytes.Buffer

	rc := runPlaneBinary(run, "code", []string{"refresh", "--project-root", t.TempDir()}, &stdout, &stderr)

	if rc != exitIO || !strings.Contains(stderr.String(), "no such binary") {
		t.Errorf("rc=%d stderr=%q, want exitIO naming the cause", rc, stderr.String())
	}
}

func TestRunPlaneBinary_AnAbsentCodeIndexIsSuccessAndOtherVerbsKeepTheirExitCode(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		verb    string
		planeRC int
		want    int
	}{{"code", codeExitAbsent, 0}, {"gc", codeExitAbsent, codeExitAbsent}, {"code", 7, 7}} {
		plane := &planeBinary{verdict: func() (int, string) { return c.planeRC, "" }}
		var stdout, stderr bytes.Buffer

		rc := runPlaneBinary(plane.run, c.verb, []string{"--project-root", t.TempDir()}, &stdout, &stderr)

		if rc != c.want {
			t.Errorf("verb %s plane rc %d: got %d, want %d", c.verb, c.planeRC, rc, c.want)
		}
	}
}

func TestDispatchWaveVerb_ARepoLocalVerbRunsItsOwnHandlerNotThePlaneBinary(t *testing.T) {
	t.Parallel()
	plane := &planeBinary{verdict: func() (int, string) { return 0, "" }}
	var stdout, stderr bytes.Buffer
	var wantOut, wantErr bytes.Buffer

	rc := dispatchWaveVerb(plane.run)("checkpoint", nil, &stdout, &stderr)

	want := runCheckpoint(nil, strings.NewReader(""), &wantOut, &wantErr)
	if rc != want || len(plane.calls) != 0 {
		t.Errorf("rc=%d want %d, plane calls=%v, want the checkpoint handler alone", rc, want, plane.calls)
	}
}
