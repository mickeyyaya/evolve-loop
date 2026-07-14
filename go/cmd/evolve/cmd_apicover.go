// cmd_apicover.go routes `evolve apicover` to internal/apicover.Main — the same
// measurement entry the standalone cmd/apicover binary and the audit's
// in-process API-gate call. Folding apicover into the evolve binary means a
// deployed cycle runs the API-coverage gate WITHOUT a `go build -o bin/apicover`
// at runtime (one-binary S1): no second first-party executable, no runtime
// rebuild.
package main

import (
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/apicover"
)

func runApicover(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return apicover.Main(args, stdout, stderr)
}
