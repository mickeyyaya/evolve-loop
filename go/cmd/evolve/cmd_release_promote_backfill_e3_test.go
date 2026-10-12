package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

type e3GHStep struct {
	match  string
	stdout string
	code   int
	err    error
}

type e3GHRunner struct {
	t     *testing.T
	steps []e3GHStep
	calls []string
}

func (r *e3GHRunner) run(_ context.Context, name, _ string, args, _ []string, _ io.Reader, stdout, stderr io.Writer) (int, error) {
	call := name + " " + strings.Join(args, " ")
	r.calls = append(r.calls, call)
	if len(r.steps) == 0 {
		r.t.Errorf("unexpected call after the script ended: %s", call)
		return -1, errors.New("unscripted")
	}
	step := r.steps[0]
	r.steps = r.steps[1:]
	if !strings.HasPrefix(call, step.match) {
		r.t.Errorf("call %q, want one starting %q", call, step.match)
	}
	if step.err != nil {
		return -1, step.err
	}
	_, _ = io.WriteString(stdout, step.stdout)
	if step.code != 0 {
		_, _ = io.WriteString(stderr, "gh: HTTP 500")
	}
	return step.code, nil
}

const (
	e3ListCall  = "gh run list --workflow release.yml --branch v9.9.9 --limit 1 --json " + runFields + " -R mickeyyaya/evolve-loop"
	e3ViewCall  = "gh run view 77 --json " + runFields + " -R mickeyyaya/evolve-loop"
	e3RelCall   = "gh api repos/mickeyyaya/evolve-loop/releases/tags/v9.9.9"
	e3PatchCall = "gh api -X PATCH repos/mickeyyaya/evolve-loop/releases/4242 -F prerelease=false"
	e3Assets    = `{"id":4242,"assets":[{"name":"evolve_darwin_amd64.tar.gz"},{"name":"evolve_linux_arm64.tar.gz"},{"name":"checksums.txt"}]}`
)

func e3Promote(t *testing.T, rerun bool, steps ...e3GHStep) (int, string, string, *e3GHRunner) {
	t.Helper()
	r := &e3GHRunner{t: t, steps: steps}
	var out, errb bytes.Buffer
	code := promoteRelease(context.Background(), r.run, fixtureConfig(), "v9.9.9", rerun, &out, &errb)
	if len(r.steps) != 0 {
		t.Errorf("%d scripted gh calls never ran: %+v", len(r.steps), r.steps)
	}
	return code, out.String(), errb.String(), r
}

func TestPromoteRelease_PromotesOnlyAVerifiedSuccessfulRun(t *testing.T) {
	code, out, errb, r := e3Promote(t, false,
		e3GHStep{match: e3ListCall, stdout: `[{"databaseId":77,"status":"completed","conclusion":"failure"}]`},
		e3GHStep{match: e3ViewCall, stdout: `{"databaseId":77,"status":"completed","conclusion":"success"}`},
		e3GHStep{match: e3RelCall, stdout: e3Assets},
		e3GHStep{match: e3PatchCall},
	)

	if code != 0 {
		t.Fatalf("code %d stderr %q", code, errb)
	}
	if !strings.HasSuffix(out, "release-promote: v9.9.9 promoted (prerelease cleared)\n") || !strings.Contains(out, "all 2 prebuilt binaries") {
		t.Errorf("stdout = %q", out)
	}
	if strings.Join(r.calls, "\n") != strings.Join([]string{e3ListCall, e3ViewCall, e3RelCall, e3PatchCall}, "\n") {
		t.Errorf("gh calls = %q", r.calls)
	}
}

