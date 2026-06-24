// cmd_release_verify_clis.go implements `evolve release-verify-clis`: the
// release-flow gate that proves the release artifact can be INSTALLED for every
// supported LLM CLI and that the release BINARY answers the core subcommands the
// installed skills shell out to.
//
// Why this exists: `evolve release` (releasepipeline) proves only LOCAL binary
// consistency — disk == committed blob == expected_ship_sha. "Every CLI installs
// and performs" lived only as prose in skills/publish/SKILL.md, which is
// non-deterministic (v21.1.0 shipped with the prose green yet published zero
// assets). This command makes that closing check deterministic Go.
//
// Determinism: no live LLM, no network. Each CLI is verified by exercising the
// real install/projection path into an isolated location, and the binary is
// smoke-checked with a side-effect-free `<bin> <sub> --help`. The effects are
// injected via matrixDeps so the orchestration (coverage, isolation, no early
// return) is unit-testable with pure stubs; defaultMatrixDeps() wires the real
// implementations used by the release flow.
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/installer"
)

// cliVerify is one row of the release verification matrix: a target (an LLM CLI,
// or the binaryRowKey pseudo-target) and whether it passed, with a human Detail.
type cliVerify struct {
	CLI    string
	OK     bool
	Detail string
}

// matrixDeps are the injectable effects the matrix performs. Tests stub these to
// drive every branch deterministically; defaultMatrixDeps() supplies the real
// install/projection/exec implementations.
type matrixDeps struct {
	// installClaude installs the Claude payload from srcDir into an isolated
	// home and reports a non-nil error if the install is empty or fails.
	installClaude func(srcDir, home string) error
	// projectTarget renders the skill projection for a codex/agy target from
	// srcDir, reporting a non-nil error if the projection fails.
	projectTarget func(srcDir, target string) error
	// geminiLayout asserts srcDir contains the payload Gemini discovers in-repo.
	geminiLayout func(srcDir string) error
	// binAnswers reports a non-nil error if the release binary at binPath does
	// not answer subcommand sub.
	binAnswers func(binPath, sub string) error
}

// releaseVerifyCLIs is the closed set of LLM CLIs the release must support. The
// matrix iterates this list, so coverage is driven by it.
var releaseVerifyCLIs = []string{"claude", "codex", "agy", "gemini"}

// binaryRowKey labels the single matrix row that verifies the release binary
// answers every core subcommand (distinct from the per-CLI install rows).
const binaryRowKey = "binary:core-subcommands"

// coreSubcommands are the subcommands the installed skills shell out to. Every
// name here MUST be a registered command (TestCoreSubcommandsRegistered guards
// the SSOT against drift). If the release binary stopped answering any of these,
// the installed skills would silently break — that is the regression this gate
// catches.
var coreSubcommands = []string{
	"loop",
	"subagent",
	"serve-phase",
	"guard",
	"ship",
	"doctor",
	"install",
	"skills",
	"release",
}

// verifyReleaseCLIMatrix runs the full matrix: one row per supported CLI (each
// install/projection isolated from the others) plus one binary row across all
// core subcommands. It never returns early — every failure is reported so the
// operator sees the complete picture, not just the first fault.
func verifyReleaseCLIMatrix(srcDir, binPath string, d matrixDeps) []cliVerify {
	results := make([]cliVerify, 0, len(releaseVerifyCLIs)+1)
	for _, cli := range releaseVerifyCLIs {
		var err error
		switch cli {
		case "claude":
			err = verifyClaudeInstall(srcDir, d.installClaude)
		case "codex", "agy":
			err = d.projectTarget(srcDir, cli)
		case "gemini":
			err = d.geminiLayout(srcDir)
		}
		results = append(results, rowFor(cli, err))
	}

	var missing []string
	for _, sub := range coreSubcommands {
		if err := d.binAnswers(binPath, sub); err != nil {
			missing = append(missing, sub)
		}
	}
	results = append(results, binaryRow(missing))
	return results
}

// verifyClaudeInstall isolates the Claude install in a throwaway home so the
// matrix never touches the operator's real ~/.claude.
func verifyClaudeInstall(srcDir string, install func(srcDir, home string) error) error {
	home, err := os.MkdirTemp("", "evolve-relverify-claude-")
	if err != nil {
		return fmt.Errorf("create isolated home: %w", err)
	}
	defer func() { _ = os.RemoveAll(home) }()
	return install(srcDir, home)
}

