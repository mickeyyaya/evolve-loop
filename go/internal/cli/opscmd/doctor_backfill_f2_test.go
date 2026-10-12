package opscmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageprobe"
)

func restoreBootTempF2(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { doctorBootWorktreeTemp = os.MkdirTemp })
}

func TestDoctorBootConfig_BuildsThePlainAndTheSandboxedConfigF2(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	cwd, _ := os.Getwd()
	var errb bytes.Buffer

	plain, plainCleanup, code, ok := doctorBootConfig(false, "/ws", &errb)

	if !ok || code != 0 || plain.Workspace != "/ws" || plain.ProjectRoot != cwd || plain.Worktree != "" || plain.Agent != "" {
		t.Errorf("plain config = %+v code=%d ok=%v, want only the workspace and project root", plain, code, ok)
	}
	plainCleanup()

	sandbox, cleanup, code, ok := doctorBootConfig(true, "/ws", &errb)

	if !ok || code != 0 || sandbox.Agent != "build" || sandbox.Worktree == "" || sandbox.Workspace != "/ws" {
		t.Fatalf("sandbox config = %+v code=%d ok=%v, want a worktree and the build agent", sandbox, code, ok)
	}
	if info, err := os.Stat(sandbox.Worktree); err != nil || !info.IsDir() {
		t.Fatalf("sandbox worktree %s is not a directory (%v)", sandbox.Worktree, err)
	}
	cleanup()
	if _, err := os.Stat(sandbox.Worktree); !os.IsNotExist(err) {
		t.Errorf("cleanup left the worktree (%v)", err)
	}
}

func TestDoctorBootConfig_ReportsAWorktreeItCannotCreateF2(t *testing.T) {
	restoreBootTempF2(t)
	doctorBootWorktreeTemp = func(string, string) (string, error) { return "", errors.New("temp boom") }
	var errb bytes.Buffer

	cfg, cleanup, code, ok := doctorBootConfig(true, "/ws", &errb)

	if cfg != nil || ok || code != 1 || cleanup == nil || !strings.Contains(errb.String(), "evolve doctor boot: temp worktree: temp boom") {
		t.Errorf("cfg=%+v ok=%v code=%d err=%q, want no config, exit 1 and the temp worktree error", cfg, ok, code, errb.String())
	}
	cleanup()
}

func TestRunDoctorBoot_RefusesBadFlagsAndHostFailuresBeforeBootingF2(t *testing.T) {
	t.Run("unknown flag", func(t *testing.T) {
		var out, errb bytes.Buffer

		rc := runDoctorBoot([]string{"--bogus", "claude-tmux"}, &out, &errb)

		if rc != 10 || !strings.Contains(errb.String(), "flag provided but not defined") {
			t.Errorf("rc=%d err=%q, want exit 10", rc, errb.String())
		}
	})
	t.Run("too many drivers", func(t *testing.T) {
		var out, errb bytes.Buffer

		rc := runDoctorBoot([]string{"a-tmux", "b-tmux"}, &out, &errb)

		if rc != 10 || !strings.Contains(errb.String(), "usage: evolve doctor boot <driver>") {
			t.Errorf("rc=%d err=%q, want the usage error", rc, errb.String())
		}
	})
	t.Run("temp workspace cannot be created", func(t *testing.T) {
		t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "gone"))
		var out, errb bytes.Buffer

		rc := runDoctorBoot([]string{"claude-tmux"}, &out, &errb)

		if rc != 1 || out.Len() != 0 || !strings.Contains(errb.String(), "evolve doctor boot: temp workspace:") {
			t.Errorf("rc=%d out=%q err=%q, want exit 1 before any boot", rc, out.String(), errb.String())
		}
	})
	t.Run("sandbox worktree cannot be created", func(t *testing.T) {
		restoreBootTempF2(t)
		doctorBootWorktreeTemp = func(string, string) (string, error) { return "", errors.New("temp boom") }
		var out, errb bytes.Buffer

		rc := runDoctorBoot([]string{"claude-tmux", "--sandbox"}, &out, &errb)

		if rc != 1 || out.Len() != 0 || !strings.Contains(errb.String(), "temp worktree: temp boom") {
			t.Errorf("rc=%d out=%q err=%q, want exit 1 before any boot", rc, out.String(), errb.String())
		}
	})
}

