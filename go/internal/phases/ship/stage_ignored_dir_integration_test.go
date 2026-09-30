//go:build integration

package ship

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShipFromWorktree_DropsGitignoredDirectoryPathspec(t *testing.T) {
	repo, wt := makeWorktreeScenario(t)
	runGit(t, wt, "reset", "HEAD", "wt-change.txt")

	// The live shape: dir-form rule, content inside, the DIRECTORY declared.
	mustWrite(t, filepath.Join(wt, ".gitignore"), ".evolve/inbox/processed/\n")
	runGit(t, wt, "add", ".gitignore")
	procRel := ".evolve/inbox/processed"
	mustWrite(t, filepath.Join(wt, filepath.FromSlash(procRel), "item.json"), "{}\n")

	ws := t.TempDir()
	mustWrite(t, filepath.Join(ws, "build-report.md"),
		"# Build Report\n\n## Files Changed\n\n- `wt-change.txt`\n- `"+procRel+"`\n")

	opts := &Options{
		Class:         ClassCycle,
		CommitMessage: "feat: inbox-lifecycle cycle ships despite gitignored processed dir",
		ProjectRoot:   repo,
		PluginRoot:    repo,
		WorkspacePath: ws,
		Stdout:        io.Discard,
		Stderr:        io.Discard,
	}
	res := &RunResult{}
	if err := shipFromWorktree(context.Background(), opts, res, "main", wt); err != nil {
		t.Fatalf("shipFromWorktree with gitignored declared DIRECTORY: %v (2026-08-14 ship|unknown|99c38818 class)", err)
	}

	files := commitFileList(t, wt, "cycle-1")
	if !strings.Contains(files, "wt-change.txt") {
		t.Errorf("declared source path absent from the ship commit; files=%q", files)
	}
	if strings.Contains(files, "processed") {
		t.Errorf("gitignored directory content rode into the ship commit; files=%q", files)
	}
	if _, err := os.Stat(filepath.Join(wt, filepath.FromSlash(procRel), "item.json")); err != nil {
		t.Errorf("runtime inbox state must survive being excluded from staging: %v", err)
	}
	// NOTE: in THIS fixture shape the manifest∩changed intersection already
	// excludes the ignored dir before add (ignored content never reaches
	// porcelain), so no refusal fires — the retry seam's loud-log contract is
	// pinned by the unit half (TestShipDirect_CycleClass_Retries…). This
	// real-git test pins the OUTCOME: the scenario ships, the runtime state
	// survives, nothing ignored lands.
}
