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

type RoutedLister interface {
	Lister
	Families() []string
}

func DefaultRouter(capturer ModelCapturer, routed RoutedLister) Router {
	byCLI := map[string]Lister{}
	if routed != nil {
		for _, family := range routed.Families() {
			byCLI[family] = routed
		}
	}
	return Router{ByCLI: byCLI, Default: RecipeLister{Capturer: capturer}}
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
