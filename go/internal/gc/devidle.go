package gc

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

const (
	reflogStampPrefix = "HEAD@{"
	reflogStampSuffix = "}"
)

func parseReflogStamp(s string) (time.Time, error) {
	digits, ok := strings.CutPrefix(strings.TrimSpace(s), reflogStampPrefix)
	if ok {
		digits, ok = strings.CutSuffix(digits, reflogStampSuffix)
	}
	if !ok {
		return time.Time{}, fmt.Errorf("gc: reflog stamp %q is not HEAD@{<unix seconds>}", s)
	}
	return parseUnixSeconds(digits)
}

func parseUnixSeconds(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, r := range s {
		if r < '0' || r > '9' {
			return time.Time{}, fmt.Errorf("gc: %q is not a count of unix seconds", s)
		}
	}
	secs, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("gc: %q is not a count of unix seconds: %w", s, err)
	}
	return time.Unix(secs, 0), nil
}

func lastActivity(ctx context.Context, tree gitexec.Git) (time.Time, error) {
	commitOut, err := tree.Output(ctx, "log", "-1", "--format=%ct", "HEAD")
	if err != nil {
		return time.Time{}, err
	}
	last, err := parseUnixSeconds(commitOut)
	if err != nil {
		return time.Time{}, err
	}
	reflogOut, err := tree.Output(ctx, "reflog", "show", "--date=unix", "--format=%gd", "-n", "1", "HEAD")
	if err != nil || reflogOut == "" {
		return last, err
	}
	logged, err := parseReflogStamp(reflogOut)
	if err != nil {
		return time.Time{}, err
	}
	if logged.After(last) {
		last = logged
	}
	return last, nil
}
