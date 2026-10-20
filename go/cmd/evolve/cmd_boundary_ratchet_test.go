package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/sizeratchet"
)

const ratchetListPath = "go/" + sizeratchet.OffendersRelPath

type ratchetOrigin struct {
	bare  *gittest.Repo
	plane *gittest.Repo
}

func newRatchetOrigin(t *testing.T, funcLines int, offenders string) ratchetOrigin {
	t.Helper()
	seed := gittest.Fixture(t)
	var src strings.Builder
	src.WriteString("package fixture\n\nfunc Big() int {\n\ttotal := 0\n")
	for i := 0; i < funcLines; i++ {
		fmt.Fprintf(&src, "\ttotal += %d\n", i)
	}
	src.WriteString("\treturn total\n}\n")
	seed.Write("go/internal/fixture/big.go", src.String())
	if offenders != "" {
		seed.Write(ratchetListPath, offenders)
	}
	seed.Git("add", "-A")
	seed.Git("commit", "-q", "-m", "seed")
	bare := gittest.Bare(t)
	seed.Git("remote", "add", "origin", bare.Dir)
	seed.Git("push", "-q", "origin", "main")
	return ratchetOrigin{bare: bare, plane: gittest.Clone(t, bare.Dir)}
}

func (o ratchetOrigin) tighten(t *testing.T) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	rc := runBoundaryRatchetTighten([]string{"--project-root", o.plane.Dir}, nil, &stdout, &stderr)
	return rc, stdout.String(), stderr.String()
}

func (o ratchetOrigin) originHead() string { return o.bare.Git("rev-parse", "main") }

func (o ratchetOrigin) planeIsUntouched(t *testing.T) {
	t.Helper()
	if status := o.plane.Git("status", "--porcelain"); status != "" {
		t.Errorf("the plane tree is dirty after the step: %q", status)
	}
	if list := o.plane.Git("worktree", "list", "--porcelain"); strings.Count(list, "worktree ") != 1 {
		t.Errorf("the step left a worktree behind:\n%s", list)
	}
}

func TestBoundaryRatchetTighten_LandsTheTightenedListOnOriginMain(t *testing.T) {
	o := newRatchetOrigin(t, 60, `{"internal/fixture.Big": 90, "internal/fixture.Gone": 60}`)
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "")
	before := o.originHead()

	rc, out, errOut := o.tighten(t)

	if rc != 0 || errOut != "" || !strings.Contains(out, "internal/fixture.Big: 90 -> 64") || !strings.Contains(out, "internal/fixture.Gone: 60 removed") {
		t.Fatalf("rc=%d stdout=%q stderr=%q, want each change named and no warning", rc, out, errOut)
	}
	if got := o.bare.Git("show", "main:"+ratchetListPath); got != "{\n  \"internal/fixture.Big\": 64\n}" {
		t.Errorf("origin main holds %q, want the tightened list", got)
	}
	if parent := o.bare.Git("rev-parse", "main^"); parent != before {
		t.Errorf("origin main^ = %s, want the old head %s: one commit on top, no rewrite", parent, before)
	}
	if subject := o.bare.Git("log", "-1", "--format=%s", "main"); subject != ratchetTightenMessage {
		t.Errorf("commit subject %q, want %q", subject, ratchetTightenMessage)
	}
	if files := o.bare.Git("show", "--name-only", "--format=", "main"); files != ratchetListPath {
		t.Errorf("the commit touches %q, want only %s", files, ratchetListPath)
	}
	o.planeIsUntouched(t)
}

func TestBoundaryRatchetTighten_ATightListLandsNothing(t *testing.T) {
	o := newRatchetOrigin(t, 60, `{"internal/fixture.Big": 64}`)
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "")
	before := o.originHead()

	rc, out, errOut := o.tighten(t)

	if rc != 0 || errOut != "" || !strings.Contains(out, "ratchet tighten: tight") || o.originHead() != before {
		t.Fatalf("rc=%d stdout=%q stderr=%q origin moved=%v, want a tight report and no commit", rc, out, errOut, o.originHead() != before)
	}
	o.planeIsUntouched(t)
}