func TestPromoteRelease_RerunWatchesTheFailedJobsBeforeJudging(t *testing.T) {
	code, out, errb, r := e3Promote(t, true,
		e3GHStep{match: e3ListCall, stdout: `[{"databaseId":77}]`},
		e3GHStep{match: "gh run rerun 77 --failed -R mickeyyaya/evolve-loop"},
		e3GHStep{match: "gh run watch 77 -R mickeyyaya/evolve-loop"},
		e3GHStep{match: e3ViewCall, stdout: `{"databaseId":77,"status":"completed","conclusion":"success"}`},
		e3GHStep{match: e3RelCall, stdout: e3Assets},
		e3GHStep{match: e3PatchCall},
	)

	if code != 0 || !strings.Contains(out, "promoted") {
		t.Fatalf("code %d stdout %q stderr %q calls %q", code, out, errb, r.calls)
	}
}

func TestPromoteRelease_RefusesOrFailsWithoutPatching(t *testing.T) {
	boom := errors.New("exec: gh not found")
	list := e3GHStep{match: e3ListCall, stdout: `[{"databaseId":77}]`}
	success := e3GHStep{match: e3ViewCall, stdout: `{"status":"completed","conclusion":"success"}`}
	cases := []struct {
		name     string
		rerun    bool
		steps    []e3GHStep
		wantCode int
		wantErr  string
	}{
		{"list cannot run", false, []e3GHStep{{match: e3ListCall, err: boom}}, 2, "release-promote: sysexec: gh"},
		{"list exits non-zero", false, []e3GHStep{{match: e3ListCall, code: 1}}, 2, "exit=1: gh: HTTP 500"},
		{"list is not JSON", false, []e3GHStep{{match: e3ListCall, stdout: "<html>"}}, 2, "release-promote: decode gh run: "},
		{"no run for the tag", false, []e3GHStep{{match: e3ListCall, stdout: "[]"}}, 2, "release-promote: no release.yml workflow run found for v9.9.9"},
		{"rerun fails", true, []e3GHStep{list, {match: "gh run rerun 77", code: 1}}, 2, "exit=1"},
		{"watch fails", true, []e3GHStep{list, {match: "gh run rerun 77"}, {match: "gh run watch 77", code: 1}}, 2, "exit=1"},
		{"view fails", false, []e3GHStep{list, {match: e3ViewCall, code: 1}}, 2, "exit=1"},
		{"run still in progress", false, []e3GHStep{list, {match: e3ViewCall, stdout: `{"status":"in_progress","conclusion":""}`}}, 1,
			`release-promote: refusing: workflow run 77 status="in_progress" conclusion="" is not success; release v9.9.9 left as prerelease`},
		{"release lookup fails", false, []e3GHStep{list, success, {match: e3RelCall, code: 1}}, 2, "release-promote: sysexec: gh"},
		{"release is not JSON", false, []e3GHStep{list, success, {match: e3RelCall, stdout: "nope"}}, 2, "release-promote: decode gh api: "},
		{"an asset is missing", false, []e3GHStep{list, success, {match: e3RelCall, stdout: `{"id":4242,"assets":[{"name":"checksums.txt"}]}`}}, 1,
			"release-promote: refusing: release v9.9.9 left as prerelease"},
		{"patch fails", false, []e3GHStep{list, success, {match: e3RelCall, stdout: e3Assets}, {match: e3PatchCall, code: 1}}, 2, "exit=1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, out, errb, _ := e3Promote(t, tc.rerun, tc.steps...)

			if code != tc.wantCode || !strings.Contains(errb, tc.wantErr) {
				t.Fatalf("code %d stderr %q, want code %d containing %q", code, errb, tc.wantCode, tc.wantErr)
			}
			if strings.Contains(out, "promoted") {
				t.Errorf("a refused promotion claimed success: %q", out)
			}
		})
	}
}

