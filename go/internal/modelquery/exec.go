package modelquery

import (
	"context"
	"errors"
)

type Runner func(ctx context.Context, name string, args []string, stdin string) (string, error)

var errNoLauncher = errors.New("modelquery: no LLM launcher is wired; the composition root calls UseLauncher(bridge.ModelQueryRunner)")

var launcher Runner

func UseLauncher(launch Runner) {
	launcher = launch
}

func defaultRunner(ctx context.Context, name string, args []string, stdin string) (string, error) {
	if launcher == nil {
		return "", errNoLauncher
	}
	return launcher(ctx, name, args, stdin)
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