func TestBoundaryRatchetTighten_AFailureWarnsAndNeverStopsTheBoundary(t *testing.T) {
	cases := []struct {
		name    string
		prep    func(t *testing.T) ratchetOrigin
		wantErr string
	}{
		{"a red ratchet on main is not pushed", func(t *testing.T) ratchetOrigin {
			return newRatchetOrigin(t, 60, `{"internal/fixture.Big": 60, "internal/fixture.Gone": 60}`)
		}, "function-size ratchet violated"},
		{"a module without the list", func(t *testing.T) ratchetOrigin {
			return newRatchetOrigin(t, 60, "")
		}, "read offender list"},
		{"a refused push", func(t *testing.T) ratchetOrigin {
			o := newRatchetOrigin(t, 60, `{"internal/fixture.Big": 90}`)
			hook := filepath.Join(o.bare.Dir, "hooks", "pre-receive")
			if err := os.WriteFile(hook, []byte("#!/bin/sh\necho protected >&2\nexit 1\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			return o
		}, "protected"},
		{"a plane without origin", func(t *testing.T) ratchetOrigin {
			o := newRatchetOrigin(t, 60, `{"internal/fixture.Big": 90}`)
			o.plane.Git("remote", "remove", "origin")
			return o
		}, "fetch"},
		{"a live cycle", func(t *testing.T) ratchetOrigin {
			o := newRatchetOrigin(t, 60, `{"internal/fixture.Big": 90}`)
			runDir := filepath.Join(o.plane.Dir, ".evolve", "runs", "cycle-7")
			if err := os.MkdirAll(runDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := runlease.Write(runDir, runlease.Lease{OwnerPID: os.Getpid()}, time.Now()); err != nil {
				t.Fatal(err)
			}
			return o
		}, "cycle-7"},
	}
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "")
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			o := c.prep(t)
			before := o.originHead()

			rc, _, errOut := o.tighten(t)

			if rc != 0 || !strings.Contains(errOut, "WARN") || !strings.Contains(errOut, c.wantErr) {
				t.Errorf("rc=%d stderr=%q, want rc 0 and a WARN naming %q", rc, errOut, c.wantErr)
			}
			if o.originHead() != before {
				t.Errorf("origin main moved after a failed tighten")
			}
		})
	}
}

func TestBoundaryRatchetTighten_ANoTempDirWarns(t *testing.T) {
	o := newRatchetOrigin(t, 60, `{"internal/fixture.Big": 90}`)
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "")
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "absent"))

	rc, _, errOut := o.tighten(t)

	if rc != 0 || !strings.Contains(errOut, "WARN") {
		t.Errorf("rc=%d stderr=%q, want a WARN for an unusable temp dir", rc, errOut)
	}
}

func TestBoundaryRatchetTighten_NeedsTheProjectRoot(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	if rc := runBoundaryRatchetTighten(nil, nil, &bytes.Buffer{}, &stderr); rc != exitUsage || !strings.Contains(stderr.String(), "--project-root") {
		t.Errorf("rc=%d stderr=%q, want a usage exit naming --project-root", rc, stderr.String())
	}
}

func TestBoundaryHaltSteps_TightenBeforeSyncMainSoThePlaneMergesIt(t *testing.T) {
	t.Parallel()
	steps := boundaryHaltSteps(boundaryArgs{prs: []string{"12"}}, "/plane")
	var verbs []string
	for _, s := range steps {
		verbs = append(verbs, s.verb)
	}
	if strings.Join(verbs, " ") != "loop-stop pr "+boundaryRatchetVerb+" sync-main gc" {
		t.Fatalf("halt steps %v, want the tighten after the PR merges and before sync-main", verbs)
	}
	if s := steps[2]; s.String() != boundaryRatchetDesc || strings.Join(s.args, " ") != "--project-root /plane" {
		t.Errorf("tighten step %q args %v", s.String(), s.args)
	}
	var stderr bytes.Buffer
	if rc := dispatchBoundaryVerb(boundaryRatchetVerb, nil, &bytes.Buffer{}, &stderr); rc != exitUsage {
		t.Errorf("the boundary dispatch has no %s handler: rc=%d stderr=%q", boundaryRatchetVerb, rc, stderr.String())
	}
}
