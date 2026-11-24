package launchoutcome

import (
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type Refusal struct {
	Guard string
	Path  string
}

func (r Refusal) Present() bool { return r.Guard != "" }

func (r Refusal) Text() string {
	if !r.Present() {
		return ""
	}
	return "guard " + r.Guard + " denied " + r.Path + " before the launch stalled"
}

func GuardRefusal(events []signalcenter.Event, cell string) Refusal {
	if cell == "" {
		return Refusal{}
	}
	for _, e := range events {
		if e.Code == core.CodeGuardDenied && e.Fields[core.DenialFieldNeutral] == "true" && under(e.Fields[core.DenialFieldCWD], cell) {
			return Refusal{Guard: e.Fields[core.DenialFieldGuard], Path: e.Fields[core.DenialFieldPath]}
		}
	}
	return Refusal{}
}

func under(dir, cell string) bool {
	sep := string(filepath.Separator)
	return strings.HasPrefix(filepath.Clean(dir)+sep, filepath.Clean(cell)+sep)
}
