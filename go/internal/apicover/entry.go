// Package apicover measures the public-API coverage of a package: the exported symbols that no test names and the named ones that never run.
// See docs/architecture/packages/internal-apicover.md.
package apicover

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
)

func Main(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("apicover", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cover := fs.String("cover", "", "path to `go tool cover -func` output")
	requireDoc := fs.Bool("require-doc", false, "flag exported decls missing a godoc comment")
	enforce := fs.Bool("enforce", false, "exit non-zero when uncovered/false-green symbols exist")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	code, err := Run(context.Background(), Config{
		Dirs:       fs.Args(),
		CoverPath:  *cover,
		RequireDoc: *requireDoc,
		Enforce:    *enforce,
	}, stdout)
	if err != nil {
		fmt.Fprintln(stderr, "apicover:", err)
		return 2
	}
	return code
}
