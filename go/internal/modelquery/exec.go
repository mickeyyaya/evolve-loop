package modelquery

import (
	"context"
	"os/exec"
	"strings"
)

type Runner func(ctx context.Context, name string, args []string, stdin string) (string, error)

var processEnv func(bin string) []string

func UseProcessEnv(env func(bin string) []string) {
	processEnv = env
}

func defaultRunner(ctx context.Context, name string, args []string, stdin string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if processEnv != nil {
		cmd.Env = processEnv(name)
	}
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func DefaultRouter(capturer ModelCapturer) Router {
	return routerWith(capturer, nil)
}

func routerWith(capturer ModelCapturer, run Runner) Router {
	agyListing := &onceLister{inner: AgyLister{Run: run}}
	return Router{
		ByCLI: map[string]Lister{
			"ollama":     OllamaLister{},
			"agy":        agyListing,
			"agy-claude": agyListing,
		},
		Default: RecipeLister{Capturer: capturer},
	}
}

type Router struct {
	ByCLI   map[string]Lister
	Default Lister
}

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
