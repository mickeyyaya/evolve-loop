package bedrock

import (
	"errors"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestEmit_MissingRole(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		role string
	}{
		{"empty", ""},
		{"whitespace", "   "},
		{"tab", "\t"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := Emit(tc.role)
			if !errors.Is(err, ErrMissingRole) {
				t.Fatalf("want ErrMissingRole, got %v", err)
			}
		})
	}
}

func TestEmit_CommonBedrockAlwaysPresent(t *testing.T) {
	t.Parallel()
	roles := []string{"scout", "builder", "auditor", "retrospective", "unknown-role", "tester"}
	for _, r := range roles {
		r := r
		t.Run(r, func(t *testing.T) {
			t.Parallel()
			out, err := Emit(r)
			if err != nil {
				t.Fatalf("Emit(%q) err: %v", r, err)
			}
			if !strings.HasPrefix(out, "# EVOLVE-LOOP SUBAGENT INVOCATION") {
				t.Errorf("missing common bedrock header: %q...", out[:min(80, len(out))])
			}
			if !strings.Contains(out, "challenge-token") {
				t.Errorf("missing challenge-token reminder for %s", r)
			}
		})
	}
}

func TestEmit_RoleExtensions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		role, marker string
	}{
		{"auditor", "Adversarial Audit Mode"},
		{"builder", "Builder operating notes"},
		{"scout", "Scout operating notes"},
		{"retrospective", "Retrospective operating notes"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.role, func(t *testing.T) {
			t.Parallel()
			out, err := Emit(tc.role)
			if err != nil {
				t.Fatalf("Emit err: %v", err)
			}
			if !strings.Contains(out, tc.marker) {
				t.Errorf("missing %q in %s bedrock", tc.marker, tc.role)
			}
		})
	}
}

func TestEmit_UnknownRoleOnlyEmitsCommon(t *testing.T) {
	t.Parallel()
	out, err := Emit("future-role-2030")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	for _, marker := range []string{
		"Adversarial Audit Mode",
		"Builder operating notes",
		"Scout operating notes",
		"Retrospective operating notes",
	} {
		if strings.Contains(out, marker) {
			t.Errorf("unknown role should not include %q", marker)
		}
	}
}

func TestEmit_Deterministic(t *testing.T) {
	t.Parallel()
	roles := []string{"scout", "builder", "auditor", "retrospective", "tester", "intent"}
	for _, r := range roles {
		r := r
		t.Run(r, func(t *testing.T) {
			t.Parallel()
			a, _ := Emit(r)
			b, _ := Emit(r)
			if a != b {
				t.Fatalf("Emit(%q) is not deterministic", r)
			}
		})
	}
}

func TestRoles(t *testing.T) {
	t.Parallel()
	got := Roles()
	want := map[string]bool{"auditor": true, "builder": true, "scout": true, "retrospective": true}
	if len(got) != len(want) {
		t.Fatalf("Roles() len = %d, want %d", len(got), len(want))
	}
	for _, r := range got {
		if !want[r] {
			t.Errorf("unexpected role %q in Roles()", r)
		}
	}
}

// TestEmit_ParityWithBash verifies byte-identical output vs. the bash original
// for every known role. This is the prompt-cache contract: any divergence
// would silently invalidate cache reuse and inflate cost. Skipped when bash
// is unavailable (e.g. some CI containers).
func TestEmit_ParityWithBash(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash parity test skipped on windows")
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not available")
	}
	repoRoot := findRepoRoot(t)
	script := filepath.Join(repoRoot, "legacy", "scripts", "dispatch", "build-invocation-context.sh")
	if !fileExists(script) {
		t.Skipf("script not found: %s", script)
	}

	roles := []string{"scout", "builder", "auditor", "retrospective", "tester", "intent", "triage"}
	for _, r := range roles {
		r := r
		t.Run(r, func(t *testing.T) {
			out, err := exec.Command(bash, script, r).Output()
			if err != nil {
				t.Fatalf("bash err: %v", err)
			}
			want := string(out)
			got, gerr := Emit(r)
			if gerr != nil {
				t.Fatalf("Emit err: %v", gerr)
			}
			if got != want {
				t.Errorf("parity mismatch for role %s\nWANT:\n%s\nGOT:\n%s", r, want, got)
			}
		})
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Fatalf("git rev-parse: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func fileExists(p string) bool {
	_, err := exec.Command("test", "-f", p).CombinedOutput()
	return err == nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
