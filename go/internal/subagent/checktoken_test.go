package subagent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckToken(t *testing.T) {
	tmp := t.TempDir()
	good := filepath.Join(tmp, "good.md")
	if err := os.WriteFile(good, []byte("first line\n<!-- challenge-token: abc123def4567890 -->\nbody\n"), 0o644); err != nil {
		t.Fatalf("seed good artifact: %v", err)
	}
	noToken := filepath.Join(tmp, "no-token.md")
	if err := os.WriteFile(noToken, []byte("first line\nbody\n"), 0o644); err != nil {
		t.Fatalf("seed no-token artifact: %v", err)
	}

	tests := []struct {
		name       string
		artifact   string
		token      string
		wantOK     bool
		wantSubstr string
	}{
		{"token present", good, "abc123def4567890", true, "OK:"},
		{"token absent", noToken, "abc123def4567890", false, "token absent"},
		{"artifact missing", filepath.Join(tmp, "nope.md"), "x", false, "artifact missing"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CheckToken(tc.artifact, tc.token)
			if got.OK != tc.wantOK {
				t.Errorf("OK=%v, want %v", got.OK, tc.wantOK)
			}
			if !strings.Contains(got.Reason, tc.wantSubstr) {
				t.Errorf("Reason=%q, want substring %q", got.Reason, tc.wantSubstr)
			}
		})
	}
}

func TestCheckToken_UnreadablePathOnPermissionError(t *testing.T) {
	// On *nix we can mask read permission to drive the non-IsNotExist branch.
	// Skip on platforms where this doesn't work (e.g., root).
	if os.Geteuid() == 0 {
		t.Skip("running as root, cannot mask read permission")
	}
	tmp := t.TempDir()
	target := filepath.Join(tmp, "sealed.md")
	if err := os.WriteFile(target, []byte("payload"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := os.Chmod(target, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(target, 0o600) })
	res := CheckToken(target, "anything")
	if res.OK {
		t.Fatalf("expected failure on permission-denied")
	}
	// Either "unreadable" (read error) or "missing" depending on platform.
	if !strings.Contains(res.Reason, "unreadable") && !strings.Contains(res.Reason, "missing") {
		t.Errorf("unexpected reason: %q", res.Reason)
	}
}
