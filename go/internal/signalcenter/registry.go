package signalcenter

import (
	"sort"
	"sync"
)

type CodeDoc struct {
	Code Code
	Doc  string
}

type Conflict struct {
	Code        Code
	Module      Module
	OtherModule Module
	Doc         string
	OtherDoc    string
}

type registry struct {
	mu        sync.RWMutex
	owner     map[Code]Module
	docs      map[Code]string
	conflicts []Conflict
}

var codes = &registry{owner: map[Code]Module{}, docs: map[Code]string{}}

func init() {
	for c, doc := range map[Code]string{
		CodeUnknownModule:    "an event named a module outside the closed set; raw value in fields.raw_module",
		CodeUnknownKind:      "an event named a kind outside the closed set; raw value in fields.raw_kind",
		CodeUnknownSeverity:  "an event carried a severity outside INFO/WARN/INCIDENT; raw value in fields.raw_severity",
		CodeMissingCode:      "a WARN or INCIDENT event carried no code",
		CodeUnregisteredCode: "an event carried a code its module never registered; raw value in fields.raw_code",
		CodeMissingReason:    "an event carried no reason",
		CodeBadOrigin:        "an event's origin is not a Func or Type.Method name; raw value in fields.raw_origin",
		CodeListenerPanicked: "a listener panicked and was unsubscribed; the panic value is in the reason",
		CodeSinkDropped:      "the durable sink had no path for N events (fields.dropped) before this write",
	} {
		RegisterCode(ModuleSignalCenter, c, doc)
	}
}

func RegisterCode(m Module, c Code, doc string) { codes.register(m, c, doc) }

func (r *registry) register(m Module, c Code, doc string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var badReason string
	switch {
	case !c.Valid():
		badReason = "malformed code (want MODULE_SNAKE_CASE)"
	case !c.BelongsTo(m):
		badReason = "prefix does not belong to " + string(m)
	}
	if badReason != "" {
		r.conflicts = append(r.conflicts, Conflict{Code: c, Module: m, OtherModule: m, Doc: doc, OtherDoc: badReason})
		return
	}
	if owner, ok := r.owner[c]; ok {
		if owner == m && r.docs[c] == doc {
			return
		}
		r.conflicts = append(r.conflicts, Conflict{Code: c, Module: owner, OtherModule: m, Doc: r.docs[c], OtherDoc: doc})
		return
	}
	r.owner[c] = m
	r.docs[c] = doc
}

func IsRegistered(c Code) (Module, bool) {
	codes.mu.RLock()
	defer codes.mu.RUnlock()
	m, ok := codes.owner[c]
	return m, ok
}

func RegisteredCodes() map[Module][]CodeDoc {
	codes.mu.RLock()
	defer codes.mu.RUnlock()
	out := map[Module][]CodeDoc{}
	for c, m := range codes.owner {
		out[m] = append(out[m], CodeDoc{Code: c, Doc: codes.docs[c]})
	}
	for m := range out {
		sort.Slice(out[m], func(i, j int) bool { return out[m][i].Code < out[m][j].Code })
	}
	return out
}

func RegistryConflicts() []Conflict {
	codes.mu.RLock()
	defer codes.mu.RUnlock()
	return append([]Conflict(nil), codes.conflicts...)
}
