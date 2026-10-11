package audit

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestEGPSRedMessage_KeepsAnIDThatIsOnlyCycleTokens(t *testing.T) {
	cases := []struct {
		name string
		ids  []string
		want string
	}{
		{"bare Test prefix", []string{"Test"}, "[Test]"},
		{"cycle dir and Test only", []string{"cycle12/Test"}, "[cycle12/Test]"},
		{"cycle dir, Test and cycle tag only", []string{"cycle7/TestC7_"}, "[cycle7/TestC7_]"},
		{"named test is stripped", []string{"cycle7/TestC7_Builds"}, "[Builds]"},
		{"mixed", []string{"Test", "cycle3/Test3_Shape"}, "[Test Shape]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := egpsRedMessage(len(tc.ids), tc.ids, nil)

			if !strings.Contains(got, tc.want) {
				t.Errorf("egpsRedMessage(%q) = %q, want it to name %s", tc.ids, got, tc.want)
			}
		})
	}
}

func TestEGPSRedMessage_NamesAtMostFiveReds(t *testing.T) {
	cases := []struct {
		n        int
		wantMore string
	}{
		{5, ""},
		{6, " +1 more"},
		{9, " +4 more"},
	}
	for _, tc := range cases {
		ids := make([]string, tc.n)
		for i := range ids {
			ids[i] = "TestR" + string(rune('a'+i))
		}

		got := egpsRedMessage(tc.n, ids, nil)

		want := "EGPS: red_count=" + strconv.Itoa(tc.n) + " [Ra Rb Rc Rd Re" + tc.wantMore + "] (cycle ships only when red_count==0)"
		if got != want {
			t.Errorf("n=%d: got %q, want %q", tc.n, got, want)
		}
	}
}

func TestQuarantineAuditProbes_GitStatusFailureDegradesOpenLoudly(t *testing.T) {
	notARepo := t.TempDir()
	probe := filepath.Join(notARepo, "zz_probe_test.go")
	if err := os.WriteFile(probe, []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer

	moved, err := quarantineAuditProbes(notARepo, t.TempDir(), time.Time{}, &log)

	if err != nil || moved != nil {
		t.Fatalf("= (%v, %v), want (nil, nil): a failed scan degrades open", moved, err)
	}
	if !strings.Contains(log.String(), "[audit] probe quarantine degraded OPEN: git status failed") {
		t.Errorf("log = %q, want the degraded-open notice", log.String())
	}
	if _, statErr := os.Stat(probe); statErr != nil {
		t.Errorf("probe stat = %v, want it untouched", statErr)
	}
}

func TestQuarantineAuditProbes_PreserveFailureStopsAndKeepsTheProbe(t *testing.T) {
	worktree, _, cutoff := probeRepo(t)
	workspace := filepath.Join(t.TempDir(), "workspace-file")
	if err := os.WriteFile(workspace, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	moved, err := quarantineAuditProbes(worktree, workspace, cutoff, &bytes.Buffer{})

	if err == nil || !strings.HasPrefix(err.Error(), "probe quarantine: go/internal/bridge/zz_audit_probe_test.go:") {
		t.Fatalf("err = %v, want the preserve failure naming the probe", err)
	}
	if len(moved) != 0 {
		t.Errorf("moved = %v, want none", moved)
	}
	if _, statErr := os.Stat(filepath.Join(worktree, "go", "internal", "bridge", "zz_audit_probe_test.go")); statErr != nil {
		t.Errorf("probe stat = %v, want it kept when it could not be preserved", statErr)
	}
}

func TestNewTestFilePath(t *testing.T) {
	cases := []struct {
		line   string
		want   string
		wantOK bool
	}{
		{"", "", false},
		{"?? ", "", false},
		{"??x", "", false},
		{"?? a_test.go", "a_test.go", true},
		{"A  go/x/b_test.go", "go/x/b_test.go", true},
		{"AM go/x/c_test.go", "go/x/c_test.go", true},
		{" A go/x/h_test.go", "go/x/h_test.go", true},
		{" M go/x/d_test.go", "", false},
		{"?? go/x/", "", false},
		{"?? go/x/e.go", "", false},
		{"?? go/acs/cycle9/f_test.go", "", false},
		{"?? go/internal/acs/g_test.go", "go/internal/acs/g_test.go", true},
		{"?? go/x/é_test.go", "go/x/é_test.go", true},
	}
	for _, tc := range cases {
		t.Run(tc.line, func(t *testing.T) {
			got, ok := newTestFilePath(tc.line)

			if got != tc.want || ok != tc.wantOK {
				t.Errorf("newTestFilePath(%q) = (%q, %v), want (%q, %v)", tc.line, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

func FuzzNewTestFilePath_OnlyNewGoTestFilesOutsideACS(f *testing.F) {
	f.Add("?? a_test.go")
	f.Add("A  go/acs/x_test.go")
	f.Add("?? ")
	f.Fuzz(func(t *testing.T, line string) {
		p, ok := newTestFilePath(line)
		if !ok {
			if p != "" {
				t.Fatalf("rejected %q but returned %q", line, p)
			}
			return
		}
		if !strings.HasSuffix(p, "_test.go") || strings.HasPrefix(p, "go/acs/") {
			t.Fatalf("accepted %q as %q", line, p)
		}
		if code := line[:2]; code != "??" && !strings.Contains(code, "A") {
			t.Fatalf("accepted status %q", code)
		}
	})
}

func TestWorktreeFailureOffenders(t *testing.T) {
	cases := []struct {
		name   string
		code   int
		report string
		want   []string
	}{
		{"drift report names its artifacts", 1, "DRIFT: skills/a/SKILL.md: stale\nMANIFEST: missing x\n", []string{"skills/a/SKILL.md", "MANIFEST: missing x"}},
		{"a single drift line is enough", 1, "DRIFT: skills/b/SKILL.md\n", []string{"skills/b/SKILL.md"}},
		{"plain failure keeps the output", 2, "  boom  \n", []string{"the worktree generator skills check exited 2 without a drift report: boom"}},
		{"empty output says so", 3, " \n\t", []string{"the worktree generator skills check exited 3 without a drift report: (no output)"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := worktreeFailureOffenders(tc.code, tc.report); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("worktreeFailureOffenders = %q, want %q", got, tc.want)
			}
		})
	}
}
