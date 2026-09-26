package bridge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// writeSkillDir writes skills/<name>/SKILL.md under a fresh root and returns the root, not the skill dir.
func writeSkillDir(t *testing.T, name, body string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "skills", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestInjectSkillOverlays_NoopPaths(t *testing.T) {
	if got := injectSkillOverlays("BODY", core.BridgeRequest{ProjectRoot: "/x"}); got != "BODY" {
		t.Errorf("no Skills must pass through, got %q", got)
	}
	if got := injectSkillOverlays("BODY", core.BridgeRequest{Skills: []string{"fable"}}); got != "BODY" {
		t.Errorf("no ProjectRoot must pass through, got %q", got)
	}
}

func TestInjectSkillOverlays_PrependsPersonaAboveBody(t *testing.T) {
	root := writeSkillDir(t, "fable", "---\nname: fable\n---\n\nEVIDENCE before opinion.\n")
	got := injectSkillOverlays("BODY", core.BridgeRequest{ProjectRoot: root, Skills: []string{"fable"}, Agent: "build"})
	si := strings.Index(got, "EVIDENCE before opinion.")
	bi := strings.Index(got, "BODY")
	if si < 0 {
		t.Fatalf("persona not injected:\n%s", got)
	}
	if si > bi {
		t.Errorf("skill overlay must precede the body; skill=%d body=%d", si, bi)
	}
}

func TestLaunch_InjectsSkillOverlay(t *testing.T) {
	root := writeSkillDir(t, "fable", "---\nname: fable\n---\n\nATTACK YOUR OWN PREMISES.\n")
	fe := &fakeEngine{}
	_, err := withEngine(fe).Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-tmux", Profile: "/p", Prompt: "TASK-BODY",
		Workspace: t.TempDir(), ArtifactPath: "/a.md", Agent: "build",
		ProjectRoot: root, Skills: []string{"fable"},
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	got := fe.gotReq.Prompt
	si := strings.Index(got, "ATTACK YOUR OWN PREMISES.")
	bi := strings.Index(got, "TASK-BODY")
	if si < 0 {
		t.Fatalf("skill overlay missing from launched prompt:\n%s", truncate(got, 400))
	}
	if si >= bi {
		t.Errorf("skill overlay must precede the task body; skill=%d body=%d", si, bi)
	}
}

func TestLaunch_NoSkills_ByteIdenticalDefault(t *testing.T) {
	fe := &fakeEngine{}
	_, err := withEngine(fe).Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-tmux", Profile: "/p", Prompt: "TASK-BODY",
		Workspace: t.TempDir(), ArtifactPath: "/a.md", Agent: "build",
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	if strings.Contains(fe.gotReq.Prompt, "PRELOADED SKILL") {
		t.Errorf("no Skills must produce no overlay block:\n%s", truncate(fe.gotReq.Prompt, 300))
	}
}

func TestLaunch_MissingSkill_StillDispatches(t *testing.T) {
	root := t.TempDir()
	fe := &fakeEngine{}
	_, err := withEngine(fe).Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-tmux", Profile: "/p", Prompt: "TASK-BODY",
		Workspace: t.TempDir(), ArtifactPath: "/a.md", Agent: "build",
		ProjectRoot: root, Skills: []string{"absent"},
	})
	if err != nil {
		t.Fatalf("missing skill must not fail the launch: %v", err)
	}
	if !strings.Contains(fe.gotReq.Prompt, "TASK-BODY") {
		t.Errorf("launch must proceed with the body despite the missing skill:\n%s", truncate(fe.gotReq.Prompt, 300))
	}
}
