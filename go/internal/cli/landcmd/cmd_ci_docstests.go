package landcmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/cmd/evolve/cmdutil"
	"github.com/mickeyyaya/evolve-loop/go/internal/docstests"
)

const (
	usageCIDocsTests = "usage: evolve ci docs-tests [--project-root P]\n" +
		"  reads NUL-separated changed paths from stdin and prints <package>\\t<regexp> lines"
	ciDocsTestsPrefix = "evolve ci docs-tests: "
)

func runCIDocsTests(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	var projectRoot string
	var help bool
	operands, err := cmdutil.Flags().
		Bools(map[string]*bool{"--help": &help, "-h": &help}).
		Values(map[string]*string{"--project-root": &projectRoot}).
		Parse(args)
	if help && err == nil && len(operands) == 0 {
		fmt.Fprintln(stdout, usageCIDocsTests)
		return exitOK
	}
	if err == nil && len(operands) > 0 {
		err = fmt.Errorf("unexpected operand %q", operands[0])
	}
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n%s\n", ciDocsTestsPrefix, err, usageCIDocsTests)
		return exitUsage
	}
	changed, err := readNULPaths(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n", ciDocsTestsPrefix, err)
		return exitIO
	}
	root, err := resolveRoot(projectRoot, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n", ciDocsTestsPrefix, err)
		return exitIO
	}
	selections, err := docstests.Select(root, changed)
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n", ciDocsTestsPrefix, err)
		return exitIO
	}
	if _, err := io.WriteString(stdout, docstests.Format(selections)); err != nil {
		fmt.Fprintf(stderr, "%s%v\n", ciDocsTestsPrefix, err)
		return exitIO
	}
	return exitOK
}

func readNULPaths(r io.Reader) ([]string, error) {
	if r == nil {
		return nil, nil
	}
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, field := range strings.Split(string(raw), "\x00") {
		if field != "" {
			paths = append(paths, field)
		}
	}
	return paths, nil
}
