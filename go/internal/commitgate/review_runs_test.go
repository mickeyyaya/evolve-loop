package commitgate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/launchrecord"
)

const stagedDiff = "diff --git a/x.go b/x.go\n"

func stagedTree() string {
	sum := sha256.Sum256([]byte(stagedDiff))
	return hex.EncodeToString(sum[:])
}

func reviewRun(id, profile string) launchrecord.Record {
	return launchrecord.Record{
		RunID: id, Profile: profile, CLI: "agy-tmux", Family: "agy", Model: "gemini-3-pro", Verdict: "PASS",
		TreeBefore: stagedTree(), TreeAfter: stagedTree(), ArtifactSHA256: "a1", TS: "2026-10-27T09:00:00Z",
	}
}

func goGate(t *testing.T, reviewers string, records ...launchrecord.Record) (Options, *scriptRunner) {
	t.Helper()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.mod"), "module example.com/x\n\ngo 1.22\n")
	mustWrite(t, filepath.Join(root, "x.go"), "package x\n")
	for _, r := range records {
		if _, err := launchrecord.Write(filepath.Join(root, ".evolve"), r); err != nil {
			t.Fatal(err)
		}
	}
	o := baseOpts(root, "shasum", "go")
	o.Reviewers = reviewers
	o.Env = os.Environ()
	sr := &scriptRunner{rules: []scriptRule{
		{matchPrefix: "git diff --name-only HEAD", stdout: "x.go\n"},
		{matchPrefix: "git diff HEAD", stdout: stagedDiff, exit: 1},
	}}
	o.Runner = sr.run()
	return o, sr
}

func TestRun_BridgeRunReviewsSatisfyTheGateAndTheAttestationNamesEachRun(t *testing.T) {
	o, _ := goGate(t, "run:r-simplify,run:r-go", reviewRun("r-simplify", "code-simplifier"), reviewRun("r-go", "go-reviewer"))

	res := o.Run(context.Background())

	if res.ExitCode != ExitPass {
		t.Fatalf("ExitCode = %d, want pass (%v)", res.ExitCode, res.Logs)
	}
	if !reflect.DeepEqual(res.Attestation.ReviewersRun, []string{"code-simplifier", "go-reviewer"}) {
		t.Errorf("ReviewersRun = %v, want the profiles of the runs", res.Attestation.ReviewersRun)
	}
	want := []ReviewRun{
		{RunID: "r-simplify", Profile: "code-simplifier", CLI: "agy-tmux", Family: "agy", Model: "gemini-3-pro"},
		{RunID: "r-go", Profile: "go-reviewer", CLI: "agy-tmux", Family: "agy", Model: "gemini-3-pro"},
	}
	if !reflect.DeepEqual(res.Attestation.ReviewRuns, want) {
		t.Errorf("ReviewRuns = %+v, want %+v", res.Attestation.ReviewRuns, want)
	}
	raw, err := os.ReadFile(filepath.Join(o.RepoRoot, ".commit-gate", "attestation.json"))
	if err != nil {
		t.Fatal(err)
	}
	var onDisk struct {
		ReviewRuns []ReviewRun `json:"review_runs"`
	}
	if err := json.Unmarshal(raw, &onDisk); err != nil || !reflect.DeepEqual(onDisk.ReviewRuns, want) {
		t.Errorf("attestation file review_runs = %+v err=%v, want %+v", onDisk.ReviewRuns, err, want)
	}
}

func TestRun_ABridgeRunMixesWithANamedReviewer(t *testing.T) {
	warned := reviewRun("r-go", "go-reviewer")
	warned.Verdict = "WARN"
	o, _ := goGate(t, "code-simplifier, run:r-go ", warned)

	res := o.Run(context.Background())

	if res.ExitCode != ExitPass || !reflect.DeepEqual(res.Attestation.ReviewersRun, []string{"code-simplifier", "go-reviewer"}) {
		t.Errorf("exit %d attestation %+v, want pass with both reviewers (%v)", res.ExitCode, res.Attestation, res.Logs)
	}
}

