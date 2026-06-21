// Command serve is a tiny static file server for previewing the built site.
// Usage: go run ./cmd/serve [dir] [addr]   (defaults: dist 127.0.0.1:8077)
package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

// parseArgs resolves the directory and listen address from positional args,
// applying the defaults (dist, 127.0.0.1:8077) when not overridden.
func parseArgs(args []string) (dir, addr string) {
	dir = "dist"
	addr = "127.0.0.1:8077"
	if len(args) > 0 {
		dir = args[0]
	}
	if len(args) > 1 {
		addr = args[1]
	}
	return dir, addr
}

// run parses args, logs the serving line, then starts serve. It returns 1 on
// error (also logging it) and 0 otherwise.
func run(args []string, log io.Writer, serve func(addr string, h http.Handler) error) int {
	dir, addr := parseArgs(args)
	fmt.Fprintf(log, "serving %s on http://%s\n", dir, addr)
	if err := serve(addr, http.FileServer(http.Dir(dir))); err != nil {
		fmt.Fprintln(log, err)
		return 1
	}
	return 0
}

// osExit and serveFn are seams so a test can verify main()'s wiring without
// binding a port or terminating the process. At runtime they are exactly
// os.Exit and http.ListenAndServe.
var (
	osExit  = os.Exit
	serveFn = http.ListenAndServe
)

func main() {
	osExit(run(os.Args[1:], os.Stdout, serveFn))
}
