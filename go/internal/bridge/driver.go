package bridge

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

type Driver interface {
	Name() string

	Launch(ctx context.Context, cfg *Config, deps Deps) (int, error)
}

type CLIPreflight interface {
	Preflight(ctx context.Context, cfg *Config, deps Deps) error
}

var (
	driverMu sync.RWMutex
	drivers  = map[string]Driver{}
)

func Register(d Driver) {
	if d == nil {
		panic("bridge: Register requires a non-nil Driver")
	}
	name := d.Name()
	if name == "" {
		panic("bridge: Register requires a non-empty Driver.Name()")
	}
	driverMu.Lock()
	defer driverMu.Unlock()
	if _, exists := drivers[name]; exists {
		panic(fmt.Sprintf("bridge: duplicate Register(%q) — each driver registers exactly once", name))
	}
	drivers[name] = d
}

func LookupDriver(cli string) (Driver, bool) {
	driverMu.RLock()
	defer driverMu.RUnlock()
	d, ok := drivers[cli]
	return d, ok
}

var bareDriverMap = map[string]string{
	"claude": "claude-tmux",
	"gemini": "claude-tmux",
	"codex":  "codex-tmux",
	"agy":    "agy-tmux",
}

func DriverFor(cli string) string {
	if _, ok := LookupDriver(cli); ok {
		return cli
	}
	if mapped, ok := bareDriverMap[cli]; ok {
		return mapped
	}
	return cli
}

func DriverNames() []string {
	driverMu.RLock()
	defer driverMu.RUnlock()
	out := make([]string, 0, len(drivers))
	for n := range drivers {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

func ResetDriversForTesting() {
	driverMu.Lock()
	defer driverMu.Unlock()
	drivers = map[string]Driver{}
}
