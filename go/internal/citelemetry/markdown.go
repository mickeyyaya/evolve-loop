package citelemetry

import (
	"fmt"
	"strings"
)

func (r Report) Markdown() string {
	var b strings.Builder
	b.WriteString("# CI critical path\n\n| Level | Unit | Seconds |\n|---|---|---|\n")
	for _, s := range r.CriticalPath {
		fmt.Fprintf(&b, "| %s | %s | %.0f |\n", s.Level, s.Unit, s.Seconds)
	}
	fmt.Fprintf(&b, "\n## G1\n\n- macOS job: %.0f s\n- ubuntu test step: %.0f s\n", r.G1.MacosJobS, r.G1.UbuntuTestStepS)
	if len(r.Packages) > 0 {
		b.WriteString("\n## Packages\n\n| Package | Result | Wall s | Limit |\n|---|---|---|---|\n")
		for _, p := range r.Packages {
			fmt.Fprintf(&b, "| %s | %s | %.0f | %s |\n", p.Package, p.Result, p.WallS, p.Limit)
		}
	}
	if len(r.Incomplete) > 0 {
		b.WriteString("\n## Incomplete\n\n")
		for _, name := range r.Incomplete {
			fmt.Fprintf(&b, "- %s\n", name)
		}
	}
	return b.String()
}
