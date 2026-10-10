package core

import (
	"context"
	"strings"
	"testing"
)

func (fx *shippedLane) audited() AuditedChange {
	return AuditedChange{Base: fx.base, Tree: fx.auditedTree, Label: "cycle-42/run-42"}
}

func TestUnwindShipCommit_RestoresTheAuditedTreeOnTheAuditedBase(t *testing.T) {
	fx := newShippedLane(t, "lane.txt", addLaneFile, addPeerFile)

	declined, err := unwindShipCommit(context.Background(), fx.worktree, fx.audited(), gitCapture)

	if err != nil || declined != "" {
		t.Fatalf("unwindShipCommit = (%q, %v), want an unwind", declined, err)
	}
	if parent := fx.git("rev-parse", "HEAD^"); parent != fx.base {
		t.Fatalf("carrier parent = %s, want the audited base %s", parent, fx.base)
	}
	if tree := fx.git("rev-parse", "HEAD^{tree}"); tree != fx.auditedTree {
		t.Fatalf("carrier tree = %s, want the audited tree %s", tree, fx.auditedTree)
	}
	if msg := fx.git("log", "-1", "--format=%B"); !strings.Contains(msg, "Evolve-Carrier: cycle-42/run-42") {
		t.Fatalf("carrier message %q lacks the trailer a resume heal recognises", msg)
	}
	if status := fx.git("status", "--porcelain"); status != "" {
		t.Fatalf("worktree differs from the audited tree:\n%s", status)
	}
}

func TestUnwindShipCommit_DeclinesWithoutMovingHEAD(t *testing.T) {
	cases := []struct {
		name, reason string
		setup        func(fx *shippedLane) AuditedChange
	}{
		{"a non-inbox change after audit", "beyond its inbox consumption", func(fx *shippedLane) AuditedChange {
			fx.write("extra.txt", "not audited\n")
			fx.commitAll("post-audit change")
			return fx.audited()
		}},
		{"an inbox change that is not a consumption", "beyond its inbox consumption", func(fx *shippedLane) AuditedChange {
			fx.write(".evolve/inbox/new.json", "{\"id\":\"new\"}\n")
			fx.commitAll("an inbox item filed after audit")
			return fx.audited()
		}},
		{"a consumed item that released a continuation", "released a continuation", func(fx *shippedLane) AuditedChange {
			fx.write(".evolve/inbox/consumed/item.json", "{\"id\":\"item\",\"released_continuations\":[{\"scope\":\"x\"}]}\n")
			fx.commitAll("consumption with a released continuation")
			return fx.audited()
		}},
		{"a removal from the inbox with no consumed copy", "not whole consumption pairs", func(fx *shippedLane) AuditedChange {
			fx.git("rm", "-q", ".evolve/inbox/consumed/item.json")
			fx.git("commit", "-q", "-m", "drop the consumed copy")
			return fx.audited()
		}},
		{"a consumed copy with no removal", "not whole consumption pairs", func(fx *shippedLane) AuditedChange {
			fx.write(inboxItem, itemBody)
			fx.commitAll("restore the root item")
			return fx.audited()
		}},
		{"a consumed copy under another name", "not whole consumption pairs", func(fx *shippedLane) AuditedChange {
			fx.git("mv", ".evolve/inbox/consumed/item.json", ".evolve/inbox/consumed/other.json")
			fx.git("commit", "-q", "-m", "consume under another name")
			return fx.audited()
		}},
		{"a dirty worktree", "ship did not commit", func(fx *shippedLane) AuditedChange {
			fx.write("base.txt", "uncommitted edit\n")
			return fx.audited()
		}},
		{"an untracked file", "ship did not commit", func(fx *shippedLane) AuditedChange {
			fx.write("stray.txt", "left behind\n")
			return fx.audited()
		}},
		{"a base that is not where the lane forked", "did not fork at the audited base", func(fx *shippedLane) AuditedChange {
			a := fx.audited()
			a.Base = fx.newBase
			return a
		}},
		{"an audited tree git does not have", "does not hold the audited tree", func(fx *shippedLane) AuditedChange {
			a := fx.audited()
			a.Tree = strings.Repeat("0", 40)
			return a
		}},
		{"an audited tree that is not an object id", "not an object id", func(fx *shippedLane) AuditedChange {
			a := fx.audited()
			a.Tree = "--output=/tmp/x"
			return a
		}},
		{"no audited tree", "not an object id", func(fx *shippedLane) AuditedChange {
			a := fx.audited()
			a.Tree = ""
			return a
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fx := newShippedLane(t, "lane.txt", addLaneFile, addPeerFile)
			audited := tc.setup(fx)
			head, status := fx.git("rev-parse", "HEAD"), fx.git("status", "--porcelain")

			declined, err := unwindShipCommit(context.Background(), fx.worktree, audited, gitCapture)

			if err != nil || !strings.Contains(declined, tc.reason) {
				t.Fatalf("unwindShipCommit = (%q, %v), want a decline naming %q", declined, err, tc.reason)
			}
			if after := fx.git("rev-parse", "HEAD"); after != head {
				t.Fatalf("HEAD moved from %s to %s on a decline", head, after)
			}
			if after := fx.git("status", "--porcelain"); after != status {
				t.Fatalf("worktree changed on a decline:\nbefore:\n%s\nafter:\n%s", status, after)
			}
		})
	}
}

