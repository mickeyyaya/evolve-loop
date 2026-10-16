package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/installer"
)

func runInstall(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	ci := installModeCI(args)
	for _, a := range args {
		if a == "--help" || a == "-h" {
			fmt.Fprintln(stdout, "Usage: evolve install [--ci]")
			fmt.Fprintln(stdout, "  --ci   Validate plugin structure only (no copying); exits 1 on any failure.")
			fmt.Fprintln(stdout, "Without --ci, copies evolve-* agents + the loop skill into ~/.claude.")
			return 0
		}
		if strings.HasPrefix(a, "--") && a != "--ci" {
			fmt.Fprintf(stderr, "[install] unknown flag: %s\n", a)
			return 10
		}
	}

	srcDir := sourceRoot()

	if ci {
		res := installer.Validate(srcDir, stdout)
		if res.Errors > 0 {
			return 1
		}
		return 0
	}

	homeDir, err := os.UserHomeDir()
	if err != nil || homeDir == "" {
		fmt.Fprintf(stderr, "[install] cannot resolve home directory: %v\n", err)
		return 1
	}

	return installForPresentCLIs(srcDir, homeDir, stdin, stdout, stderr)
}

var installLookPath = exec.LookPath

var supportedInstallCLIs = []string{"claude", "codex", "agy", "gemini"}

func presentCLIs() []string {
	var out []string
	for _, c := range supportedInstallCLIs {
		if _, err := installLookPath(c); err == nil {
			out = append(out, c)
		}
	}
	return out
}

func installForPresentCLIs(srcDir, homeDir string, stdin io.Reader, stdout, stderr io.Writer) int {
	clis := presentCLIs()
	if len(clis) == 0 {
		return installClaude(srcDir, homeDir, stdin, stdout, stderr)
	}
	failed := 0
	for _, c := range clis {
		switch c {
		case "claude":
			if installClaude(srcDir, homeDir, stdin, stdout, stderr) != 0 {
				failed++
			}
		case "codex":
			if runSkillsPublish(srcDir, publishConfig{Targets: []string{"codex"}, Install: true, Prune: true, CodexHome: defaultCodexHome()}, stdout, stderr) != 0 {
				failed++
			}
		case "agy":
			if runSkillsPublish(srcDir, publishConfig{Targets: []string{"agy"}, Install: true, Prune: true}, stdout, stderr) != 0 {
				failed++
			}
		case "gemini":
			fmt.Fprintln(stdout, "[install] gemini: skills auto-discover from .agents/skills/ — run gemini from a repo checkout.")
		}
	}
	if failed > 0 {
		return 1
	}
	return 0
}

func installClaude(srcDir, homeDir string, stdin io.Reader, stdout, stderr io.Writer) int {
	if installer.PluginAlreadyInstalled(homeDir) {
		fmt.Fprintln(stdout, "WARNING: evo is already installed as a plugin.")
		fmt.Fprintln(stdout, "Manual install will create DUPLICATES (/evo:loop will appear twice).")
		fmt.Fprintln(stdout, "")
		fmt.Fprintln(stdout, "To upgrade the plugin version instead, run in your AI CLI:")
		fmt.Fprintln(stdout, "  /plugin marketplace update evo")
		fmt.Fprintln(stdout, "  /plugin update evo@evo")
		fmt.Fprintln(stdout, "  /plugin reload")
		fmt.Fprintln(stdout, "")
		fmt.Fprint(stdout, "Continue with manual install anyway? [y/N] ")
		if !confirmYes(stdin) {
			fmt.Fprintln(stdout, "Aborted. Use plugin commands above to upgrade.")
			return 0
		}
	}

	fmt.Fprintf(stdout, "Installing Evolve Loop %s...\n", installer.Version)
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "NOTE: Preferred method is plugin install:")
	fmt.Fprintln(stdout, "  /plugin marketplace add mickeyyaya/evolve-loop")
	fmt.Fprintln(stdout, "  /plugin install evo@evo")
	fmt.Fprintln(stdout, "")

	res, err := installer.Install(srcDir, homeDir, stdout)
	if err != nil {
		fmt.Fprintf(stderr, "[install] FAIL: %v\n", err)
		return 1
	}

	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "Installation complete!")
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "Installed:")
	fmt.Fprintf(stdout, "  - %d agents (Scout, Builder, Auditor, Operator)\n", res.Agents)
	fmt.Fprintf(stdout, "  - %d skill files\n", res.Skills)
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, installer.UsageLine)
	return 0
}

func runUninstall(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	ci := installModeCI(args)
	for _, a := range args {
		if a == "--help" || a == "-h" {
			fmt.Fprintln(stdout, "Usage: evolve uninstall [--ci]")
			fmt.Fprintln(stdout, "  --ci   Dry-run: list what would be removed (no deletions).")
			return 0
		}
		if strings.HasPrefix(a, "--") && a != "--ci" {
			fmt.Fprintf(stderr, "[uninstall] unknown flag: %s\n", a)
			return 10
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil || homeDir == "" {
		fmt.Fprintf(stderr, "[uninstall] cannot resolve home directory: %v\n", err)
		return 1
	}

	if ci {
		installer.UninstallDryRun(homeDir, stdout)
		return 0
	}

	fmt.Fprintln(stdout, "Uninstalling Evolve Loop...")
	fmt.Fprintln(stdout, "")
	if _, err := installer.Uninstall(homeDir, stdout); err != nil {
		fmt.Fprintf(stderr, "[uninstall] FAIL: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "Uninstallation complete.")
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "Note: Project workspace files (.evolve/) are NOT removed.")
	fmt.Fprintln(stdout, "Delete them manually if you no longer need cycle history.")
	return 0
}

func installModeCI(args []string) bool {
	for _, a := range args {
		if a == "--ci" {
			return true
		}
	}
	return strings.EqualFold(os.Getenv("CI"), "true")
}

func confirmYes(r io.Reader) bool {
	if r == nil {
		return false
	}
	sc := bufio.NewScanner(r)
	if !sc.Scan() {
		return false
	}
	resp := strings.TrimSpace(sc.Text())
	return resp == "y" || resp == "Y"
}
