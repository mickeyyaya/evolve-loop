package ship

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

type carriedShipAfterPush struct {
	lane carriedLane
	head string
	opts *Options
}

func carriedShipPushed(t *testing.T, withCarry bool) carriedShipAfterPush {
	t.Helper()
	l := rebasedLane(t)
	seedAudit(t, l.repo, "PASS", map[string]string{"bound_tree": l.tree0})
	ref := mustHashFile(t, filepath.Join(l.repo, ".evolve", "runs", "cycle-1", "audit-report.md"))
	if withCarry {
		writeCarry(t, l, ref, l.tree0)
	}
	runGit(t, l.repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "carried lane")
	head := strings.TrimSpace(runGitOut(t, l.repo, "rev-parse", "HEAD"))
	mustWrite(t, filepath.Join(l.repo, ".evolve", "runs", "cycle-1", "ship-binding.json"), fmt.Sprintf(
		`{"audit_bound_tree_sha":%q,"tree_sha_committed":%q,"commit_sha":%q,"cycle":1}`, l.tree0, l.tree1, head))
	opts := auditOpts(t, l.repo)
	opts.Class = ClassCycle
	return carriedShipAfterPush{lane: l, head: head, opts: opts}
}

func TestCheckPostPushIdempotency_HealsACarriedShipInterruptedAfterItsPush(t *testing.T) {
	t.Parallel()
	c := carriedShipPushed(t, true)

	sha, idempotent, err := checkPostPushIdempotency(context.Background(), c.opts)

	if err != nil || !idempotent || sha != c.head {
		t.Fatalf("checkPostPushIdempotency = (%q, %v, %v); a carried ship whose committed tree differs from the audited tree by design must resume report-only, not re-audit", sha, idempotent, err)
	}
}

func TestCheckPostPushIdempotency_DoesNotHealADriftNoCarryExplains(t *testing.T) {
	t.Parallel()
	c := carriedShipPushed(t, false)

	sha, idempotent, err := checkPostPushIdempotency(context.Background(), c.opts)

	if err != nil || idempotent || sha != "" {
		t.Fatalf("checkPostPushIdempotency = (%q, %v, %v); a tree that differs from the audited one with no re-proven carry must fall through to the full gates", sha, idempotent, err)
	}
}

func TestVerifyPostPushPredicateEvidence_AcceptsACarriedLanding(t *testing.T) {
	t.Parallel()
	c := carriedShipPushed(t, true)

	err := verifyPostPushPredicateEvidence(context.Background(), c.opts, &RunResult{}, c.head)

	if err != nil {
		t.Fatalf("verifyPostPushPredicateEvidence = %v; the carried landing is the audited change byte for byte and its predicate evidence binds the audited tree", err)
	}
}

func TestVerifyPostPushPredicateEvidence_RefusesALandingNoCarryExplains(t *testing.T) {
	t.Parallel()
	c := carriedShipPushed(t, false)

	err := verifyPostPushPredicateEvidence(context.Background(), c.opts, &RunResult{}, c.head)

	wantShipErr(t, err, core.CodeAuditBindingTreeMismatch, core.ShipClassPrecondition, "landed commit differs")
}

func TestCheckPostPushIdempotency_FailsLoudlyWhenAGitReadFails(t *testing.T) {
	t.Parallel()
	boom := errors.New("git unavailable")
	cases := map[string]func(c carriedShipAfterPush, r *argvRecorder){
		"HEAD": func(c carriedShipAfterPush, r *argvRecorder) { r.on("rev-parse HEAD", scriptedCall{err: boom}) },
		"HEAD^{tree}": func(c carriedShipAfterPush, r *argvRecorder) {
			r.on("rev-parse HEAD", scriptedCall{stdout: c.head + "\n"})
			r.on("rev-parse HEAD^{tree}", scriptedCall{err: boom})
		},
	}
	for name, script := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c := carriedShipPushed(t, true)
			script(c, newArgvRecorder(c.opts))

			sha, idempotent, err := checkPostPushIdempotency(context.Background(), c.opts)

			if err == nil || idempotent || sha != "" {
				t.Fatalf("checkPostPushIdempotency = (%q, %v, %v); an unreadable %s must fail loudly, not heal", sha, idempotent, err, name)
			}
		})
	}
}

func TestCheckPostPushIdempotency_FailsLoudlyOnAMalformedBinding(t *testing.T) {
	t.Parallel()
	c := carriedShipPushed(t, true)
	mustWrite(t, filepath.Join(c.lane.repo, ".evolve", "runs", "cycle-1", "ship-binding.json"), "not json")

	_, idempotent, err := checkPostPushIdempotency(context.Background(), c.opts)

	if err == nil || idempotent {
		t.Fatalf("checkPostPushIdempotency = (%v, %v); a malformed ship binding must surface its error", idempotent, err)
	}
}

func TestVerifyPostPushPredicateEvidence_FailsLoudlyWhenTheLandedTreeIsUnreadable(t *testing.T) {
	t.Parallel()
	c := carriedShipPushed(t, true)
	newArgvRecorder(c.opts).on("rev-parse "+c.head+"^{tree}", scriptedCall{err: errors.New("git unavailable")})

	err := verifyPostPushPredicateEvidence(context.Background(), c.opts, &RunResult{}, c.head)

	if err == nil {
		t.Fatal("verifyPostPushPredicateEvidence = nil; an unreadable landed tree must fail loudly")
	}
}

func TestCheckPostPushIdempotency_FailsLoudlyWhenTheCycleStateIsUnreadable(t *testing.T) {
	t.Parallel()
	c := carriedShipPushed(t, true)
	c.opts.CycleID = 0
	mustWrite(t, filepath.Join(c.lane.repo, ".evolve", "cycle-state.json"), "not json")

	_, idempotent, err := checkPostPushIdempotency(context.Background(), c.opts)

	if err == nil || idempotent {
		t.Fatalf("checkPostPushIdempotency = (%v, %v); an unreadable cycle state must surface its error", idempotent, err)
	}
}

func TestVerifyPostPushPredicateEvidence_FailsLoudlyWithoutTheAuditItBindsTo(t *testing.T) {
	t.Parallel()
	c := carriedShipPushed(t, true)
	missingRun := *c.opts
	missingRun.RunID = "a-run-no-audit-names"
	noReport := carriedShipPushed(t, true)
	if err := os.Remove(filepath.Join(noReport.lane.repo, ".evolve", "runs", "cycle-1", "audit-report.md")); err != nil {
		t.Fatal(err)
	}

	if err := verifyPostPushPredicateEvidence(context.Background(), &missingRun, &RunResult{}, c.head); err == nil {
		t.Error("a run with no auditor entry must fail loudly")
	}
	if err := verifyPostPushPredicateEvidence(context.Background(), noReport.opts, &RunResult{}, noReport.head); err == nil {
		t.Error("an audit report that is gone must fail loudly")
	}
}
