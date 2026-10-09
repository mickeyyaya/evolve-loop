package audit

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const auditProbesDir = "audit-probes"

var auditPromptArtifact = phasecontract.PromptArtifactFilename("audit")

const quarantineGitTimeout = 30 * time.Second

const anchorFileName = ".dispatch-anchor"

func quarantineProbesForRequest(req core.PhaseRequest) error {
	if req.WorktreeVerified {
		return nil
	}
	if req.Worktree == "" {
		fmt.Fprintf(os.Stderr, "[audit] probe quarantine skipped: no worktree on cycle %d — the EGPS suite runs against the main tree unscanned\n", req.Cycle)
		return nil
	}
	fi, err := os.Stat(filepath.Join(req.Workspace, auditPromptArtifact))
	if err != nil {
		fmt.Fprintf(os.Stderr, "[audit] probe quarantine skipped: no %s dispatch anchor in %s (%v)\n", auditPromptArtifact, req.Workspace, err)
		return nil
	}
	_, qerr := quarantineAuditProbes(req.Worktree, req.Workspace, firstDispatchAnchor(req.Workspace, fi.ModTime()), os.Stderr)
	return qerr
}

func firstDispatchAnchor(workspace string, promptMtime time.Time) time.Time {
	stamp := filepath.Join(workspace, auditProbesDir, anchorFileName)
	if fi, err := os.Stat(stamp); err == nil {
		return fi.ModTime()
	}
	if err := os.MkdirAll(filepath.Dir(stamp), 0o755); err != nil {
		return promptMtime
	}
	if err := os.WriteFile(stamp, nil, 0o644); err != nil {
		return promptMtime
	}
	_ = os.Chtimes(stamp, promptMtime, promptMtime)
	return promptMtime
}

func quarantineAuditProbes(worktree, workspace string, dispatchedAt time.Time, log io.Writer) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), quarantineGitTimeout)
	defer cancel()
	run := runCmd
	out, err := sysexec.Output(ctx, run, worktree, "git", "status", "--porcelain", "-uall")
	if err != nil {
		fmt.Fprintf(log, "[audit] probe quarantine degraded OPEN: git status failed (%v) — tree not scanned for audit-authored probes\n", err)
		return nil, nil
	}

	var moved []string
	for _, line := range strings.Split(out, "\n") {
		rel, ok := newTestFilePath(line)
		if !ok {
			continue
		}
		abs := filepath.Join(worktree, rel)
		fi, statErr := os.Stat(abs)
		if statErr != nil || fi.ModTime().Before(dispatchedAt) {
			continue
		}
		if err := preserveThenRemove(abs, filepath.Join(workspace, auditProbesDir, rel)); err != nil {
			return moved, fmt.Errorf("probe quarantine: %s: %w", rel, err)
		}
		moved = append(moved, rel)
		fmt.Fprintf(log, "[audit] quarantined audit-authored probe test %s → %s/%s (preserved, excluded from the EGPS tree — use `go test -overlay` for probes)\n",
			rel, auditProbesDir, rel)
	}
	return moved, nil
}

func preserveThenRemove(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	raw, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.WriteFile(dst, raw, 0o644); err != nil {
		return err
	}
	return os.Remove(src)
}

func newTestFilePath(line string) (string, bool) {
	if len(line) < 4 {
		return "", false
	}
	code := line[:2]
	if code != "??" && !strings.Contains(code, "A") {
		return "", false
	}
	p := gitexec.PorcelainPath(line)
	if p == "" || strings.HasSuffix(p, "/") || !strings.HasSuffix(p, "_test.go") {
		return "", false
	}
	if strings.HasPrefix(p, "go/acs/") {
		return "", false
	}
	return p, true
}
