package agentclass

import (
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

const (
	RequestsDir  = "requests"
	KindDispatch = "dispatch"
	KindInboxAdd = "inbox-add"
)

var (
	requestFile = regexp.MustCompile(`^([a-z][a-z-]*)-([0-9]+)\.json$`)
	requestName = regexp.MustCompile(`^[a-z][a-z-]*-[0-9]+(\.[a-z][a-z-]*-[0-9]+)*$`)
)

func RequestFile(file string) (string, int, bool) {
	m := requestFile.FindStringSubmatch(file)
	if m == nil {
		return "", 0, false
	}
	n, err := strconv.Atoi(m[2])
	return m[1], n, err == nil
}

func ChildRequest(parent, file string) string {
	stem := strings.TrimSuffix(file, ".json")
	if parent == "" {
		return stem
	}
	return parent + "." + stem
}

func RequestKind(name string) string {
	leaf := name[strings.LastIndex(name, ".")+1:]
	kind, _, _ := RequestFile(leaf + ".json")
	return kind
}

func ValidRequestName(name string) bool { return requestName.MatchString(name) }

type ArgSpec struct {
	Name     string
	Numbered bool
}

var argSpecs = map[string][]ArgSpec{
	KindDispatch: {{Name: "role"}},
	KindInboxAdd: {{Name: "id"}, {Name: "title"}, {Name: "kind"}, {Name: "fix"}, {Name: "priority_class"},
		{Name: "acceptance", Numbered: true}, {Name: "files", Numbered: true}},
}

var numberedArg = regexp.MustCompile(`^([a-z_]+)-([0-9]+)$`)

func ArgSpecs(kind string) []ArgSpec { return slices.Clone(argSpecs[kind]) }

func RequiredArgs(kind string) []string {
	var out []string
	for _, spec := range argSpecs[kind] {
		if spec.Numbered {
			out = append(out, spec.Name+"-1")
		} else {
			out = append(out, spec.Name)
		}
	}
	return out
}

func ParseArg(kind, key string) (name string, n int, ok bool) {
	for _, spec := range argSpecs[kind] {
		if !spec.Numbered && spec.Name == key {
			return key, 0, true
		}
		if m := numberedArg.FindStringSubmatch(key); spec.Numbered && m != nil && m[1] == spec.Name {
			n, err := strconv.Atoi(m[2])
			return spec.Name, n, err == nil
		}
	}
	return "", 0, false
}

func ArgKinds() []string { return slices.Sorted(maps.Keys(argSpecs)) }
