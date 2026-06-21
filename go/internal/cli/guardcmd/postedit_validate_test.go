package guardcmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCmd_PostEditValidate_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := RunPostEditValidate([]string{"--help"}, nil, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc = %d, want 0", rc)
	}
	if !strings.Contains(stdout.String(), "postedit-validate") {
		t.Errorf("help missing keyword: %s", stdout.String())
	}
}

func TestCmd_PostEditValidate_NoPayload(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", t.TempDir())
	var stdout, stderr bytes.Buffer
	// Empty stdin — should succeed (skip).
	rc := RunPostEditValidate(nil, bytes.NewReader(nil), &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc = %d, want 0", rc)
	}
}

func TestCmd_PostEditValidate_UnknownFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	// Even unknown flags exit 0 — PostToolUse can't block.
	rc := RunPostEditValidate([]string{"--bogus"}, bytes.NewReader(nil), &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc = %d, want 0", rc)
	}
}

func TestCmd_PostEditValidate_ValidJSON(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "ok.json")
	if err := os.WriteFile(p, []byte(`{"a":1}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", d)
	payload := []byte(`{"tool_input":{"file_path":"` + p + `"}}`)
	var stdout, stderr bytes.Buffer
	rc := RunPostEditValidate(nil, bytes.NewReader(payload), &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc = %d, want 0", rc)
	}
	if strings.Contains(stderr.String(), "WARN") {
		t.Errorf("stderr unexpectedly WARN'd: %s", stderr.String())
	}
}

func TestCmd_PostEditValidate_InvalidJSON(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "bad.json")
	if err := os.WriteFile(p, []byte(`{`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", d)
	payload := []byte(`{"tool_input":{"file_path":"` + p + `"}}`)
	var stdout, stderr bytes.Buffer
	rc := RunPostEditValidate(nil, bytes.NewReader(payload), &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc = %d, want 0 (PostToolUse cannot block)", rc)
	}
	if !strings.Contains(stderr.String(), "WARN") {
		t.Errorf("stderr missing WARN for invalid JSON: %s", stderr.String())
	}
}

func TestCmd_PostEditValidate_BypassFlag(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "bad.json")
	if err := os.WriteFile(p, []byte(`{`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", d)
	payload := []byte(`{"tool_input":{"file_path":"` + p + `"}}`)
	var stdout, stderr bytes.Buffer
	rc := RunPostEditValidate([]string{"--bypass"}, bytes.NewReader(payload), &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc = %d, want 0", rc)
	}
	if strings.Contains(stderr.String(), "WARN") {
		t.Errorf("bypass should suppress WARN: %s", stderr.String())
	}
}
