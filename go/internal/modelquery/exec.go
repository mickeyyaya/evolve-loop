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

// DefaultRouter is the SINGLE production registry of which CLI is enumerated
// how. It exists because the registry was previously duplicated as a map
// literal at each call site, and a CLI added to one copy but not the other is
// invisible: the missing entry does not error, it just falls through to the
// picker Default and yields plausible-but-wrong model names.
//
// Registered here means "this CLI has a non-interactive listing that is more
// faithful than its picker". Everything else uses the picker via Default.
// capturer may be nil in tests that only assert the routing.
func DefaultRouter(capturer ModelCapturer) Router {
	return Router{
		ByCLI: map[string]Lister{
			"ollama": OllamaLister{},
			// agy's picker splits model from effort, so its pane cannot
			// produce a valid --model value — see AgyLister.
			"agy": AgyLister{},
		},
		Default: RecipeLister{Capturer: capturer},
	}
}

// Router dispatches List calls to a per-CLI Lister strategy (ollama and agy
// use non-interactive listings; the rest drive the REPL /model picker). cli
// names are expected to be base names already (claude|codex|agy|ollama).
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
