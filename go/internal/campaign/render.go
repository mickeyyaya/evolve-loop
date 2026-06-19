package campaign

import (
	"fmt"
	"strings"
)

// Render produces the human-readable campaign plan (Markdown) shown for approval
// before any execution — goal, cited research strategy, the cycle table, and the
// computed dependency-ordered execution waves. This is the "plan mode" view: the
// operator reads it, then approves or sends suggestions to replan. Verify should
// pass first; Render surfaces the wave-computation error if the DAG is invalid.
func (p *Plan) Render() (string, error) {
	waves, err := p.Waves()
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Campaign Plan (v%d)\n\n", p.Version)
	fmt.Fprintf(&b, "**Goal:** %s\n\n", p.Goal)
	if s := strings.TrimSpace(p.Research.Summary); s != "" {
		fmt.Fprintf(&b, "**Strategy:** %s\n\n", s)
	}
	if len(p.Research.Citations) > 0 {
		b.WriteString("**Research citations:**\n")
		for _, c := range p.Research.Citations {
			fmt.Fprintf(&b, "- %s", c.Title)
			if c.URL != "" {
				fmt.Fprintf(&b, " (%s)", c.URL)
			}
			if c.Note != "" {
				fmt.Fprintf(&b, " — %s", c.Note)
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("## Cycles\n\n")
	b.WriteString("| id | files | depends_on | priority | output_contract |\n")
	b.WriteString("|---|---|---|---|---|\n")
	for _, c := range p.Cycles {
		fmt.Fprintf(&b, "| %s | %s | %s | %d | %s |\n",
			c.ID, strings.Join(c.Files, ", "), strings.Join(c.DependsOn, ", "), c.Priority, c.OutputContract)
	}
	b.WriteString("\n")

	fmt.Fprintf(&b, "## Execution Waves (%d)\n\n", len(waves))
	for i, wave := range waves {
		groups := make([]string, len(wave))
		for j, spec := range wave {
			groups[j] = "[" + strings.Join(spec.Scope, "+") + "]"
		}
		fmt.Fprintf(&b, "- **Wave %d** — %d concurrent cycle(s): %s\n", i+1, len(wave), strings.Join(groups, " "))
	}
	b.WriteString("\nApprove to execute, or send suggestions to replan.\n")
	return b.String(), nil
}
