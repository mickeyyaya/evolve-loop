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
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type cliVerify struct {
	CLI    string
	OK     bool
	Detail string
}

type matrixDeps struct {
	installClaude func(srcDir, home string) error
	projectTarget func(srcDir, target string) error
	geminiLayout  func(srcDir string) error
	binAnswers    func(binPath, sub string) error
}

var releaseVerifyCLIs = []string{"claude", "codex", "agy", "gemini"}

const binaryRowKey = "binary:core-subcommands"

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

func verifyClaudeInstall(srcDir string, install func(srcDir, home string) error) error {
	home, err := os.MkdirTemp("", "evolve-relverify-claude-")
	if err != nil {
		return fmt.Errorf("create isolated home: %w", err)
	}
	defer func() { _ = os.RemoveAll(home) }()
	return install(srcDir, home)
}

func rowFor(cli string, err error) cliVerify {
	if err != nil {
		return cliVerify{CLI: cli, OK: false, Detail: err.Error()}
	}
	return cliVerify{CLI: cli, OK: true, Detail: "install/projection verified"}
}

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

func installClaudeDep(srcDir, home string) error {
	res, err := installer.Install(srcDir, home, io.Discard)
	if err != nil {
		return fmt.Errorf("claude install: %w", err)
	}
	if res.Agents == 0 || res.Skills == 0 {
		return fmt.Errorf("empty install (agents=%d skills=%d)", res.Agents, res.Skills)
	}
	return nil
}

func projectTargetDryRunDep(srcDir, target string) error {
	rc := runSkillsPublish(srcDir, publishConfig{
		Targets: []string{target},
		DryRun:  true,
	}, io.Discard, io.Discard)
	if rc != 0 {
		return fmt.Errorf("projection failed (rc=%d)", rc)
	}
	return nil
}

func assertGeminiPayloadPresentDep(srcDir string) error {
	payloadPaths := []string{"skills", filepath.Join(".claude-plugin", "plugin.json")}
	for _, rel := range payloadPaths {
		if _, err := os.Stat(filepath.Join(srcDir, rel)); err != nil {
			return fmt.Errorf("payload missing %s: %w", rel, err)
		}
	}
	return nil
}

func binaryAnswersSubcommandDep(binPath, sub string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := sysexec.Command(ctx, binPath, sub, "--help")
	var stderr bytes.Buffer
	cmd.Stdout = io.Discard
	cmd.Stderr = &stderr
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
}

func defaultMatrixDeps() matrixDeps {
	return matrixDeps{
		installClaude: installClaudeDep,
		projectTarget: projectTargetDryRunDep,
		geminiLayout:  assertGeminiPayloadPresentDep,
		binAnswers:    binaryAnswersSubcommandDep,
	}
}

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
