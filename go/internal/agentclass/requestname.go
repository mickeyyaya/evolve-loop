package agentclass

import (
	"regexp"
	"strconv"
	"strings"
)

const (
	RequestsDir  = "requests"
	KindDispatch = "dispatch"
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

func ValidRequestName(name string) bool { return requestName.MatchString(name) }
