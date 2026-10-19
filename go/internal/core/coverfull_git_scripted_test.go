package core

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type scriptedReply struct {
	stdout string
	exit   int
	err    error
}

func useScriptedGit(t *testing.T, reply func(call int, args []string) scriptedReply) {
	t.Helper()
	orig := gitRunner
	n := 0
	gitRunner = sysexec.RunFunc(func(_ context.Context, _, _ string, args, _ []string, _ io.Reader, out, _ io.Writer) (int, error) {
		n++
		r := reply(n, args)
		if out != nil && r.stdout != "" {
			_, _ = out.Write([]byte(r.stdout))
		}
		return r.exit, r.err
	})
	t.Cleanup(func() { gitRunner = orig })
}

func hasArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

func TestCarryforward_GitFailureModes(t *testing.T) {
	ctx := context.Background()
	boom := io.ErrUnexpectedEOF
	notSuperseded := func(args []string) (scriptedReply, bool) {
		switch {
		case hasArg(args, "--is-ancestor"):
			return scriptedReply{exit: 1}, true
		case hasArg(args, "cherry"):
			return scriptedReply{stdout: "+ abc\n"}, true
		}
		return scriptedReply{}, false
	}
	t.Run("merge-tree transport error", func(t *testing.T) {
		useScriptedGit(t, func(_ int, args []string) scriptedReply {
			if r, ok := notSuperseded(args); ok {
				return r
			}
			return scriptedReply{err: boom}
		})
		if _, err := CarryforwardCandidateLandable(ctx, "/d", "ref", "main"); err == nil {
			t.Fatal("want merge-tree error")
		}
	})
	t.Run("merge-tree unexpected exit", func(t *testing.T) {
		useScriptedGit(t, func(_ int, args []string) scriptedReply {
			if r, ok := notSuperseded(args); ok {
				return r
			}
			return scriptedReply{exit: 2}
		})
		if _, err := CarryforwardCandidateLandable(ctx, "/d", "ref", "main"); err == nil || !strings.Contains(err.Error(), "exited 2") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("is-ancestor transport error", func(t *testing.T) {
		useScriptedGit(t, func(int, []string) scriptedReply { return scriptedReply{err: boom} })
		if _, err := refSuperseded(ctx, "/d", "ref", "main"); err == nil {
			t.Fatal("want is-ancestor error")
		}
	})
	t.Run("cherry transport error", func(t *testing.T) {
		useScriptedGit(t, func(int, []string) scriptedReply { return scriptedReply{err: boom} })
		if _, err := commitsFullyLanded(ctx, "/d", "ref", "main"); err == nil {
			t.Fatal("want cherry error")
		}
	})
	t.Run("classify: landable check fails", func(t *testing.T) {
		useScriptedGit(t, func(int, []string) scriptedReply { return scriptedReply{err: boom} })
		if v, err := ClassifyFleetRebaseCandidate(ctx, "/d", "ref", "main"); err == nil || v != FleetRebaseAlreadyLanded {
			t.Fatalf("verdict=%v err=%v", v, err)
		}
	})
	t.Run("classify: second supersede check fails", func(t *testing.T) {
		ancestorCalls := 0
		useScriptedGit(t, func(_ int, args []string) scriptedReply {
			switch {
			case hasArg(args, "--is-ancestor"):
				ancestorCalls++
				if ancestorCalls > 1 {
					return scriptedReply{err: boom}
				}
				return scriptedReply{exit: 1}
			case hasArg(args, "cherry"):
				return scriptedReply{stdout: "+ abc\n"}
			}
			return scriptedReply{exit: 1}
		})
		if v, err := ClassifyFleetRebaseCandidate(ctx, "/d", "ref", "main"); err == nil || v != FleetRebaseAlreadyLanded {
			t.Fatalf("verdict=%v err=%v", v, err)
		}
	})
}

func TestAdvanceContinuationBase_UnresolvableMain(t *testing.T) {
	useScriptedGit(t, func(int, []string) scriptedReply { return scriptedReply{exit: 128} })
	if _, err := advanceContinuationBase(context.Background(), "/wt", 1); err == nil || !strings.Contains(err.Error(), "base-advance resolve main") {
		t.Fatalf("err = %v", err)
	}
}

func TestSnapshotPreservedWorktree_GitStepFailures(t *testing.T) {
	failAt := func(step string) func(int, []string) scriptedReply {
		return func(_ int, args []string) scriptedReply {
			if args[0] == "status" {
				return scriptedReply{stdout: " M f\n"}
			}
			if hasArg(args, step) {
				return scriptedReply{exit: 1}
			}
			return scriptedReply{stdout: "deadbeef\n"}
		}
	}
	for _, step := range []string{"add", "commit", "rev-parse"} {
		t.Run(step, func(t *testing.T) {
			useScriptedGit(t, failAt(step))
			_, err := snapshotPreservedWorktree(context.Background(), "/p", "/w")
			if err == nil || !strings.Contains(err.Error(), "snapshot: "+step) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestValidateContinuation_Rejections(t *testing.T) {
	ctx := context.Background()
	if err := validateContinuation(ctx, "/p", nil); err == nil {
		t.Fatal("nil binding must be rejected")
	}
	if err := validateContinuation(ctx, "/p", &continuation.Continuation{}); err == nil {
		t.Fatal("empty snapshot must be rejected")
	}
	t.Run("base is not an ancestor", func(t *testing.T) {
		useScriptedGit(t, func(_ int, args []string) scriptedReply {
			if hasArg(args, "--is-ancestor") {
				return scriptedReply{exit: 1}
			}
			return scriptedReply{}
		})
		err := validateContinuation(ctx, "/p", &continuation.Continuation{SnapshotSHA: "abc", BaseSHA: "def"})
		if err == nil || !strings.Contains(err.Error(), "not an ancestor") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("re-screen fails", func(t *testing.T) {
		useScriptedGit(t, func(_ int, args []string) scriptedReply {
			if hasArg(args, "--is-ancestor") {
				return scriptedReply{err: io.ErrUnexpectedEOF}
			}
			return scriptedReply{}
		})
		err := validateContinuation(ctx, "/p", &continuation.Continuation{SnapshotSHA: "abcdefabcdef"})
		if err == nil || !strings.Contains(err.Error(), "re-screen") {
			t.Fatalf("err = %v", err)
		}
	})
}

func corruptRegistry(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFileAt(t, continuation.RegistryPath(root), "{broken")
	return root
}

func TestRegisterLaneScopeBinding_RegistryFailureContinues(t *testing.T) {
	t.Parallel()
	root := corruptRegistry(t)
	ws := t.TempDir()
	body, err := json.Marshal(LaneScope{TodoIDs: []string{"", "  ", "a", "b"}})
	if err != nil {
		t.Fatal(err)
	}
	writeFileAt(t, filepath.Join(ws, LaneScopeFile), string(body))
	registerLaneScopeBinding(ws, root, continuation.Continuation{SnapshotSHA: "abcdefabcdef"}, 7)
	raw, _ := os.ReadFile(continuation.RegistryPath(root))
	if string(raw) != "{broken" {
		t.Fatalf("registry must stay untouched on failure, got %q", raw)
	}
}

func TestReleaseDeclinedBinding_SkipsBlankIDsAndSurvivesRegistryFailure(t *testing.T) {
	t.Parallel()
	releaseDeclinedBinding(corruptRegistry(t), []string{"", "a"}, &continuation.Continuation{Cycle: 3})
}

func TestReverifyResolution_EmptyDiffsError(t *testing.T) {
	t.Parallel()
	diff := []byte("diff --git a/f b/f\n--- a/f\n+++ b/f\n@@ -1 +1 @@\n-a\n+b\n")
	if _, err := ReverifyResolution(nil, diff); err == nil {
		t.Fatal("empty audited diff must error")
	}
	if _, err := ReverifyResolution(diff, nil); err == nil {
		t.Fatal("empty resolved diff must error")
	}
}

func TestConsoleLease_PathAndAdoption(t *testing.T) {
	t.Parallel()
	if got := consoleLeasePath(t.TempDir()); got != "" {
		t.Fatalf("non-repo lease path = %q", got)
	}
	root := initTestRepo(t)
	path := consoleLeasePath(root)
	if path == "" {
		t.Fatal("repo must resolve a lease path")
	}
	now := time.Now()
	lease := consoleLease{Paths: []string{"a.go", ""}, ExpiresAt: now.Add(time.Hour).Format(time.RFC3339)}
	body, _ := json.Marshal(lease)
	writeFileAt(t, path, string(body))
	var warn strings.Builder
	got := adoptConsoleLease(root, now, &warn)
	if !got["a.go"] || len(got) != 1 || !strings.Contains(warn.String(), "console-lease adopted") {
		t.Fatalf("leased=%v warn=%q", got, warn.String())
	}
}
