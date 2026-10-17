package bridge

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
)

const tmuxSuffix = "-tmux"

type DriverSet struct {
	drivers map[string]string
	load    func(string) (Manifest, error)
}

func NewDriverSet(names []string, load func(string) (Manifest, error)) DriverSet {
	drivers := make(map[string]string, len(names))
	for _, name := range names {
		if family, ok := strings.CutSuffix(name, tmuxSuffix); ok {
			drivers[family] = name
		}
	}
	return DriverSet{drivers: drivers, load: load}
}

var DefaultDriverSet = sync.OnceValue(func() DriverSet {
	return NewDriverSet(ManifestNames(), LoadManifest)
})

func (s DriverSet) WithLoader(load func(string) (Manifest, error)) DriverSet {
	return DriverSet{drivers: s.drivers, load: load}
}

func (s DriverSet) Families() []string {
	return slices.Sorted(maps.Keys(s.drivers))
}

type Resolved struct {
	Driver   string
	Manifest Manifest
}

type UnknownFamilyError struct{ Family string }

func (e *UnknownFamilyError) Error() string {
	return fmt.Sprintf("bridge: no interactive driver for family %q", e.Family)
}

func Resolve(set DriverSet, family string) (Resolved, error) {
	driver, ok := set.drivers[family]
	if !ok {
		return Resolved{}, &UnknownFamilyError{Family: family}
	}
	m, err := set.load(driver)
	if err != nil {
		return Resolved{Driver: driver}, fmt.Errorf("bridge: resolve %s: %w", family, err)
	}
	return Resolved{Driver: driver, Manifest: m}, nil
}
