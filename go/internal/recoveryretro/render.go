package recoveryretro

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
)

const (
	infrastructureMark = "infrastructure?"
	shortSHALen        = 8
)

func RecoveryName(r Recovery) string {
	return "cycle-" + strconv.Itoa(r.Pass.Cycle) + "-recovery-" + itemSlug(r.TaskID)
}

func RenderRecovery(r Recovery) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Recovery of %s in cycle %d\n\n", quote(r.TaskID), r.Pass.Cycle)
	b.WriteString("- **Kind:** a recovery retrospective. The host wrote it from the committed cycle dossiers.\n")
	fmt.Fprintf(&b, "- **Item:** %s. **Shipped:** cycle %d, %s.\n", quote(r.TaskID), r.Pass.Cycle, commitText(r.Pass.CommitSHA))
	fmt.Fprintf(&b, "- **Failed tries:** %d, since the last pass of the item.\n\n", len(r.Failures))
	writeTries(&b, r)
	writeDifference(&b, r)
	fmt.Fprintf(&b, "## Lesson\n\n%s\n\n", recoveryLesson(r))
	b.WriteString("## Judgment\n\nThe host states only the facts. Write the reason for the pass here.\n")
	return b.String()
}

func commitText(sha string) string {
	if sha == "" {
		return "with no commit in the dossier"
	}
	return "commit " + quote(shortSHA(sha))
}

func shortSHA(sha string) string {
	if len(sha) > shortSHALen {
		return sha[:shortSHALen]
	}
	return sha
}

func writeTries(b *strings.Builder, r Recovery) {
	b.WriteString("## Tries\n\n| Cycle | Verdict | Class | Fingerprint | First reason | Mark |\n|---|---|---|---|---|---|\n")
	for _, f := range r.Failures {
		fmt.Fprintf(b, "| %d | FAIL | %s | %s | %s | %s |\n", f.Cycle, cell(f.Class), cell(f.Fingerprint), cell(f.Reason), mark(f))
	}
	passReason := "—"
	if r.Pass.CommitSHA != "" {
		passReason = "commit " + quote(shortSHA(r.Pass.CommitSHA))
	}
	fmt.Fprintf(b, "| %d | PASS | — | — | %s | |\n\n", r.Pass.Cycle, passReason)
	b.WriteString("The mark \"infrastructure?\" shows the class `infra-error` or `unknown`. Such a failure can come from the host, not from the item.\n\n")
}

func mark(t Try) string {
	if t.Class == dossier.PreClassInfraError || t.Class == dossier.PreClassUnknown {
		return infrastructureMark
	}
	return ""
}

func writeDifference(b *strings.Builder, r Recovery) {
	last := r.Failures[len(r.Failures)-1]
	fmt.Fprintf(b, "## Difference\n\nThe host compares the last failed try (cycle %d) with the passed try (cycle %d).\n\n", last.Cycle, r.Pass.Cycle)
	passOnly, failOnly := phaseDelta(r.Pass.Models, last.Models), phaseDelta(last.Models, r.Pass.Models)
	if len(passOnly) == 0 && len(failOnly) == 0 {
		b.WriteString("- The two tries ran the same phases.\n")
	}
	if len(passOnly) > 0 {
		fmt.Fprintf(b, "- Phases in the passed try only: %s.\n", quoteAll(passOnly))
	}
	if len(failOnly) > 0 {
		fmt.Fprintf(b, "- Phases in the last failed try only: %s.\n", quoteAll(failOnly))
	}
	changes := modelChanges(last.Models, r.Pass.Models)
	writeModelChanges(b, changes)
	if len(r.Rungs) > 0 {
		fmt.Fprintf(b, "- The passed try used these ladder rungs: %s.\n", quoteAll(r.Rungs))
	} else {
		b.WriteString("- The passed try used no ladder rung above `R0`.\n")
	}
	fmt.Fprintf(b, "- The %d failed tries have %s.\n", len(r.Failures), fingerprintCount(r.Failures))
	if len(passOnly)+len(failOnly)+len(changes)+len(r.Rungs) == 0 {
		b.WriteString("- The host found no difference in phases, models or rungs. The pass can be luck, so read the passed commit.\n")
	}
	b.WriteString("\n")
}

func writeModelChanges(b *strings.Builder, changes []string) {
	if len(changes) == 0 {
		b.WriteString("- No phase changed its model.\n")
		return
	}
	b.WriteString("- The model changed in these phases:\n")
	for _, c := range changes {
		fmt.Fprintf(b, "  - %s.\n", c)
	}
}

func phaseDelta(from, against []PhaseModel) []string {
	var out []string
	for _, m := range from {
		if !slices.Contains(out, m.Phase) && !slices.ContainsFunc(against, func(o PhaseModel) bool { return o.Phase == m.Phase }) {
			out = append(out, m.Phase)
		}
	}
	return out
}

func modelChanges(before, after []PhaseModel) []string {
	var out []string
	for _, a := range after {
		for _, b := range before {
			if a.Phase == b.Phase && a.Model != "" && b.Model != "" && a.Model != b.Model {
				out = append(out, fmt.Sprintf("%s: %s to %s", quote(a.Phase), quote(b.Model), quote(a.Model)))
			}
		}
	}
	return out
}

func fingerprintCount(failures []Try) string {
	seen := map[string]bool{}
	for _, f := range failures {
		if f.Fingerprint != "" {
			seen[f.Fingerprint] = true
		}
	}
	if len(seen) == 1 {
		return "1 fingerprint"
	}
	return strconv.Itoa(len(seen)) + " fingerprints"
}

func failureClasses(failures []Try) []string {
	var out []string
	for _, f := range failures {
		if f.Class != "" && !slices.Contains(out, f.Class) {
			out = append(out, f.Class)
		}
	}
	return out
}

func recoveryLesson(r Recovery) string {
	classes := "with no recorded class"
	if c := failureClasses(r.Failures); len(c) > 0 {
		classes = "with the classes " + quoteAll(c)
	}
	return fmt.Sprintf("The item %s failed in %d tries %s. Cycle %d shipped it. Before a new try of a similar item, read the difference above.",
		quote(r.TaskID), len(r.Failures), classes, r.Pass.Cycle)
}
