package modelquery

import (
	"context"
	"slices"
	"sync"
)

type onceLister struct {
	inner Lister
	once  sync.Once
	ids   []string
	err   error
}

func (l *onceLister) List(ctx context.Context, cli string) ([]string, error) {
	l.once.Do(func() { l.ids, l.err = l.inner.List(ctx, cli) })
	return slices.Clone(l.ids), l.err
}
