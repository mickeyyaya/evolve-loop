package ship

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/shipmanifest"
)

// classifyStageRunner scripts the three git calls stageExplicitPaths makes:
// `status --porcelain` (the changed set), `check-ignore` (rc=1 — nothing
// ignored, so the pathspec survives to the add), and `add`, which fails with
// the fixture's exit code, stderr, and Go-level error.
func classifyStageRunner(porcelain, addStderr string, addExit int, addErr error) CmdRunner {
	return func(ctx context.Context, name, cwd string, args, env []string,
		stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		sub := ""
		for i := 0; i < len(args); i++ {
			a := args[i]
			if !strings.HasPrefix(a, "-") {
				sub = a
				break
			}
			if a == "-C" || a == "-c" {
				i++
			}
		}
		switch sub {
		case "status":
			_, _ = io.WriteString(stdout, porcelain)
			return 0, nil
		case "check-ignore":
			// rc=1 = no path is ignored (a success for captureGitOutput).
			return 1, nil
		case "add":
			_, _ = io.WriteString(stderr, addStderr)
			return addExit, addErr
		}
		return 0, nil
	}
}

// stageWithGitFailure drives the PRODUCTION seam (stageExplicitPaths, the sole
// caller of the classifier) against a scripted failing `git add` and returns the
// typed ShipError it must produce. Driving the real caller — rather than calling
// a classifier helper directly — is what keeps this a reachability proof: a
// classifier that exists but is never consulted leaves this RED.
func stageWithGitFailure(t *testing.T, porcelain, addStderr string, addExit int, addErr error) *core.ShipError {
	t.Helper()
	opts := &Options{
		ProjectRoot:   t.TempDir(),
		WorkspacePath: writeWorkspaceReports(t, shipmanifest.ChangedPaths(porcelain)...), // fresh workspace ⇒ this is always strike ONE
		Runner:        classifyStageRunner(porcelain, addStderr, addExit, addErr),
		Stderr:        io.Discard,
	}
	err := stageExplicitPaths(context.Background(), opts, &RunResult{}, "")
	if err == nil {
		t.Fatalf("a failing `git add` must produce a ship error, got nil")
	}
	se, ok := core.AsShipError(err)
	if !ok {
		t.Fatalf("staging failure must be a typed ShipError, got %T: %v", err, err)
	}
	if se.Code != core.CodeGitStageFailed {
		t.Fatalf("code = %q, want %q", se.Code, core.CodeGitStageFailed)
	}
	return se
}

// TestStageFailureClassification is the cycle's verifiableBy target
// (`go test ./internal/phases/ship -run 'TestStageFailureClassification'`). Each
// row is a real git stderr fixture; every row is strike ONE (fresh workspace),
// so any non-transient verdict here can only come from the stderr classifier and
// never from the pre-existing two-strikes memo.
func TestStageFailureClassification(t *testing.T) {
	const porcelain = " M docs/architecture/control-flags.md\n"

	cases := []struct {
		name    string
		stderr  string
		exit    int
		runErr  error
		want    core.ShipErrorClass
		because string
	}{
		{
			name:    "rc128_invalid_path_is_deterministic",
			stderr:  "fatal: Invalid path '/go/bin/evolve': No such file or directory\n",
			exit:    128,
			want:    core.ShipClassPrecondition,
			because: "cycle-1098: an absolute pathspec can never become valid on retry",
		},
		{
			name:    "rc128_outside_repository_is_deterministic",
			stderr:  "fatal: /tmp/elsewhere/x.md: '/tmp/elsewhere/x.md' is outside repository at '/repo'\n",
			exit:    128,
			want:    core.ShipClassPrecondition,
			because: "a path outside the repo is outside it on every attempt",
		},
		{
			name:    "rc128_pathspec_did_not_match_is_deterministic",
			stderr:  "fatal: pathspec 'docs/architecture/gone.md' did not match any files\n",
			exit:    128,
			want:    core.ShipClassPrecondition,
			because: "a missing pathspec cannot be conjured by a retry",
		},
		{
			name: "rc1_gitignore_advice_refusal_is_deterministic",
			stderr: "The following paths are ignored by one of your .gitignore files:\n" +
				"docs/architecture/control-flags.md\n" +
				"hint: Use -f if you really want to add them.\n",
			exit:    1,
			want:    core.ShipClassPrecondition,
			because: "cycle-1101: an ignore rule is deterministic; rc=1 advice-refusals are the second live instance",
		},
		{
			name:    "rc128_index_lock_contention_stays_transient",
			stderr:  "fatal: Unable to create '/repo/.git/index.lock': File exists.\n\nAnother git process seems to be running in this repository.\n",
			exit:    128,
			want:    core.ShipClassTransient,
			because: "index-lock contention is the canonical fleet-concurrency retry — reclassifying it would delete a retry that WINS",
		},
		{
			name:    "unrecognised_stderr_degrades_to_transient",
			stderr:  "error: something nobody has catalogued yet\n",
			exit:    128,
			want:    core.ShipClassTransient,
			because: "an unknown shape must fall back to today's behavior, never guess deterministic",
		},
		{
			name:    "empty_stderr_degrades_to_transient",
			stderr:  "",
			exit:    128,
			want:    core.ShipClassTransient,
			because: "no captured evidence ⇒ no classification",
		},
		{
			name:    "go_error_text_alone_does_not_classify",
			stderr:  "",
			exit:    0,
			runErr:  errors.New("fatal: Invalid path '/go/bin/evolve'"),
			want:    core.ShipClassTransient,
			because: "trust boundary: classify the CAPTURED git_stderr, never the message Go composes — an error-wrapper edit must not move recovery routes",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			se := stageWithGitFailure(t, porcelain, tc.stderr, tc.exit, tc.runErr)
			if se.Class != tc.want {
				t.Errorf("class = %q, want %q — %s\n  stderr fixture: %q", se.Class, tc.want, tc.because, tc.stderr)
			}
		})
	}
}

