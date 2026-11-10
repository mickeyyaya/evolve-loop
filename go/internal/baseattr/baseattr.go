// Package baseattr judges whether a red test belongs to the lane or to its base.
// See docs/architecture/packages/internal-baseattr.md.
package baseattr

import (
	"context"
	"regexp"
	"slices"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

type Key struct {
	Package string
	Test    string
}

type Reds struct {
	Keys    []Key
	Unnamed bool
	Flags   []string
}

type Verdict struct {
	LaneOnly []Key
	BaseRed  []Key
	MainRed  []Key
	Unjudged bool
	Base     string
	Main     string
	Took     time.Duration
}

type Runner func(ctx context.Context, moduleDir string, r Reds) ([]Key, error)

type Attributor struct {
	Git       gitexec.Git
	Run       Runner
	MainRef   string
	Scratch   string
	Timeout   time.Duration
	Serialize func(context.Context) func()
}

var namedTest = regexp.MustCompile(`^(.+?)\.((?:Test|Benchmark|Example|Fuzz)[^/]*)(?:/.*)?$`)

func KeysOf(names []string) Reds {
	var r Reds
	for _, name := range names {
		m := namedTest.FindStringSubmatch(name)
		if m == nil {
			r.Unnamed = true
			continue
		}
		r.Keys = appendKey(r.Keys, Key{Package: m[1], Test: m[2]})
	}
	return r
}

func appendKey(keys []Key, k Key) []Key {
	if slices.Contains(keys, k) {
		return keys
	}
	return append(keys, k)
}

func (r Reds) Attributable() bool {
	return !r.Unnamed && len(r.Keys) > 0
}

func (v Verdict) Blamed() bool {
	return v.Unjudged || len(v.LaneOnly) > 0
}

func (v Verdict) MainGreen() bool {
	return len(v.BaseRed) > 0 && v.Main != "" && len(v.MainRed) == 0
}
