package filter

import (
	"errors"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var ErrUsage = errors.New("usage error")

var ErrRefused = errors.New("refused")

type Record struct {
	Source string
	Signal *signalcenter.Event
}

type Filter struct {
	terms []term
}

func (f Filter) Match(r Record) bool {
	if r.Signal == nil {
		return true
	}
	for _, t := range f.terms {
		if !t.match(r) {
			return false
		}
	}
	return true
}

func (f Filter) Until(r Record) bool {
	return r.Signal != nil && f.Match(r)
}
