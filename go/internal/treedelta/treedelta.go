// Package treedelta is the byte-exact change between a base and a tree, as a carry across a rebase
// must prove it: full blob ids, binary content, no rename detection, no textconv.
package treedelta

import (
	"bytes"
	"context"
	"fmt"
	"strings"
)

// Git runs one git command in dir and returns its stdout, exit code and error.
type Git func(ctx context.Context, dir string, args ...string) (string, int, error)

// Args is the one diff invocation every reader of a delta uses.
func Args(base, tree string) []string {
	return []string{"diff", "--binary", "--full-index", "--no-ext-diff", "--no-textconv", "--no-renames", base, tree}
}

// Delta is the change from base to tree.
func Delta(ctx context.Context, git Git, dir, base, tree string) ([]byte, error) {
	args := Args(base, tree)
	out, code, err := git(ctx, dir, args...)
	if err != nil {
		return nil, err
	}
	if code != 0 {
		return nil, fmt.Errorf("git %s: exit %d", strings.Join(args, " "), code)
	}
	return []byte(out), nil
}

// Identical holds when the change from base1 to tree1 is, byte for byte, the change from base0 to
// tree0, and that change is not empty. Both deltas come back for the record that carries them.
func Identical(ctx context.Context, git Git, dir, base0, tree0, base1, tree1 string) (audited, composed []byte, ok bool, err error) {
	if audited, err = Delta(ctx, git, dir, base0, tree0); err != nil {
		return nil, nil, false, err
	}
	if composed, err = Delta(ctx, git, dir, base1, tree1); err != nil {
		return nil, nil, false, err
	}
	return audited, composed, len(audited) > 0 && bytes.Equal(audited, composed), nil
}
