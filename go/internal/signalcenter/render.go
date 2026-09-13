package signalcenter

import (
	"fmt"
	"sort"
	"strings"
)

// RenderCodes projects the code registry into markdown — one table per
// module, modules and codes sorted — the GENERATED region of
// docs/architecture/signal-codes.md (`evolve signals codes generate|check`).
// One source (RegisterCode), one projection: a code cannot be documented
// differently from how it is registered.
func RenderCodes() string { return renderCodes(RegisteredCodes()) }

// renderCodes is the pure projection of one registry snapshot.
func renderCodes(all map[Module][]CodeDoc) string {
	modules := make([]Module, 0, len(all))
	for m := range all {
		modules = append(modules, m)
	}
	sort.Slice(modules, func(i, j int) bool { return modules[i] < modules[j] })
	var b strings.Builder
	for _, m := range modules {
		docs := append([]CodeDoc(nil), all[m]...)
		sort.Slice(docs, func(i, j int) bool { return docs[i].Code < docs[j].Code })
		fmt.Fprintf(&b, "### %s\n\n| Code | Meaning |\n|---|---|\n", m)
		for _, d := range docs {
			fmt.Fprintf(&b, "| `%s` | %s |\n", d.Code, d.Doc)
		}
		b.WriteString("\n")
	}
	return b.String()
}