// TestStageFailureClassification_PreservesCapturedStderr pins the evidence half
// of the contract: whatever the class, the captured git stderr must still travel
// in Debug["git_stderr"]. That field is what the failure digest, retro, and
// escalation report read — and it is the classifier's own input, so a
// refactor that stops capturing it would silently disable classification.
func TestStageFailureClassification_PreservesCapturedStderr(t *testing.T) {
	const marker = "fatal: Invalid path '/go/bin/evolve'"
	se := stageWithGitFailure(t, " M docs/architecture/control-flags.md\n", marker+"\n", 128, nil)
	if got := se.Debug["git_stderr"]; !strings.Contains(got, marker) {
		t.Errorf("Debug[git_stderr] = %q, want it to contain %q — the classifier's input must stay observable", got, marker)
	}
	if got := se.Debug["git_rc"]; got != "128" {
		t.Errorf("Debug[git_rc] = %q, want \"128\"", got)
	}
}

// TestStageFailureClassification_TwoStrikesStillApplies is the regression
// guard for the router that already ships: an UNRECOGNISED refusal keeps its
// first retry and still escalates on the second consecutive attempt with the
// same pathspec. The new stderr classifier must sit in FRONT of that rule,
// not replace it.
func TestStageFailureClassification_TwoStrikesStillApplies(t *testing.T) {
	const porcelain = " M docs/architecture/control-flags.md\n"
	const unknown = "error: something nobody has catalogued yet\n"
	ws := writeWorkspaceReports(t, shipmanifest.ChangedPaths(porcelain)...)

	stage := func() *core.ShipError {
		opts := &Options{
			ProjectRoot:   t.TempDir(),
			WorkspacePath: ws,
			Runner:        classifyStageRunner(porcelain, unknown, 128, nil),
			Stderr:        io.Discard,
		}
		err := stageExplicitPaths(context.Background(), opts, &RunResult{}, "")
		se, ok := core.AsShipError(err)
		if !ok {
			t.Fatalf("want a typed ShipError, got %T: %v", err, err)
		}
		return se
	}

	if first := stage(); first.Class != core.ShipClassTransient {
		t.Fatalf("first unrecognised refusal class = %q, want %q", first.Class, core.ShipClassTransient)
	}
	if second := stage(); second.Class != core.ShipClassPrecondition {
		t.Errorf("second consecutive unrecognised refusal class = %q, want %q — the two-strikes router must survive the classifier",
			second.Class, core.ShipClassPrecondition)
	}
}