// rowFor builds a per-CLI row from the verification error (nil == OK).
func rowFor(cli string, err error) cliVerify {
	if err != nil {
		return cliVerify{CLI: cli, OK: false, Detail: err.Error()}
	}
	return cliVerify{CLI: cli, OK: true, Detail: "install/projection verified"}
}

// binaryRow builds the single binary row, naming any unanswered subcommands so a
// failure is actionable.
func binaryRow(missing []string) cliVerify {
	if len(missing) > 0 {
		return cliVerify{
			CLI:    binaryRowKey,
			OK:     false,
			Detail: "unanswered subcommands: " + strings.Join(missing, ", "),
		}
	}
	return cliVerify{
		CLI:    binaryRowKey,
		OK:     true,
		Detail: fmt.Sprintf("%d core subcommands answered", len(coreSubcommands)),
	}
}

// defaultMatrixDeps wires the real install/projection/exec effects used by the
// release flow.
func defaultMatrixDeps() matrixDeps {
	return matrixDeps{
		installClaude: func(srcDir, home string) error {
			res, err := installer.Install(srcDir, home, io.Discard)
			if err != nil {
				return fmt.Errorf("claude install: %w", err)
			}
			if res.Agents == 0 || res.Skills == 0 {
				return fmt.Errorf("empty install (agents=%d skills=%d)", res.Agents, res.Skills)
			}
			return nil
		},
		projectTarget: func(srcDir, target string) error {
			// DryRun renders the projection without shelling out to codex/agy,
			// so this stays deterministic and side-effect-free.
			rc := runSkillsPublish(srcDir, publishConfig{
				Targets: []string{target},
				DryRun:  true,
			}, io.Discard, io.Discard)
			if rc != 0 {
				return fmt.Errorf("projection failed (rc=%d)", rc)
			}
			return nil
		},
		geminiLayout: func(srcDir string) error {
			// Gemini discovers skills in-repo; assert the canonical payload
			// sources exist.
			payloadPaths := []string{"skills", filepath.Join(".claude-plugin", "plugin.json")}
			for _, rel := range payloadPaths {
				if _, err := os.Stat(filepath.Join(srcDir, rel)); err != nil {
					return fmt.Errorf("payload missing %s: %w", rel, err)
				}
			}
			return nil
		},
		binAnswers: func(binPath, sub string) error {
			// Signal is the dispatcher's `unknown command "<name>"` stderr
			// message (main.go), NOT the exit code: a registered command is
			// dispatched to its handler, which may reject --help with its own
			// non-zero exit (10/1/…) yet is still wired. --help is the safest
			// probe arg (flag-parsing handlers reject it before doing real
			// work); the timeout guards a handler that hangs on startup.
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, binPath, sub, "--help")
			var stderr bytes.Buffer
			cmd.Stdout = io.Discard
			cmd.Stderr = &stderr
			// A start failure (binary missing/not executable) means the smoke
			// never ran — fail loudly rather than silently passing. Exit-code
			// errors from a dispatched handler are *exec.ExitError and expected.
			if err := cmd.Run(); err != nil {
				var exitErr *exec.ExitError
				if !errors.As(err, &exitErr) {
					return fmt.Errorf("run %q %s: %w", binPath, sub, err)
				}
			}
			if strings.Contains(stderr.String(), fmt.Sprintf("unknown command %q", sub)) {
				return fmt.Errorf("binary does not recognize subcommand %q", sub)
			}
			return nil
		},
	}
}

// runReleaseVerifyCLIs is the `evolve release-verify-clis` handler: it runs the
// matrix against the in-repo payload (sourceRoot) and the running binary
// (os.Executable), prints a table, and exits non-zero if any row failed.
func runReleaseVerifyCLIs(_ []string, _ io.Reader, stdout, stderr io.Writer) int {
	srcDir := sourceRoot()
	binPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(stderr, "resolve release binary path: %v\n", err)
		return 1
	}

	results := verifyReleaseCLIMatrix(srcDir, binPath, defaultMatrixDeps())

	allOK := true
	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "TARGET\tSTATUS\tDETAIL")
	for _, r := range results {
		status := "OK"
		if !r.OK {
			status = "FAIL"
			allOK = false
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", r.CLI, status, r.Detail)
	}
	_ = tw.Flush()

	if !allOK {
		fmt.Fprintln(stderr, "release-verify-clis: one or more targets failed install/perform verification")
		return 1
	}
	fmt.Fprintln(stdout, "release-verify-clis: all CLIs install and the binary answers all core subcommands")
	return 0
}
