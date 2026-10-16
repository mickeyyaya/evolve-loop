package signalcenter

import (
	"fmt"
	"sort"
	"strings"
)

func RenderCodes() string { return renderCodes(RegisteredCodes()) }

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
