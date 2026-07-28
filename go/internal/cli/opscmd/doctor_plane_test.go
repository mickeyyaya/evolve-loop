package opscmd

// doctor_plane_test.go — ADR-0080 S2 CLI surface: the plane report and the
// exit-2 WARN when loop state lives in the primary (console) checkout.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func planePrimaryRoot(t *testing.T, withState bool) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if withState {
		if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".evolve", "state.json"), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestRunDoctorPlane_PrimaryWithLoopStateWarnsExit2(t *testing.T) {
	root := planePrimaryRoot(t, true)
	var out, errb bytes.Buffer
	rc := runDoctorPlane([]string{root}, &out, &errb)
	if rc != 2 {
		t.Fatalf("rc = %d, want 2 (WARN): %s%s", rc, out.String(), errb.String())
	}
	if !strings.Contains(errb.String(), "ADR-0080") || !strings.Contains(out.String(), "PRIMARY") {
		t.Errorf("WARN goes to stderr with the ADR; the report names the plane on stdout: out=%s err=%s", out.String(), errb.String())
	}
}

func TestRunDoctorPlane_PrimaryWithoutStateIsClean(t *testing.T) {
	root := planePrimaryRoot(t, false)
	var out, errb bytes.Buffer
	if rc := runDoctorPlane([]string{root}, &out, &errb); rc != 0 {
		t.Fatalf("rc = %d, want 0: %s%s", rc, out.String(), errb.String())
	}
	if !strings.Contains(out.String(), "PRIMARY checkout") {
		t.Errorf("report must state the plane: %s", out.String())
	}
}

func TestRunDoctorPlane_NotARepoErrors(t *testing.T) {
	var out, errb bytes.Buffer
	if rc := runDoctorPlane([]string{t.TempDir()}, &out, &errb); rc != 1 {
		t.Fatalf("rc = %d, want 1 on a non-repo", rc)
	}
}
