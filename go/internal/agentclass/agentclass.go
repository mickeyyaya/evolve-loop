package agentclass

import (
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

const (
	Allowed       = "A"
	Request       = "R"
	Denied        = "D"
	RequestSchema = "evolve.request/1"
	nameSlot      = "<name>"
	cellTemplate  = ".evolve/runs/cycle-<N>"
)

type Verb struct {
	Path       string
	Class      string
	Kind       string
	OnlyWith   string
	DeniedWith string
}

type Match struct {
	Name  string
	Class string
	Kind  string
}

func Verbs() []Verb { return slices.Clone(table) }

func find(path string) (Verb, bool) {
	i := slices.IndexFunc(table, func(v Verb) bool { return v.Path == path })
	if i < 0 {
		return Verb{}, false
	}
	return table[i], true
}

func Of(args []string) Match {
	if len(args) == 0 {
		return Match{Class: Denied}
	}
	v, name, ok := lookup(args)
	if !ok {
		return Match{Name: args[0], Class: Denied}
	}
	rest := args[len(strings.Fields(name)):]
	if (v.OnlyWith != "" && !flagSet(rest, v.OnlyWith, true)) || (v.DeniedWith != "" && flagSet(rest, v.DeniedWith, false)) {
		return Match{Name: name, Class: Denied}
	}
	return Match{Name: name, Class: v.Class, Kind: v.Kind}
}

func lookup(args []string) (Verb, string, bool) {
	if len(args) > 1 {
		name := args[0] + " " + args[1]
		if v, ok := find(name); ok {
			return v, name, true
		}
		if v, ok := find(args[0] + " " + nameSlot); ok && !strings.HasPrefix(args[1], "-") {
			return v, name, true
		}
	}
	v, ok := find(args[0])
	return v, args[0], ok
}

func flagSet(args []string, flag string, onlyWith bool) bool {
	name := strings.TrimLeft(flag, "-")
	set, valueSlot := false, false
	for _, a := range args {
		if a == "--" {
			break
		}
		isFlag := strings.HasPrefix(a, "-")
		key, value, hasValue := strings.Cut(strings.TrimLeft(a, "-"), "=")
		if isFlag && key == name && (!onlyWith || !valueSlot) {
			on, err := strconv.ParseBool(value)
			set = !hasValue || (err == nil && on) || (err != nil && !onlyWith)
		}
		valueSlot = isFlag && !hasValue && key != name
	}
	return set
}

func RequestPath(workspace, kind string) string {
	if workspace == "" {
		workspace = cellTemplate
	}
	return filepath.Join(workspace, "requests", kind+"-<n>.json")
}

func (m Match) Refusal(workspace string) string {
	if m.Class == Request {
		return fmt.Sprintf("refused inside a phase: %s is class R; write the request to %s (schema %s, see evolve agent capabilities --json)", m.Name, RequestPath(workspace, m.Kind), RequestSchema)
	}
	return fmt.Sprintf("refused inside a phase: %s is class %s; it is an operator or boundary verb", m.Name, m.Class)
}
