package guardcmd

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/commitgate"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/rootwire"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func RunCommitGate(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "evolve commit-gate: usage: commit-gate run --reviewers \"<csv>\" [--files \"p1 p2\"] [--no-install] [--project-root P] [--message M]")
		return commitgate.ExitBadArgs
	}
	sub, rest := args[0], args[1:]
	if sub != "run" {
		fmt.Fprintf(stderr, "evolve commit-gate: unknown subcommand %q (want: run)\n", sub)
		return commitgate.ExitBadArgs
	}

	fs := flag.NewFlagSet("commit-gate run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		reviewers   = fs.String("reviewers", "", "CSV of reviewers that ran (simplify + one review capability)")
		files       = fs.String("files", "", "override change detection: whitespace-separated path list")
		noInstall   = fs.Bool("no-install", false, "do not auto-install missing tools (hard-fail instead)")
		projectRoot = fs.String("project-root", "", "git working tree root (default: git rev-parse --show-toplevel)")
		message     = fs.String("message", "", "the commit message; at regression_proof.stage shadow a fix: subject with no valid Fix-Journey trailer is reported")
	)
	if err := fs.Parse(rest); err != nil {
		return commitgate.ExitBadArgs
	}

	runner := sysexec.DefaultRunner
	root, code := commitGateRoot(*projectRoot, runner, stderr)
	if code != commitgate.ExitPass {
		return code
	}
	signals, closeSignals := commitGateSignals(root, stderr)
	defer closeSignals()
	opts := commitgate.Options{
		RepoRoot:      root,
		Reviewers:     *reviewers,
		Files:         *files,
		NoInstall:     *noInstall,
		AttestDir:     os.Getenv("CG_ATTEST_DIR"),
		Env:           os.Environ(),
		Runner:        runner,
		Now:           time.Now,
		Message:       *message,
		FixProofStage: fixProofStage(root, stderr),
		Signals:       signals,
	}
	return printCommitGateResult(opts.Run(context.Background()), stdout, stderr)
}

func fixProofStage(root string, stderr io.Writer) config.Stage {
	stage, err := policy.RegressionProofStageFor(root)
	if err != nil {
		fmt.Fprintf(stderr, "[commit-gate] WARN: %v; the fix-proof check keeps the stage %s\n", err, stage)
	}
	return stage
}

func commitGateSignals(root string, stderr io.Writer) (*signalcenter.Center, func()) {
	c := signalcenter.New(signalcenter.WithPID(os.Getpid()))
	return c, rootwire.Attach(c, "commit-gate", filepath.Join(root, ".evolve"), stderr)
}

func commitGateRoot(projectRoot string, runner sysexec.RunFunc, stderr io.Writer) (string, int) {
	if projectRoot != "" {
		if abs, err := filepath.Abs(projectRoot); err == nil {
			projectRoot = abs
		}
		return projectRoot, commitgate.ExitPass
	}
	out, err := sysexec.Output(context.Background(), runner, "", "git", "rev-parse", "--show-toplevel")
	if err != nil {
		fmt.Fprintln(stderr, "[commit-gate] not in a git repo")
		return "", commitgate.ExitGitFatal
	}
	return out, commitgate.ExitPass
}

func printCommitGateResult(res *commitgate.Result, stdout, stderr io.Writer) int {
	for _, line := range res.Logs {
		fmt.Fprintln(stderr, line)
	}
	if res.ExitCode == commitgate.ExitPass && res.Attestation != nil {
		fmt.Fprintf(stdout, "%s\n", res.Attestation.TreeStateSHA)
	}
	return res.ExitCode
}