func TestParseReleasePromoteArgs(t *testing.T) {
	cases := []struct {
		args      []string
		wantTag   string
		wantRerun bool
		wantOK    bool
	}{
		{nil, "", false, false},
		{[]string{"v1.2.3"}, "v1.2.3", false, true},
		{[]string{"--rerun", "v1.2.3"}, "v1.2.3", true, true},
		{[]string{"v1.2.3", "--rerun"}, "v1.2.3", true, true},
		{[]string{"--rerun"}, "", true, false},
		{[]string{"v1", "v2"}, "", false, false},
		{[]string{"-x"}, "", false, false},
		{[]string{"--rerun=true", "v1"}, "", false, false},
		{[]string{""}, "", false, false},
		{[]string{"v1", ""}, "", false, false},
		{[]string{"vÜ.1"}, "vÜ.1", false, true},
	}
	for _, tc := range cases {
		tag, rerun, ok := parseReleasePromoteArgs(tc.args)
		if tag != tc.wantTag || rerun != tc.wantRerun || ok != tc.wantOK {
			t.Errorf("parseReleasePromoteArgs(%q) = %q,%v,%v want %q,%v,%v", tc.args, tag, rerun, ok, tc.wantTag, tc.wantRerun, tc.wantOK)
		}
	}
}

func FuzzParseReleasePromoteArgs(f *testing.F) {
	for _, seed := range []string{"v1.2.3", "--rerun v1", "v1 v2", "-", "", "--rerun", "ü"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, joined string) {
		args := strings.Split(joined, " ")
		tag, rerun, ok := parseReleasePromoteArgs(args)
		if ok != (tag != "") {
			t.Fatalf("ok=%v with tag %q", ok, tag)
		}
		if ok && (tag[0] == '-' || !slices.Contains(args, tag)) {
			t.Fatalf("tag %q is a flag or not an operand of %q", tag, args)
		}
		if rerun && ok && !slices.Contains(args, "--rerun") {
			t.Fatalf("rerun without --rerun in %q", args)
		}
	})
}

func TestGHJSON_ReturnsTheRunnerErrorUnwrapped(t *testing.T) {
	boom := errors.New("network down")
	var v map[string]any

	err := ghJSON(func(...string) (string, error) { return "", boom }, &v, "api", "x")

	if !errors.Is(err, boom) || v != nil {
		t.Fatalf("err %v v %v", err, v)
	}
	if err := ghJSON(func(...string) (string, error) { return `{"a":1}`, nil }, &v, "api"); err != nil || v["a"] != float64(1) {
		t.Fatalf("decode: err %v v %v", err, v)
	}
}

func e3GoreleaserRoot(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	if body != "" {
		if err := os.WriteFile(filepath.Join(root, ".goreleaser.yml"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("EVOLVE_WORKTREE_ROOT", "")
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	return root
}

const e3GoreleaserNoRepo = `
version: 2
builds:
  - id: evolve
    targets: [ darwin_amd64 ]
archives:
  - id: evolve
    name_template: 'evolve_{{ .Os }}_{{ .Arch }}'
    formats: [ 'tar.gz' ]
checksum:
  name_template: 'checksums.txt'
`

func TestRunReleasePromote_RefusesBadArgsAndAnUnusableMatrix(t *testing.T) {
	cases := []struct {
		name, goreleaser string
		args             []string
		want             string
	}{
		{"no tag", e3GoreleaserNoRepo, []string{"--rerun"}, "release-promote: bad arguments\n" + releasePromoteUsage + "\n"},
		{"two tags", e3GoreleaserNoRepo, []string{"v1", "v2"}, "release-promote: bad arguments\n" + releasePromoteUsage + "\n"},
		{"no matrix file", "", []string{"v1"}, "release matrix "},
		{"matrix without repo", e3GoreleaserNoRepo, []string{"v1"}, "unusable: <nil>"},
		{"matrix with an owner but no repo name", e3GoreleaserNoRepo + "release:\n  github:\n    owner: e3owner\n", []string{"v1"}, "unusable: <nil>"},
		{"matrix unparsable", "builds: [ { targets: [ nounderscore ] } ]", []string{"v1"}, "unusable: "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e3GoreleaserRoot(t, tc.goreleaser)
			var out, errb bytes.Buffer

			code := runReleasePromote(tc.args, nil, &out, &errb)

			if code != 2 || out.Len() != 0 || !strings.Contains(errb.String(), tc.want) {
				t.Fatalf("code %d stdout %q stderr %q, want 2 with %q", code, out.String(), errb.String(), tc.want)
			}
		})
	}
}
