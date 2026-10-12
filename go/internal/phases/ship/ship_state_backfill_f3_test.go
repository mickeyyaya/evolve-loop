package ship

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestReconcileManifest_CommitsOnTheCycleBranchJoinTheChangedSet(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	mustWrite(t, filepath.Join(ws, "build-report.md"), "changed `docs/declared.md`")
	runner := scriptedRunnerF3(func(args []string, out, _ io.Writer) (int, error) {
		switch {
		case hasArgF3(args, "status"):
			io.WriteString(out, " M docs/declared.md\n")
		case hasArgF3(args, "diff"):
			io.WriteString(out, "docs/committed-undeclared.md\n\n  docs/declared.md  \n")
		}
		return 0, nil
	})
	opts := &Options{WorkspacePath: ws, ProjectRoot: t.TempDir(), Runner: runner, Stdout: io.Discard, Stderr: io.Discard}
	res := &RunResult{}

	if err := reconcileManifest(context.Background(), opts, res, "wt", "main", "cycle"); err != nil {
		t.Fatalf("shadow must not block: %v", err)
	}

	logs := strings.Join(res.Logs, "\n")
	if !strings.Contains(logs, "1 out-of-manifest path(s) about to be bound (SHADOW") || !strings.Contains(logs, "docs/committed-undeclared.md") || strings.Contains(logs, "docs/declared.md,") {
		t.Fatalf("a path committed on the cycle branch is bound even when git status is clean; logs=%v", res.Logs)
	}
	opts.ManifestGate = ManifestGateEnforce
	err := reconcileManifest(context.Background(), opts, &RunResult{}, "wt", "main", "cycle")
	if err == nil || !strings.Contains(err.Error(), "docs/committed-undeclared.md") {
		t.Fatalf("enforce must block the committed undeclared path, got %v", err)
	}
}

func TestReconcileManifest_AFailingDiffOrStatusLeavesTheChangedSetToTheOtherSource(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	mustWrite(t, filepath.Join(ws, "build-report.md"), "changed `docs/declared.md`")
	runner := scriptedRunnerF3(func(args []string, out, _ io.Writer) (int, error) {
		if hasArgF3(args, "diff") {
			io.WriteString(out, "docs/never-read.md\n")
			return 128, nil
		}
		io.WriteString(out, " M docs/declared.md\n")
		return 0, nil
	})
	opts := &Options{WorkspacePath: ws, ProjectRoot: t.TempDir(), Runner: runner, ManifestGate: ManifestGateEnforce, Stdout: io.Discard, Stderr: io.Discard}
	res := &RunResult{}

	if err := reconcileManifest(context.Background(), opts, res, "wt", "main", "cycle"); err != nil {
		t.Fatalf("a failed branch diff must not invent paths: %v", err)
	}

	if !strings.Contains(strings.Join(res.Logs, "\n"), "all 1 bound path(s) covered") {
		t.Fatalf("logs = %v", res.Logs)
	}
}

func decisionCycleF3(t *testing.T, cycle int, decision string) (root, inbox string) {
	t.Helper()
	root = t.TempDir()
	mustWrite(t, filepath.Join(root, ".evolve", "cycle-state.json"), `{"cycle_id":`+strconv.Itoa(cycle)+`}`)
	mustWrite(t, filepath.Join(root, ".evolve", "runs", "cycle-"+strconv.Itoa(cycle), "triage-decision.json"), decision)
	return root, filepath.Join(root, ".evolve", "inbox")
}

func TestPromoteInbox_ASupersededItemIsRetiredAndSaidSo(t *testing.T) {
	t.Parallel()
	root, inbox := decisionCycleF3(t, 31, `{"top_n":[],"superseded":["old-1","old-1",""]}`)
	writeDrainInboxItem(t, inbox, "old-1")
	res := &RunResult{}

	if err := promoteInbox(context.Background(), &Options{ProjectRoot: root, Stderr: io.Discard}, res); err != nil {
		t.Fatal(err)
	}

	if !containsLog(*res, "retired 1 superseded inbox item(s) for cycle 31: [old-1]") {
		t.Fatalf("logs = %v", res.Logs)
	}
	if _, err := os.Stat(filepath.Join(inbox, "old-1.json")); !os.IsNotExist(err) {
		t.Fatalf("the superseded item must leave the inbox root: %v", err)
	}
}

func TestPromoteInbox_ASupersededRetirementThatCannotBeDeliveredIsAWarningNotAFailure(t *testing.T) {
	t.Parallel()
	root, inbox := decisionCycleF3(t, 32, `{"top_n":[],"superseded":["old-2"]}`)
	writeDrainInboxItem(t, inbox, "old-2")
	blockDrainPath(t, filepath.Join(inbox, "processed"))
	res := &RunResult{}

	if err := promoteInbox(context.Background(), &Options{ProjectRoot: root, Stderr: io.Discard}, res); err != nil {
		t.Fatalf("a retirement fault is logged, not raised: %v", err)
	}

	if !containsLog(*res, "WARN: superseded reconcile for cycle 32: ") || containsLog(*res, "retired 1 superseded") {
		t.Fatalf("logs = %v", res.Logs)
	}
	if _, err := os.Stat(filepath.Join(inbox, "old-2.json")); err != nil {
		t.Fatalf("an item that could not be retired stays pickable: %v", err)
	}
}

func TestPromoteInbox_ACommittedIdNoInboxItemBacksIsRetiredAndSaidSo(t *testing.T) {
	t.Parallel()
	root, _ := decisionCycleF3(t, 33, `{"top_n":[{"id":"ghost-1"}]}`)
	res := &RunResult{}

	if err := promoteInbox(context.Background(), &Options{ProjectRoot: root, Stderr: io.Discard}, res); err != nil {
		t.Fatal(err)
	}

	if !containsLog(*res, "retired 1 committed id(s) no inbox item backs for cycle 33: [ghost-1]") || !containsLog(*res, "inbox lifecycle drain complete for cycle 33") {
		t.Fatalf("logs = %v", res.Logs)
	}
}
