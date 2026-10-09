package filter

import (
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type keyClass int

const (
	textClass keyClass = iota
	globClass
	numberClass
	severityClass
)

type unknownValue int

const (
	unknownIsFree unknownValue = iota
	unknownIsRefused
	unknownWarns
)

type value struct {
	text string
	rank int64
}

type keySpec struct {
	class     keyClass
	read      func(Record) (value, bool)
	vocab     func(Catalog) []string
	onUnknown unknownValue
}

var keys = map[string]keySpec{
	"kind":     {class: globClass, read: textOf(func(e *signalcenter.Event) string { return string(e.Kind) }), vocab: Catalog.kindNames, onUnknown: unknownIsRefused},
	"code":     {class: globClass, read: textOf(func(e *signalcenter.Event) string { return string(e.Code) }), vocab: Catalog.codeNames, onUnknown: unknownWarns},
	"module":   {class: textClass, read: textOf(func(e *signalcenter.Event) string { return string(e.Module) }), vocab: Catalog.moduleNames, onUnknown: unknownIsRefused},
	"severity": {class: severityClass, read: severityOf},
	"cycle":    {class: numberClass, read: nonZeroNumberOf(func(e *signalcenter.Event) int64 { return int64(e.Cycle) })},
	"attempt":  {class: numberClass, read: nonZeroNumberOf(func(e *signalcenter.Event) int64 { return int64(e.Attempt) })},
	"pid":      {class: numberClass, read: numberOf(func(e *signalcenter.Event) int64 { return int64(e.PID) })},
	"seq":      {class: numberClass, read: numberOf(func(e *signalcenter.Event) int64 { return int64(e.Seq) })},
	"phase":    {class: textClass, read: textOf(func(e *signalcenter.Event) string { return e.Phase })},
	"run_id":   {class: textClass, read: textOf(func(e *signalcenter.Event) string { return e.RunID })},
	"origin":   {class: textClass, read: textOf(func(e *signalcenter.Event) string { return e.Origin })},
	"source":   {class: textClass, read: func(r Record) (value, bool) { return value{text: r.Source}, r.Source != "" }},
}

func lookupKey(name string) (keySpec, bool) {
	if field, ok := strings.CutPrefix(name, "fields."); ok && field != "" {
		return keySpec{class: textClass, read: fieldOf(field)}, true
	}
	spec, ok := keys[name]
	return spec, ok
}

func textOf(get func(*signalcenter.Event) string) func(Record) (value, bool) {
	return func(r Record) (value, bool) {
		text := get(r.Signal)
		return value{text: text}, text != ""
	}
}

func severityOf(r Record) (value, bool) {
	sev := r.Signal.Severity
	return value{text: string(sev), rank: int64(sev.Level())}, sev.Valid()
}

func numberOf(get func(*signalcenter.Event) int64) func(Record) (value, bool) {
	return func(r Record) (value, bool) {
		n := get(r.Signal)
		return value{text: strconv.FormatInt(n, 10), rank: n}, true
	}
}

func nonZeroNumberOf(get func(*signalcenter.Event) int64) func(Record) (value, bool) {
	return func(r Record) (value, bool) {
		v, _ := numberOf(get)(r)
		return v, v.rank != 0
	}
}

func fieldOf(name string) func(Record) (value, bool) {
	return func(r Record) (value, bool) {
		text, ok := r.Signal.Fields[name]
		return value{text: text}, ok
	}
}

func (c keyClass) ordered() bool { return c == numberClass || c == severityClass }

func (c keyClass) equal(got, want value) bool {
	switch c {
	case globClass:
		return globMatch(want.text, got.text)
	case numberClass, severityClass:
		return got.rank == want.rank
	}
	return got.text == want.text
}
