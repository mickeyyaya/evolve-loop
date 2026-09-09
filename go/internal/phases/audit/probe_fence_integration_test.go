//go:build integration

package audit

import (
	"context"
	"go/format"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

type probeFenceBridge struct {
	fakeBridge
	t      *testing.T
	mutate bool
}

func (b *probeFenceBridge) Launch(ctx context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	b.t.Helper()
	if b.mutate {
		if err := os.WriteFile(filepath.Join(req.Worktree, "go", "repair_test.go"), []byte("package fixture // auditor mutation\n"), 0o644); err != nil {
			b.t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(req.Worktree, "go", "probe_test.go"), []byte("package fixture\nimport \"testing\"\nfunc TestProbe(t *testing.T) { t.Fatal(\"auditor probe\") }\n"), 0o644); err != nil {
		b.t.Fatal(err)
	}
	return b.fakeBridge.Launch(ctx, req)
}

func TestAudit_ContentFencePreservesBuilderTestsBeforeHostExecution(t *testing.T) {
	for _, mutate := range []bool{false, true} {
		name := "repair_after_first_audit"
		if mutate {
			name = "restored_builder_test_has_new_mtime"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeGoPredFixture(t, root, 7, true)
			regression := filepath.Join(root, "go", "acs", "regression", "source")
			if err := os.MkdirAll(regression, 0o755); err != nil {
				t.Fatal(err)
			}
			grader, err := format.Source([]byte("package source\nimport (\"os\"; \"testing\")\nfunc TestBuilderRegressionIsDelivered(t *testing.T) { if _, err := os.Stat(\"../../../repair_test.go\"); err != nil { t.Fatal(err) } }\n"))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(regression, "source_test.go"), grader, 0o644); err != nil {
				t.Fatal(err)
			}
			gitInAudit(t, root, "init", "-q")
			if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".evolve/\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			gitInAudit(t, root, "add", "-A")
			gitInAudit(t, root, "commit", "-qm", "base")
			ws := filepath.Join(root, ".evolve", "runs", "cycle-7")
			if err := os.MkdirAll(ws, 0o755); err != nil {
				t.Fatal(err)
			}
			first := time.Now().Add(-time.Hour)
			firstDispatchAnchor(ws, first)
			path := filepath.Join(root, "go", "repair_test.go")
			want, err := format.Source([]byte("package fixture\nimport (\"os\"; \"testing\")\nfunc TestRepair(t *testing.T) { if _, err := os.Stat(\"repair_test.go\"); err != nil { t.Fatal(err) } }\n"))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, want, 0o644); err != nil {
				t.Fatal(err)
			}
			if mutate {
				// Even a pre-first-Audit Builder file must survive restoration,
				// which writes its authenticated bytes with a fresh mtime.
				if err := os.Chtimes(path, first.Add(-time.Hour), first.Add(-time.Hour)); err != nil {
					t.Fatal(err)
				}
			}
			gitInAudit(t, root, "add", "go/repair_test.go")
			if err := os.WriteFile(filepath.Join(ws, auditPromptArtifact), []byte("second audit"), 0o644); err != nil {
				t.Fatal(err)
			}
			phase := NewDefault(&probeFenceBridge{fakeBridge: fakeBridge{writeArtifact: "## Verdict\n**PASS**\n"}, t: t, mutate: mutate}, fakePromptsFS("body"))
			resp, err := phase.Run(context.Background(), core.PhaseRequest{
				Cycle: 7, RunID: "repair-7", AuditRound: 2,
				ProjectRoot: root, Worktree: root, Workspace: ws, WorktreeReadOnly: true,
			})
			if err != nil {
				t.Fatal(err)
			}
			got, readErr := os.ReadFile(path)
			if readErr != nil || string(got) != string(want) {
				t.Fatalf("host quarantine removed or changed Builder's staged regression: %v; got %q", readErr, got)
			}
			if _, err := os.Stat(filepath.Join(root, "go", "probe_test.go")); !os.IsNotExist(err) {
				t.Fatalf("auditor probe reached host execution: %v", err)
			}
			if resp.Verdict != core.VerdictPASS {
				t.Fatalf("host rejected the restored Builder tree: %s %+v", resp.Verdict, resp.Diagnostics)
			}
		})
	}
}
