// Command apicover measures public-API coverage: it enumerates every exported
// symbol of a package (go/ast, stdlib only) and applies a two-signal check —
// named by a _test AST AND >0% in `go tool cover -func` — flagging uncovered
// symbols and named-but-0% false-greens. //apicover:ignore reason=... suppresses
// a symbol (reason mandatory). It is warning-only by default; -enforce makes it
// exit non-zero. See docs/architecture/adr/0050 and the decision log.
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	cover := flag.String("cover", "", "path to `go tool cover -func` output")
	requireDoc := flag.Bool("require-doc", false, "flag exported decls missing a godoc comment")
	enforce := flag.Bool("enforce", false, "exit non-zero when uncovered/false-green symbols exist")
	flag.Parse()

	code, err := Run(Config{
		Dirs:       flag.Args(),
		CoverPath:  *cover,
		RequireDoc: *requireDoc,
		Enforce:    *enforce,
	}, os.Stdout)
	if err != nil {
		fmt.Fprintln(os.Stderr, "apicover:", err)
		os.Exit(2)
	}
	os.Exit(code)
}
