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
	// Will fail at preflight or full-dry-run step (no real repo); we just
	// verify it's > 0 (some pre-publish error) and doesn't hit exit 10.
	rc := RunReleasePipeline([]string{"99.99.99", "--dry-run", "--require-preflight"}, nil, &stdout, &stderr)
	if rc == 0 || rc == 10 {
		t.Errorf("rc = %d, want pre-publish or post-publish error (1/2/3)", rc)
	}
}
