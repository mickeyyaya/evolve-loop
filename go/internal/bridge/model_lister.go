package bridge

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelquery"
)

type modelListing struct {
	once  sync.Once
	names []string
	err   error
}

type manifestModelLister struct {
	set      DriverSet
	run      modelquery.Runner
	mu       sync.Mutex
	listings map[string]*modelListing
}

func NewModelLister(set DriverSet, run modelquery.Runner) modelquery.RoutedLister {
	return &manifestModelLister{set: set, run: run, listings: map[string]*modelListing{}}
}

func (l *manifestModelLister) Families() []string {
	return slices.DeleteFunc(l.set.Families(), func(family string) bool {
		_, _, err := l.format(family)
		return err != nil
	})
}

func (l *manifestModelLister) List(ctx context.Context, cli string) ([]string, error) {
	r, format, err := l.format(cli)
	if err != nil {
		return nil, err
	}
	listing := l.listingFor(r.Manifest.Binary + "\x00" + strings.Join(format.argv, "\x00"))
	listing.once.Do(func() { listing.names, listing.err = l.start(ctx, r.Manifest.Binary, format) })
	return slices.Clone(listing.names), listing.err
}

func (l *manifestModelLister) format(cli string) (Resolved, modelsFormat, error) {
	r, err := Resolve(l.set, cli)
	if err != nil {
		return r, modelsFormat{}, err
	}
	id := r.Manifest.Parsers[ParserModels]
	format, ok := modelsFormats[id]
	if !ok {
		return r, format, fmt.Errorf("bridge: %s declares no known parsers.%s id (%q)", r.Driver, ParserModels, id)
	}
	return r, format, nil
}

func (l *manifestModelLister) listingFor(key string) *modelListing {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.listings[key] == nil {
		l.listings[key] = &modelListing{}
	}
	return l.listings[key]
}

type modelListingError struct {
	Command string
	Output  string
	Err     error
}

func (e *modelListingError) Error() string {
	if e.Output == "" {
		return fmt.Sprintf("%s: %v", e.Command, e.Err)
	}
	return fmt.Sprintf("%s: %v: %s", e.Command, e.Err, e.Output)
}

func (e *modelListingError) Unwrap() error { return e.Err }

func (l *manifestModelLister) start(ctx context.Context, binary string, format modelsFormat) ([]string, error) {
	command := binary + " " + strings.Join(format.argv, " ")
	out, err := l.run(ctx, binary, format.argv, "")
	if err != nil {
		return nil, &modelListingError{Command: command, Output: modelquery.ClipOutput(out), Err: err}
	}
	names, err := format.parse(out)
	if err != nil {
		return nil, &modelListingError{Command: command, Err: err}
	}
	return names, nil
}
