package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/reviewclass"
)

const (
	reviewClassUsage  = "usage: evolve review-class [--base B] [--json] [--project-root P]"
	reviewClassPrefix = "evolve review-class: "
	reviewClassBase   = "HEAD"
)

var reviewClassRoot = resolveRepoRoot

type reviewClassArgs struct {
	base, projectRoot string
	json, help        bool
}

type reviewClassData struct {
	ProjectRoot string   `json:"project_root"`
	Base        string   `json:"base"`
	Paths       []string `json:"paths"`
	reviewclass.Requirements
}

func runReviewClass(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	a, err := parseReviewClassArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n%s\n", reviewClassPrefix, err, reviewClassUsage)
		return exitUsage
	}
	if a.help {
		fmt.Fprintln(stdout, reviewClassUsage)
		return 0
	}
	root, err := reviewClassRoot(a.projectRoot)
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n", reviewClassPrefix, err)
		return exitIO
	}
	d, err := reviewclass.Read(reviewClassGit(root), func(p string) ([]byte, error) { return os.ReadFile(filepath.Join(root, p)) }, a.base)
	if err != nil {
		fmt.Fprintf(stderr, "%sread the diff against %s: %v\n", reviewClassPrefix, a.base, err)
		return exitIO
	}
	data := reviewClassData{ProjectRoot: root, Base: a.base, Paths: append([]string{}, d.Paths...), Requirements: reviewclass.Classify(d)}
	if a.json {
		return printVerbJSON("review-class", data, stdout, stderr)
	}
	writeReviewClassText(stdout, data)
	return 0
}

func parseReviewClassArgs(args []string) (reviewClassArgs, error) {
	a := reviewClassArgs{base: reviewClassBase}
	operands, err := cliFlags{
		bools:  map[string]*bool{"--json": &a.json, "--help": &a.help, "-h": &a.help},
		values: map[string]*string{"--base": &a.base, "--project-root": &a.projectRoot},
	}.parse(args)
	switch {
	case err != nil:
		return a, err
	case len(operands) > 0:
		return a, fmt.Errorf("unexpected operand %q", operands[0])
	case a.base == "" || strings.HasPrefix(a.base, "-"):
		return a, fmt.Errorf("--base needs a revision, got %q", a.base)
	}
	return a, nil
}

func reviewClassGit(root string) reviewclass.Git {
	return func(args ...string) (string, error) {
		out, errOut, code, err := gitexec.Default(root).Capture(context.Background(), args...)
		switch {
		case err != nil:
			return "", err
		case code != 0:
			return "", fmt.Errorf("git %s: exit %d: %s", strings.Join(args, " "), code, strings.TrimSpace(errOut))
		}
		return out, nil
	}
}

func plannedNote(planned string) string {
	if planned == "" {
		return ""
	}
	return " (planned " + planned + ")"
}

func writeReviewClassText(w io.Writer, data reviewClassData) {
	fmt.Fprintf(w, "review-class: base %s, %d path(s), %d class(es)\n", data.Base, len(data.Paths), len(data.Classes))
	fmt.Fprintln(w, "classes:")
	for _, h := range data.Classes {
		fmt.Fprintf(w, "  %s %s: %s\n", h.Class, h.Name, strings.Join(h.Paths, ", "))
	}
	fmt.Fprintln(w, "seats:")
	for _, s := range data.Seats {
		fmt.Fprintf(w, "  %s%s\n", s, plannedNote(s.Planned))
	}
	fmt.Fprintln(w, "checks:")
	for _, c := range data.Checks {
		fmt.Fprintf(w, "  %s: %s%s\n", c.ID, c.Rule, plannedNote(c.Planned))
	}
}