func TestProductionDoctorUsageEvidence_BuildsAnExplainerOverATempWorkspaceThatCleanupRemovesF2(t *testing.T) {
	cases := map[string]func(t *testing.T, root string){
		"no policy file": func(*testing.T, string) {},
		"a valid policy file": func(t *testing.T, root string) {
			writeLeaseFileF2(t, filepath.Join(root, ".evolve", "policy.json"), "{}")
		},
		"a broken policy file": func(t *testing.T, root string) {
			writeLeaseFileF2(t, filepath.Join(root, ".evolve", "policy.json"), "{not json")
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			tmp := t.TempDir()
			t.Setenv("TMPDIR", tmp)
			root := t.TempDir()
			setup(t, root)
			var errb bytes.Buffer

			explain, cleanup := productionDoctorUsageEvidence(root, bridge.Deps{Stderr: &errb})

			if explain == nil || cleanup == nil {
				t.Fatalf("explain=%v cleanup=%v, want both", explain != nil, cleanup != nil)
			}
			if entries, _ := os.ReadDir(tmp); len(entries) != 1 || !strings.HasPrefix(entries[0].Name(), "evolve-doctor-usage-") {
				t.Fatalf("temp dir holds %v, want one usage workspace", entries)
			}
			ev := explain(context.Background(), "ollama-tmux", time.Now())
			if ev.Verdict == "" {
				t.Errorf("evidence %+v has no verdict", ev)
			}
			cleanup()
			if entries, _ := os.ReadDir(tmp); len(entries) != 0 {
				t.Errorf("cleanup left %v", entries)
			}
		})
	}
}

func TestProductionDoctorUsageEvidence_AWorkspaceItCannotCreateYieldsAnUnavailableVerdictF2(t *testing.T) {
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "gone"))

	explain, cleanup := productionDoctorUsageEvidence(t.TempDir(), bridge.Deps{})

	ev := explain(context.Background(), "claude-tmux", time.Now())
	if ev.Family != "claude-tmux" || ev.Verdict != usageprobe.VerdictUnavailable || !strings.HasPrefix(ev.Detail, "usage workspace: ") {
		t.Errorf("evidence = %+v, want unavailable for the driver naming the workspace failure", ev)
	}
	cleanup()
}

func TestRunDoctorLiveWith_ATempWorkspaceItCannotCreateExitsOneBeforeAnyLaunchF2(t *testing.T) {
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "gone"))
	var out, errb bytes.Buffer

	rc := runDoctorLiveWith([]string{"claude-tmux"}, &out, bridge.Deps{Stderr: &errb})

	if rc != 1 || out.Len() != 0 || !strings.Contains(errb.String(), "evolve doctor live: temp workspace:") {
		t.Errorf("rc=%d out=%q err=%q, want exit 1 naming the workspace", rc, out.String(), errb.String())
	}
}

func TestRunDoctorLiveWith_RefusesBadArgumentsF2(t *testing.T) {
	cases := map[string][]string{
		"unknown flag": {"--bogus", "claude-tmux"},
		"no driver":    {},
		"two drivers":  {"a-tmux", "b-tmux"},
	}
	for name, args := range cases {
		var out, errb bytes.Buffer

		rc := runDoctorLiveWith(args, &out, bridge.Deps{Stderr: &errb})

		if rc != 10 || out.Len() != 0 {
			t.Errorf("%s: rc=%d out=%q err=%q, want exit 10", name, rc, out.String(), errb.String())
		}
	}
}
