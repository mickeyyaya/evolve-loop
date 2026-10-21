package recoveryretro

import (
	"strconv"
	"strings"
)

const (
	DocsHome         = "docs/retrospectives"
	lessonConfidence = "0.8"
	kindRecovery     = "recovery"
	kindJudgment     = "judgment"
)

type lessonEntry struct {
	id, pattern, description, source, kind, action, failedStep, errorCategory string
}

func recoveryLessonYAML(r Recovery, docPath string) []byte {
	return lessonEntry{
		id: RecoveryName(r), pattern: "recovery of " + r.TaskID, description: recoveryLesson(r),
		source: docPath, kind: kindRecovery, action: "Read " + docPath + " before a new try of the item " + r.TaskID + " or a similar item.",
		errorCategory: strings.Join(failureClasses(r.Failures), " "),
	}.yaml()
}

func judgmentLessonYAML(l JudgmentLesson, docPath string) []byte {
	return lessonEntry{
		id: l.ID, pattern: "judgment of " + l.Phase, description: l.Action,
		source: docPath, kind: kindJudgment, action: "Read " + docPath + " before the " + l.Phase + " phase judges a similar plan.",
		failedStep: l.Phase,
	}.yaml()
}

func (e lessonEntry) yaml() []byte {
	var b strings.Builder
	b.WriteString("- id: " + strconv.Quote(e.id) + "\n")
	for _, kv := range [][2]string{
		{"pattern", e.pattern}, {"description", e.description}, {"source", e.source},
		{"type", e.kind}, {"category", "retrospective"}, {"preventiveAction", e.action},
	} {
		b.WriteString("  " + kv[0] + ": " + strconv.Quote(kv[1]) + "\n")
	}
	b.WriteString("  confidence: " + lessonConfidence + "\n")
	b.WriteString("  failureContext:\n")
	b.WriteString("    failedStep: " + strconv.Quote(e.failedStep) + "\n")
	b.WriteString("    errorCategory: " + strconv.Quote(e.errorCategory) + "\n")
	return []byte(b.String())
}
