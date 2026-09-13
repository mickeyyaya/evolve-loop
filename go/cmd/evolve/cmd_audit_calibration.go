package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/auditcalibration"
)

func runAudit(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "evolve audit: expected subcommand: calibration")
		return 2
	}
	if args[0] != "calibration" {
		fmt.Fprintf(stderr, "evolve audit: unknown subcommand %q (want calibration)\n", args[0])
		return 2
	}
	return runAuditCalibration(args[1:], stdout, stderr)
}

func runAuditCalibration(args []string, _ io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("audit calibration", flag.ContinueOnError)
	fs.SetOutput(stderr)
	projectRoot := fs.String("project-root", "", "project root used for default corpus paths")
	dossiersDir := fs.String("dossiers-dir", "", "directory containing cycle dossiers")
	runsDir := fs.String("runs-dir", "", "directory containing cycle run artifacts")
	output := fs.String("output", "", "Markdown report output path (required)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "evolve audit calibration: unexpected argument %q\n", fs.Arg(0))
		return 2
	}
	if *output == "" {
		fmt.Fprintln(stderr, "evolve audit calibration: --output is required")
		return 2
	}
	root := *projectRoot
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			fmt.Fprintf(stderr, "evolve audit calibration: project root: %v\n", err)
			return 1
		}
	}
	if *dossiersDir == "" {
		*dossiersDir = filepath.Join(root, "knowledge-base", "cycles")
	}
	if *runsDir == "" {
		*runsDir = filepath.Join(root, ".evolve", "runs")
	}
	if err := requireOutputParent(*output); err != nil {
		fmt.Fprintf(stderr, "evolve audit calibration: %v\n", err)
		return 1
	}
	report, err := auditcalibration.Generate(*dossiersDir, *runsDir)
	if err != nil {
		fmt.Fprintf(stderr, "evolve audit calibration: %v\n", err)
		return 1
	}
	if err := atomicwrite.Bytes(*output, report); err != nil {
		fmt.Fprintf(stderr, "evolve audit calibration: output %q: %v\n", *output, err)
		return 1
	}
	return 0
}

func requireOutputParent(path string) error {
	dir := filepath.Dir(path)
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("output directory %q: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("output directory %q is not a directory", dir)
	}
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return fmt.Errorf("output %q is a directory", path)
	}
	return nil
}
