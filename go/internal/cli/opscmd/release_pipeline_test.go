package opscmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestCmd_ReleasePipeline_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := RunReleasePipeline([]string{"--help"}, nil, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc = %d, want 0", rc)
	}
	if !strings.Contains(stdout.String(), "release") {
		t.Errorf("help missing keyword: %s", stdout.String())
	}
}

func TestCmd_ReleasePipeline_NoTarget(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := RunReleasePipeline(nil, nil, &stdout, &stderr)
	if rc != 10 {
		t.Fatalf("rc = %d, want 10", rc)
	}
}

func TestCmd_ReleasePipeline_UnknownFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := RunReleasePipeline([]string{"--bogus"}, nil, &stdout, &stderr)
	if rc != 10 {
		t.Fatalf("rc = %d, want 10", rc)
	}
}

func TestCmd_ReleasePipeline_BadMaxPollWait(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := RunReleasePipeline([]string{"--max-poll-wait-s", "0", "1.2.3"}, nil, &stdout, &stderr)
	if rc != 10 {
		t.Fatalf("rc = %d, want 10", rc)
	}
}

func TestCmd_ReleasePipeline_MaxPollWaitMissingValue(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := RunReleasePipeline([]string{"--max-poll-wait-s"}, nil, &stdout, &stderr)
	if rc != 10 {
		t.Fatalf("rc = %d, want 10", rc)
	}
}

func TestCmd_ReleasePipeline_FromTagMissingValue(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := RunReleasePipeline([]string{"--from-tag"}, nil, &stdout, &stderr)
	if rc != 10 {
		t.Fatalf("rc = %d, want 10", rc)
	}
}

func TestCmd_ReleasePipeline_ExtraPositional(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := RunReleasePipeline([]string{"1.2.3", "extra"}, nil, &stdout, &stderr)
	if rc != 10 {
		t.Fatalf("rc = %d, want 10", rc)
	}
}

func TestCmd_ReleasePipeline_InvalidSemver(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", t.TempDir())
	var stdout, stderr bytes.Buffer
	rc := RunReleasePipeline([]string{"garbage"}, nil, &stdout, &stderr)
	if rc != 1 {
		t.Fatalf("rc = %d, want 1 (pre-publish failure on bad semver)", rc)
	}
}

func TestCmd_ReleasePipeline_EnvRequirePreflight(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", t.TempDir())
	var stdout, stderr bytes.Buffer
	rc := RunReleasePipeline([]string{"99.99.99", "--dry-run", "--require-preflight"}, nil, &stdout, &stderr)
	if rc == 0 || rc == 10 {
		t.Errorf("rc = %d, want pre-publish or post-publish error (1/2/3)", rc)
	}
}

func TestParseReleasePipelineArgs_SkipContentReviewTakesATrimmedReason(t *testing.T) {
	var stdout, stderr bytes.Buffer

	f, code, done := parseReleasePipelineArgs([]string{"--skip-content-review", "  models down ", "1.2.0"}, &stdout, &stderr)

	if done || code != 0 || f.skipContentReview != "models down" || f.target != "1.2.0" {
		t.Fatalf("got flags=%+v code=%d done=%v", f, code, done)
	}
}

func TestParseReleasePipelineArgs_SkipContentReviewWithoutAReasonIsRefused(t *testing.T) {
	cases := [][]string{
		{"1.2.0", "--skip-content-review"},
		{"--skip-content-review", "   ", "1.2.0"},
	}
	for _, args := range cases {
		var stdout, stderr bytes.Buffer

		_, code, done := parseReleasePipelineArgs(args, &stdout, &stderr)

		if !done || code != 10 || !strings.Contains(stderr.String(), "non-empty reason") {
			t.Errorf("args %v: code=%d done=%v stderr=%s", args, code, done, stderr.String())
		}
	}
}

func TestParseReleasePipelineArgs_EachSwitchSetsItsFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer

	f, code, done := parseReleasePipelineArgs([]string{"--dry-run", "--no-rollback", "--skip-tests", "--strict-pass", "--require-preflight", "--max-poll-wait-s", "7", "--from-tag", "v1", "1.2.3"}, &stdout, &stderr)

	if done || code != 0 || !f.dryRun || !f.noRollback || !f.skipTests || !f.strictPass || !f.requirePreflight || f.maxPollWaitS != 7 || f.fromTag != "v1" {
		t.Fatalf("got flags=%+v code=%d done=%v", f, code, done)
	}
}
