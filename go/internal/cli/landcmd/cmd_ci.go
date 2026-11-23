package landcmd

import (
	"bufio"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/cmd/evolve/cmdutil"
	"github.com/mickeyyaya/evolve-loop/go/internal/ciwatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const (
	ciUsage = "usage: evolve ci classify <run-id|run:N|pr:N|sha:H> [--json] [--rerun] [--project-root P]\n" +
		"usage: evolve ci watch (--sha S | --pr N | --tag T) [--workflow W]... [--cycle N] [--project-root P]\n" +
		"usage: evolve ci darwin-set <generate|check> [--project-root P]\n" +
		"usage: evolve ci docs-tests [--project-root P]\n" +
		"usage: evolve ci telemetry --jobs FILE [--job-artifacts \"JOB NAME=DIR\"]... [--json]"
	ciClassifyPrefix = "evolve ci classify: "
	ciModuleDir      = "go"
	ciMainWindow     = 10
	ciClassifyHeader = "label\tpackage\ttest\trule\ttouched\tbase_red\trecurred_on_main\trerun_green"
)

var ciClassifyMarshalIndent = json.MarshalIndent

type ciClassifyArgs struct {
	target      ciwatch.Target
	asJSON      bool
	rerun       bool
	projectRoot string
}

func parseCIClassifyArgs(args []string) (ciClassifyArgs, error) {
	var a ciClassifyArgs
	operands, err := cmdutil.Flags().
		Bools(map[string]*bool{"--json": &a.asJSON, "--rerun": &a.rerun}).
		Values(map[string]*string{"--project-root": &a.projectRoot}).
		Parse(args)
	if err != nil {
		return a, err
	}
	if len(operands) != 1 {
		return a, fmt.Errorf("want exactly one target, got %d", len(operands))
	}
	a.target, err = ciwatch.ParseTarget(operands[0])
	return a, err
}

func RunCI(args []string, stdin io.Reader, stdout, stderr io.Writer, deps Deps) int {
	current.Store(&deps)
	return dispatchCI(args, stdin, stdout, stderr)
}

func dispatchCI(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		if args[0] == "docs-tests" {
			return runCIDocsTests(args[1:], stdin, stdout, stderr)
		}
		if run := ciSubverbRunner(args[0]); run != nil {
			return run(args[1:], stdout, stderr)
		}
	}
	if len(args) == 0 || args[0] != "classify" {
		fmt.Fprintln(stderr, ciUsage)
		return exitUsage
	}
	return runCIClassify(args[1:], stdout, stderr)
}

func runCIClassify(args []string, stdout, stderr io.Writer) int {
	a, err := parseCIClassifyArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n%s\n", ciClassifyPrefix, err, ciUsage)
		return exitUsage
	}
	root, err := resolveRoot(a.projectRoot, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n", ciClassifyPrefix, err)
		return exitIO
	}
	opts, err := ciClassifyOptions(root, a.rerun, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n", ciClassifyPrefix, err)
		return exitIO
	}
	rep, err := ciwatch.ClassifyTarget(context.Background(), ciwatch.NewGHClassifySource(root, opts.ModulePath), a.target, opts)
	switch {
	case errors.Is(err, ciwatch.ErrNoRun), errors.Is(err, ciwatch.ErrRunNotCompleted):
		fmt.Fprintf(stderr, "%s%v\n", ciClassifyPrefix, err)
		return exitRefused
	case err != nil:
		fmt.Fprintf(stderr, "%s%v\n", ciClassifyPrefix, err)
		return exitIO
	}
	writeClassifyWarnings(stderr, ciClassifyPrefix, rep)
	if err := writeClassifyReport(stdout, rep, a.asJSON); err != nil {
		fmt.Fprintf(stderr, "%s%v\n", ciClassifyPrefix, err)
		return exitIO
	}
	if err := fileBaseRedItems(root, rep, stderr); err != nil {
		fmt.Fprintf(stderr, "%s%v\n", ciClassifyPrefix, err)
		return exitIO
	}
	if rep.RetrySafe {
		return 0
	}
	return exitRefused
}

func ciSubverbRunner(name string) func([]string, io.Writer, io.Writer) int {
	switch name {
	case "watch":
		return runCIWatch
	case "telemetry":
		return runCITelemetry
	case "darwin-set":
		return host().DarwinSet
	}
	return nil
}

func fileBaseRedItems(root string, rep ciwatch.ClassifyReport, stderr io.Writer) error {
	filed, err := ciwatch.FileBaseRedItems(filepath.Join(paths.EvolveDirOf(root), "inbox"), rep, time.Now())
	for _, id := range filed {
		fmt.Fprintf(stderr, "%sfiled fix-first item %s\n", ciClassifyPrefix, id)
	}
	return err
}

func ciClassifyOptions(root string, rerun bool, stderr io.Writer) (ciwatch.ClassifyOptions, error) {
	pol, err := policy.Load(paths.PolicyPath(paths.EvolveDirOf(root)))
	if err != nil {
		return ciwatch.ClassifyOptions{}, err
	}
	cw, err := pol.CIWatchConfig()
	if err != nil {
		return ciwatch.ClassifyOptions{}, err
	}
	modulePath, err := goModulePath(filepath.Join(root, ciModuleDir, "go.mod"))
	if err != nil {
		fmt.Fprintf(stderr, "%sWARN: %v; every touched fact is unknown\n", ciClassifyPrefix, err)
	}
	return ciwatch.ClassifyOptions{
		Rerun: rerun, RerunTimeout: time.Duration(*cw.TimeoutS) * time.Second, Poll: time.Duration(*cw.PollS) * time.Second,
		MainWindow: ciMainWindow, ModulePath: modulePath, ModuleDir: ciModuleDir,
	}, nil
}

func goModulePath(goMod string) (string, error) {
	f, err := os.Open(goMod)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if fields := strings.Fields(sc.Text()); len(fields) == 2 && fields[0] == "module" {
			return strings.Trim(fields[1], `"`), nil
		}
	}
	if err := sc.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("%s has no module line", goMod)
}

func writeClassifyReport(w io.Writer, rep ciwatch.ClassifyReport, asJSON bool) error {
	if asJSON {
		raw, err := ciClassifyMarshalIndent(rep, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(w, "%s\n", raw)
		return err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "run %d %s head=%s base=%s conclusion=%s\n", rep.RunID, rep.RunURL, shortSHA(rep.HeadSHA), shortSHA(rep.BaseSHA), rep.Conclusion)
	fmt.Fprintln(&b, ciClassifyHeader)
	for _, f := range rep.Failures {
		e := f.Evidence
		fmt.Fprintf(&b, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", f.Label, orDash(f.Package), orDash(failureLabel(f.FailingTest)), f.Rule,
			e.Touched, e.BaseRed, e.RecurredOnMain, e.RerunGreen)
	}
	fmt.Fprintf(&b, "retry-safe: %s\n", map[bool]string{true: "yes", false: "no"}[rep.RetrySafe])
	_, err := io.WriteString(w, b.String())
	return err
}

func failureLabel(f ciwatch.FailingTest) string {
	label := cmp.Or(f.Test, f.Step)
	if f.Detail == "" {
		return label
	}
	return label + ": " + f.Detail
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func writeClassifyWarnings(w io.Writer, prefix string, rep ciwatch.ClassifyReport) {
	for _, warning := range rep.Warnings {
		fmt.Fprintf(w, "%sWARN: %s\n", prefix, warning)
	}
}
