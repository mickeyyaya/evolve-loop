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

var requiredArgs = map[string][]string{
	KindDispatch: {"role"},
	KindInboxAdd: {"id", "title", "kind", "fix", "priority_class", "acceptance-1"},
}

func RequiredArgs(kind string) []string { return slices.Clone(requiredArgs[kind]) }

func ArgKinds() []string { return slices.Sorted(maps.Keys(requiredArgs)) }
