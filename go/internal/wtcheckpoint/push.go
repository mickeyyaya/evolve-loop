package wtcheckpoint

import (
	"context"
	"fmt"
	"strings"
)

func Push(ctx context.Context, h Hub, refs []string) error {
	if len(refs) == 0 {
		return nil
	}
	args := []string{"push", "--quiet", remoteName}
	for _, r := range refs {
		if !strings.HasPrefix(r, refPrefix) {
			return fmt.Errorf("wtcheckpoint: refused: %s is not under %s; push sends checkpoints only", r, refPrefix)
		}
		args = append(args, r+":"+r)
	}
	if _, stderr, code, err := h.git().Capture(ctx, args...); err != nil || code != 0 {
		return fmt.Errorf("wtcheckpoint: git push %s rc=%d err=%v: %s", remoteName, code, err, strings.TrimSpace(stderr))
	}
	return nil
}
