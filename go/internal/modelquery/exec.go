package modelquery

import (
	"context"
	"os/exec"
	"strings"
)

// Runner executes a command and returns its combined stdout+stderr. Combined
// output is deliberate: the CLI classifiers (e.g. `codex exec`) frame the
// model's reply with header/footer lines whose stream is unspecified, and the
// JSON extractor tolerates surrounding noise. Injectable so listers and the
// classifier are unit-testable without shelling out.
type Runner func(ctx context.Context, name string, args []string, stdin string) (string, error)

// defaultRunner is the production Runner.
func defaultRunner(ctx context.Context, name string, args []string, stdin string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// Router dispatches List calls to a per-CLI Lister strategy (ollama uses a
// non-interactive listing; the rest drive the REPL /model picker). cli names
// are expected to be base names already (claude|codex|agy|ollama).
type Router struct {
	ByCLI   map[string]Lister
	Default Lister
}

// List routes to the CLI's strategy, falling back to Default.
func (r Router) List(ctx context.Context, cli string) ([]string, error) {
	if l, ok := r.ByCLI[cli]; ok {
		return l.List(ctx, cli)
	}
	if r.Default != nil {
		return r.Default.List(ctx, cli)
	}
	return nil, errNoLister(cli)
}

type errNoLister string

func (e errNoLister) Error() string { return "modelquery: no lister for cli " + string(e) }