func TestRun_ABridgeRunThatCannotAttestIsDenied(t *testing.T) {
	failed := reviewRun("r-go", "go-reviewer")
	failed.Verdict = "FAIL"
	other := reviewRun("r-go", "go-reviewer")
	other.TreeAfter = "another-tree"
	noLaunch := reviewRun("r-go", "go-reviewer")
	noLaunch.CLI = ""
	noVerdict := reviewRun("r-go", "go-reviewer")
	noVerdict.Verdict = ""
	writer := reviewRun("r-go", "go-reviewer")
	writer.WritesSource = true
	skipped := reviewRun("r-go", "go-reviewer")
	skipped.Verdict = "SKIPPED"
	noReport := reviewRun("r-go", "go-reviewer")
	noReport.Verdict, noReport.ArtifactSHA256 = "WARN", ""
	cases := []struct {
		name   string
		record *launchrecord.Record
		token  string
		reason string
	}{
		{"no record", nil, "run:r-go", "r-go"},
		{"a bad run id", nil, "run:../x", "../x"},
		{"a FAIL verdict", &failed, "run:r-go", "FAIL"},
		{"another tree", &other, "run:r-go", "staged tree"},
		{"no bridge launch", &noLaunch, "run:r-go", "no bridge launch"},
		{"no verdict (a dispatch error)", &noVerdict, "run:r-go", "not PASS or WARN"},
		{"a writer run", &writer, "run:r-go", "a writer"},
		{"a SKIPPED review", &skipped, "run:r-go", "not PASS or WARN"},
		{"a WARN review with no report", &noReport, "run:r-go", "no report"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var records []launchrecord.Record
			if tc.record != nil {
				records = append(records, *tc.record)
			}
			o, _ := goGate(t, "code-simplifier,"+tc.token, records...)

			res := o.Run(context.Background())

			logs := strings.Join(res.Logs, "\n")
			if res.ExitCode != ExitFail || res.Attestation != nil || !strings.Contains(logs, tc.reason) || !strings.Contains(logs, "DENY") {
				t.Errorf("exit %d attestation %v logs %q, want a DENY that names %q", res.ExitCode, res.Attestation, logs, tc.reason)
			}
		})
	}
}

func TestRun_ATreeStateFailureWhileResolvingABridgeRunIsGitFatal(t *testing.T) {
	o, sr := goGate(t, "code-simplifier,run:r-go", reviewRun("r-go", "go-reviewer"))
	sr.rules[1].exit = 128

	res := o.Run(context.Background())

	if res.ExitCode != ExitGitFatal || !strings.Contains(strings.Join(res.Logs, "\n"), "tree") {
		t.Errorf("exit %d logs %v, want git fatal with a tree reason", res.ExitCode, res.Logs)
	}
}

func TestRun_NamedReviewersAloneReadNoTreeBeforeTheLanes(t *testing.T) {
	o, sr := goGate(t, "code-simplifier,go-reviewer")

	res := o.Run(context.Background())

	diffs := 0
	for _, c := range sr.calls {
		if c == "git diff HEAD" {
			diffs++
		}
	}
	if res.ExitCode != ExitPass || diffs != 1 || res.Attestation.ReviewRuns != nil {
		t.Errorf("exit %d tree reads %d review runs %v, want pass, one tree read for the attestation and no runs", res.ExitCode, diffs, res.Attestation.ReviewRuns)
	}
}

func TestAttestDirName_IsTheDefaultAttestationDirectory(t *testing.T) {
	o := Options{RepoRoot: "/repo"}
	if got, want := o.attestPath(), filepath.Join("/repo", AttestDirName, "attestation.json"); got != want || AttestDirName != ".commit-gate" {
		t.Errorf("attestPath = %q, AttestDirName = %q; want %q under .commit-gate", got, AttestDirName, want)
	}
}
