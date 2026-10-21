package recoveryretro

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

var judgmentID = regexp.MustCompile(`^cycle-([0-9]+)-judgment-([a-z0-9-]+)$`)

type JudgmentLesson struct {
	cyclestate.CarryoverTodo
	Cycle int
	Phase string
}

func JudgmentLessons(todos []cyclestate.CarryoverTodo) []JudgmentLesson {
	var out []JudgmentLesson
	for _, t := range todos {
		m := judgmentID.FindStringSubmatch(t.ID)
		if m == nil {
			continue
		}
		cycle, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		out = append(out, JudgmentLesson{CarryoverTodo: t, Cycle: cycle, Phase: m[2]})
	}
	return out
}

func RenderJudgmentLesson(l JudgmentLesson) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Judgment lesson %s\n\n", quote(l.ID))
	b.WriteString("- **Kind:** a judgment lesson. The host wrote it from the carryover todo of the same id.\n")
	fmt.Fprintf(&b, "- **Cycle:** %d. **Phase:** %s.\n", l.Cycle, quote(l.Phase))
	b.WriteString("- **Carryover todo:** at stage `enforce`, the todo leaves the carryover log when this document is on `main`.\n\n")
	fmt.Fprintf(&b, "## Lesson\n\n%s\n\n", quote(l.Action))
	b.WriteString("## Judgment\n\nThe host states only the facts. Write the judgment here.\n")
	return b.String()
}
