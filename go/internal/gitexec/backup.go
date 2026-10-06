package gitexec

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const defaultRemoteMain = "origin/main"

type HeadPlace int

const (
	HeadInMain HeadPlace = iota
	HeadOnOtherRemoteBranch
	HeadOnlyInBundle
)

func (p HeadPlace) String() string {
	switch p {
	case HeadInMain:
		return "IN_MAIN"
	case HeadOnOtherRemoteBranch:
		return "OTHER_BRANCH"
	}
	return "ONLY_IN_BACKUP"
}

type BundleHead struct{ SHA, Ref string }

func (g Git) BundleHeads(ctx context.Context, bundlePath string) ([]BundleHead, error) {
	out, err := g.Output(ctx, "bundle", "list-heads", bundlePath)
	if err != nil {
		return nil, fmt.Errorf("gitexec: bundle list-heads %s: %w", bundlePath, err)
	}
	var heads []BundleHead
	for _, line := range strings.Split(out, "\n") {
		sha, ref, _ := strings.Cut(strings.TrimSpace(line), " ")
		if sha != "" {
			heads = append(heads, BundleHead{SHA: sha, Ref: ref})
		}
	}
	return heads, nil
}

func (g Git) ClassifyHead(ctx context.Context, sha string) (HeadPlace, error) {
	_, _, code, err := g.Capture(ctx, "cat-file", "-e", sha+"^{commit}")
	if err != nil {
		return HeadOnlyInBundle, fmt.Errorf("gitexec: cat-file %s: %w", sha, err)
	}
	if code != 0 {
		return HeadOnlyInBundle, nil
	}
	_, stderr, code, err := g.Capture(ctx, "merge-base", "--is-ancestor", sha, defaultRemoteMain)
	if err != nil {
		return HeadOnlyInBundle, fmt.Errorf("gitexec: merge-base %s: %w", sha, err)
	}
	switch code {
	case 0:
		return HeadInMain, nil
	case 1:
	default:
		return HeadOnlyInBundle, fmt.Errorf("gitexec: merge-base --is-ancestor %s %s exit=%d: %s", sha, defaultRemoteMain, code, strings.TrimSpace(stderr))
	}
	out, err := g.Output(ctx, "branch", "-r", "--contains", sha)
	if err != nil {
		return HeadOnlyInBundle, fmt.Errorf("gitexec: branch -r --contains %s: %w", sha, err)
	}
	for _, line := range strings.Split(out, "\n") {
		if name := strings.TrimSpace(line); name != "" && !strings.Contains(name, "->") {
			return HeadOnOtherRemoteBranch, nil
		}
	}
	return HeadOnlyInBundle, nil
}

func (g Git) PatchApplied(ctx context.Context, patchPath string) (bool, error) {
	scratch, err := os.MkdirTemp("", "gitexec-patch-index-*")
	if err != nil {
		return false, fmt.Errorf("gitexec: scratch index: %w", err)
	}
	defer func() { _ = os.RemoveAll(scratch) }()
	onMain := Git{Dir: g.Dir, Exec: isolate(g.Exec, "GIT_INDEX_FILE="+filepath.Join(scratch, "index"))}
	if err := onMain.Run(ctx, "read-tree", defaultRemoteMain); err != nil {
		return false, fmt.Errorf("gitexec: read-tree %s: %w", defaultRemoteMain, err)
	}
	_, stderr, code, err := onMain.Capture(ctx, "apply", "--check", "--reverse", "--cached", patchPath)
	if err != nil {
		return false, fmt.Errorf("gitexec: apply --check %s: %w", patchPath, err)
	}
	switch code {
	case 0:
		return true, nil
	case 1:
		return false, nil
	}
	return false, fmt.Errorf("gitexec: apply --check --reverse --cached %s against %s exit=%d: %s", patchPath, defaultRemoteMain, code, strings.TrimSpace(stderr))
}
