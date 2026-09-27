package core

import (
	"bytes"
	"context"
	"regexp"
	"testing"
)

func TestIdenticalChange_HoldsAcrossACleanRebaseAndDeclinesAnyByte(t *testing.T) {
	fx := newShippedLane(t, "lane.txt", addLaneFile, addPeerFile)
	ctx := context.Background()
	if declined, err := unwindShipCommit(ctx, fx.worktree, fx.audited(), gitCapture); err != nil || declined != "" {
		t.Fatalf("unwindShipCommit = (%q, %v)", declined, err)
	}
	fx.git("rebase", "-q", "main")
	if err := pendRebasedChange(ctx, fx.worktree, gitCapture); err != nil {
		t.Fatal(err)
	}
	tree1 := fx.git("write-tree")

	audited, composed, ok, err := identicalChange(ctx, gitCapture, fx.worktree, fx.base, fx.auditedTree, fx.newBase, tree1)

	if err != nil || !ok || len(audited) == 0 || !bytes.Equal(audited, composed) {
		t.Fatalf("identicalChange = (ok %v, err %v, %d/%d bytes); the pended change on the new base is the audited change byte for byte", ok, err, len(audited), len(composed))
	}

	staged := fx.git("show", ":lane.txt")
	flipped := []byte(staged + "\n")
	flipped[0] ^= 0x20
	fx.write("lane.txt", string(flipped))
	fx.git("add", "lane.txt")
	if _, _, ok, err := identicalChange(ctx, gitCapture, fx.worktree, fx.base, fx.auditedTree, fx.newBase, fx.git("write-tree")); err != nil || ok {
		t.Fatalf("one flipped byte of the same length declines: ok=%v err=%v", ok, err)
	}
	if _, _, ok, err := identicalChange(ctx, gitCapture, fx.worktree, fx.base, fx.base+"^{tree}", fx.newBase, fx.newBase+"^{tree}"); err != nil || ok {
		t.Fatalf("an empty change never carries: ok=%v err=%v", ok, err)
	}
	if _, _, _, err := identicalChange(ctx, gitCapture, fx.worktree, "not-a-commit", fx.auditedTree, fx.newBase, tree1); err == nil {
		t.Fatal("an unreadable base is a fault, not a decline")
	}
}

func TestTreeDelta_IsByteExactWithFullBlobIDs(t *testing.T) {
	fx := newShippedLane(t, "lane.txt", addLaneFile, addPeerFile)
	fx.write("lane.txt", "the lane file \n")
	fx.git("add", "lane.txt")

	delta, err := treeDelta(context.Background(), gitCapture, fx.worktree, "HEAD", fx.git("write-tree"))

	if err != nil || len(delta) == 0 {
		t.Fatalf("a trailing space is a change: %q %v", delta, err)
	}
	if !regexp.MustCompile(`(?m)^index [0-9a-f]{40}\.\.[0-9a-f]{40}`).Match(delta) {
		t.Errorf("the delta names full blob ids, so a blob swap never hides behind an abbreviation:\n%s", delta)
	}
}