func TestPendRebasedChange_LeavesTheChangePendingOnItsForkPoint(t *testing.T) {
	fx := newShippedLane(t, "lane.txt", addLaneFile, addPeerFile)
	ctx := context.Background()
	if declined, err := unwindShipCommit(ctx, fx.worktree, fx.audited(), gitCapture); err != nil || declined != "" {
		t.Fatalf("unwindShipCommit = (%q, %v)", declined, err)
	}
	fx.git("rebase", "-q", "main")
	fx.landOnMain("later.txt", "a later landing\n")

	if err := pendRebasedChange(ctx, fx.worktree, gitCapture); err != nil {
		t.Fatal(err)
	}

	fx.requirePendingLaneChangeOn(fx.newBase)
}

func TestPendRebasedChange_AfterAnAbortedRebaseRestoresTheAuditedShape(t *testing.T) {
	fx := newShippedLane(t, "shared.txt", editSharedLine(1, "lane edit"), editSharedLine(1, "peer edit"))
	ctx := context.Background()
	if declined, err := unwindShipCommit(ctx, fx.worktree, fx.audited(), gitCapture); err != nil || declined != "" {
		t.Fatalf("unwindShipCommit = (%q, %v)", declined, err)
	}
	if ok, conflicts := rebaseWithDerivedRegen(ctx, fx.worktree, gitCapture, regenerateDerivedArtifact, derivedEntryOf); ok || len(conflicts) == 0 {
		t.Fatalf("rebase = (ok %v, conflict %v), want the peer's edit to the same line to conflict", ok, conflicts)
	}

	if err := pendRebasedChange(ctx, fx.worktree, gitCapture); err != nil {
		t.Fatal(err)
	}

	fx.requirePendingLaneChangeOn(fx.base)
}

func TestUnwindToAuditedShape_LeavesTheAuditedChangePendingOnTheAuditedBase(t *testing.T) {
	fx := newShippedLane(t, "lane.txt", addLaneFile, addPeerFile)

	declined, err := UnwindToAuditedShape(context.Background(), fx.worktree, fx.audited())

	if err != nil || declined != "" {
		t.Fatalf("UnwindToAuditedShape = (%q, %v), want the lane back in the audited shape", declined, err)
	}
	fx.requirePendingLaneChangeOn(fx.base)
}

func TestUnwindToAuditedShape_DeclinesWithoutTouchingTheLane(t *testing.T) {
	fx := newShippedLane(t, "lane.txt", addLaneFile, addPeerFile)
	fx.write("stray.txt", "left behind\n")

	declined, err := UnwindToAuditedShape(context.Background(), fx.worktree, fx.audited())

	if err != nil || !strings.Contains(declined, "ship did not commit") {
		t.Fatalf("UnwindToAuditedShape = (%q, %v), want a decline naming the stray file", declined, err)
	}
	if head := fx.git("rev-parse", "HEAD"); head != fx.shipCommit {
		t.Errorf("HEAD = %s, want ship's commit %s untouched", head, fx.shipCommit)
	}
}
