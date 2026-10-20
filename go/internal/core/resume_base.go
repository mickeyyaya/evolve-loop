package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const resumeMainRef = "origin/main"

func (o ResumeOptions) withMaxAge(evolveDir string) ResumeOptions {
	if o.MaxAge > 0 {
		return o
	}
	pol, err := policy.Load(filepath.Join(evolveDir, "policy.json"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(o.Log, "[resume] WARN: policy.json is unreadable (%v); the checkpoint age bound is the compiled default\n", err)
	}
	o.MaxAge = pol.ResumeConfig().MaxCheckpointAge()
	return o
}

func staleBase(projectRoot, base, savedAt string, opts ResumeOptions) string {
	if saved, err := time.Parse(time.RFC3339, savedAt); err == nil && opts.Now().Sub(saved) > opts.MaxAge {
		return fmt.Sprintf("saved at %s, older than the %s bound of policy resume.max_checkpoint_age_h", savedAt, opts.MaxAge)
	}
	if base == "" {
		return ""
	}
	ahead, err := opts.MainAhead(projectRoot, base)
	if err != nil {
		fmt.Fprintf(opts.Log, "[resume] WARN: could not compare the base %s with %s (%v); the checkpoint is kept\n", base, resumeMainRef, err)
		return ""
	}
	if ahead == 0 {
		return ""
	}
	return fmt.Sprintf("%s is %d commit(s) past its base %s", resumeMainRef, ahead, base)
}

func checkpointBase(gitHead string, blob map[string]any) string {
	for _, b := range []string{gitHead, strFromAny(blob["worktree_base_sha"]), strFromAny(blob["pre_cycle_head"])} {
		if b != "" && b != "unknown" {
			return b
		}
	}
	return ""
}

func staleHint(statePath string) string {
	return fmt.Sprintf("discard it with `%s=%s evolve cycle reset`, or resume it deliberately with `%s=%s evolve loop --resume`",
		ipcenv.CycleStateFileKey, statePath, ipcenv.CycleStateFileKey, statePath)
}

func defaultMainAhead(projectRoot, base string) (int, error) {
	out, stderr, code, err := gitexec.Git{Dir: projectRoot, Exec: gitRunner}.Capture(context.Background(), "rev-list", "--count", base+".."+resumeMainRef)
	if err != nil {
		return 0, err
	}
	if code != 0 {
		return 0, fmt.Errorf("git rev-list %s..%s: rc=%d: %s", base, resumeMainRef, code, strings.TrimSpace(stderr))
	}
	return strconv.Atoi(strings.TrimSpace(out))
}
