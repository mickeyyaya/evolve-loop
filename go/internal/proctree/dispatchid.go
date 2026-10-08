package proctree

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type DispatchID struct {
	Run   string
	Cycle int
	Agent string
	Owner int
	Nonce string
}

var dispatchIDRE = regexp.MustCompile(`^([^/]+)/(\d+)/([^/]+)/p(\d+)n([0-9a-z]+)$`)

func (d DispatchID) String() string {
	return fmt.Sprintf("%s/%d/%s/p%dn%s", idPart(d.Run), d.Cycle, idPart(d.Agent), d.Owner, d.Nonce)
}

func idPart(s string) string { return strings.ReplaceAll(s, "/", "_") }

func ParseDispatchID(s string) (DispatchID, bool) {
	m := dispatchIDRE.FindStringSubmatch(s)
	if m == nil {
		return DispatchID{}, false
	}
	cycle, cerr := strconv.Atoi(m[2])
	owner, oerr := strconv.Atoi(m[4])
	if cerr != nil || oerr != nil || owner <= 1 {
		return DispatchID{}, false
	}
	return DispatchID{Run: m[1], Cycle: cycle, Agent: m[3], Owner: owner, Nonce: m[5]}, true
}
