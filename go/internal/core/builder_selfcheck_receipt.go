package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/treefence"
)

const SelfCheckReceiptFile = "builder-selfcheck-receipt.json"

type selfCheckReceipt struct {
	Tree  string `json:"tree"`
	Green bool   `json:"green"`
}

func WriteSelfCheckReceipt(ctx context.Context, workspace, worktree string, green bool) error {
	tree, err := worktreeTreeDigest(ctx, worktree)
	if err != nil {
		return fmt.Errorf("self-check receipt: %w", err)
	}
	if err := atomicwrite.JSON(filepath.Join(workspace, SelfCheckReceiptFile), selfCheckReceipt{Tree: tree, Green: green}); err != nil {
		return fmt.Errorf("self-check receipt: %w", err)
	}
	return nil
}

func selfCheckReceiptGreen(ctx context.Context, workspace, worktree string) bool {
	raw, err := os.ReadFile(filepath.Join(workspace, SelfCheckReceiptFile))
	if err != nil {
		return false
	}
	var receipt selfCheckReceipt
	if json.Unmarshal(raw, &receipt) != nil || !receipt.Green {
		return false
	}
	tree, err := worktreeTreeDigest(ctx, worktree)
	return err == nil && tree == receipt.Tree
}

func worktreeTreeDigest(ctx context.Context, worktree string) (string, error) {
	snap, err := treefence.Take(ctx, worktree)
	return snap.Tree, err
}
