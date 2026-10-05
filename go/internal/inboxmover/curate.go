package inboxmover

import (
	"context"
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover/lifecycle"
)

type (
	EditOp    = lifecycle.EditOp
	FieldEdit = lifecycle.FieldEdit
)

const (
	EditSet    = lifecycle.EditSet
	EditAdd    = lifecycle.EditAdd
	EditRemove = lifecycle.EditRemove
)

var ErrNotWithdrawable = lifecycle.ErrNotWithdrawable

func Edit(opts Options, target string, edits []FieldEdit) (string, error) {
	return opts.mover().Edit(target, edits)
}

func Withdraw(opts Options, taskID, reason string) (string, error) {
	return opts.mover().Withdraw(taskID, reason)
}

func VerifyPremise(opts Options, taskID, evidence string) (string, error) {
	return opts.mover().VerifyPremise(taskID, evidence)
}

const verifiedMainRef = "origin/main"

func originMainHead(root string) (string, error) {
	out, stderr, code, err := gitexec.Default(root).Capture(context.Background(), "rev-parse", "--verify", "--quiet", verifiedMainRef+"^{commit}")
	switch {
	case err != nil:
		return "", fmt.Errorf("git rev-parse %s: %w", verifiedMainRef, err)
	case code != 0:
		return "", fmt.Errorf("git rev-parse %s exit=%d: %s", verifiedMainRef, code, strings.TrimSpace(stderr))
	}
	return strings.TrimSpace(out), nil
}
