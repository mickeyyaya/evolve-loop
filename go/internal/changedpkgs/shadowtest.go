package changedpkgs

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/lanebudget"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type ShadowTestResult struct {
	BaseRef    string   `json:"base_ref,omitempty"`
	MergeBase  string   `json:"merge_base,omitempty"`
	Selected   []string `json:"selected"`
	DurationMS int64    `json:"duration_ms"`
	Pass       bool     `json:"pass"`
	Output     string   `json:"output,omitempty"`
}

func RunShadowTests(ctx context.Context, repoRoot, baseRef string, run sysexec.RunFunc) (ShadowTestResult, error) {
	if repoRoot == "" {
		return ShadowTestResult{}, fmt.Errorf("changedpkgs: repo root is required")
	}
	if baseRef == "" {
		baseRef = "main"
	}
	if run == nil {
		run = sysexec.DefaultRunner
	}
	run = lanebudget.Runner(run)
	mergeBase, err := gitexec.Default(repoRoot).Output(ctx, "merge-base", "HEAD", baseRef)
	if err != nil {
		return ShadowTestResult{}, fmt.Errorf("changedpkgs: merge-base HEAD %s: %w", baseRef, err)
	}
	files, ok := ChangedFilesChecked(repoRoot, mergeBase)
	if !ok {
		return ShadowTestResult{}, fmt.Errorf("changedpkgs: cannot read changed files against %s", mergeBase)
	}
	pkgs := PackagesOf(files)
	closure, ok := ImporterClosureChecked(repoRoot, pkgs)
	if !ok {
		return ShadowTestResult{}, fmt.Errorf("changedpkgs: cannot compute importer closure for %s", repoRoot)
	}
	selected := withoutAcs(closure.Testable)
	if selected == nil {
		selected = []string{}
	}
	res := ShadowTestResult{
		BaseRef:   baseRef,
		MergeBase: mergeBase,
		Selected:  selected,
		Pass:      true,
	}
	if len(selected) == 0 {
		return res, nil
	}
	moduleDir := filepath.Join(repoRoot, "go")
	args := append([]string{"test", "-count=1"}, selected...)
	start := time.Now()
	var buf strings.Builder
	code, err := run(ctx, "go", moduleDir, args, nil, nil, &buf, &buf)
	res.DurationMS = time.Since(start).Milliseconds()
	res.Output = buf.String()
	if err != nil {
		return ShadowTestResult{}, fmt.Errorf("changedpkgs: go test: %w", err)
	}
	res.Pass = code == 0
	return res, nil
}
