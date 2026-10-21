package evalgate

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

type ConsoleFollowupFiler func(projectRoot string, raw []byte) error

func WithConsoleFollowupFiler(file ConsoleFollowupFiler) Option {
	return func(o *options) { o.fileFollowup = file }
}

type followupSource struct {
	ID            string  `json:"id"`
	Kind          string  `json:"kind"`
	PriorityClass string  `json:"priority_class"`
	Weight        float64 `json:"weight"`
}

func (g protectedACGate) fileFollowups(in core.ReviewInput, t acMaterialization) (string, bool) {
	if g.file == nil || in.ProjectRoot == "" {
		return "", false
	}
	src := sourceOf(in)
	var faults []string
	for _, row := range t.rows {
		if !t.isConsoleFollowup(row) {
			continue
		}
		id, raw := t.companion(src, in.Cycle, row)
		if err := g.file(in.ProjectRoot, raw); err != nil && !errors.Is(err, inboxmover.ErrAlreadyFiled) {
			faults = append(faults, fmt.Sprintf("%s: %v", id, err))
		}
	}
	if len(faults) == 0 {
		return "", false
	}
	return "the console-followup companion item(s) could not be filed, so the console has no record of them: " + strings.Join(faults, "; "), false
}

func sourceOf(in core.ReviewInput) followupSource {
	fallback := followupSource{ID: fmt.Sprintf("cycle-%d", in.Cycle), Kind: "feature", PriorityClass: "correctness", Weight: 0.5}
	ids, _ := committedset.Committed(in.Workspace)
	if len(ids) == 0 {
		return fallback
	}
	fallback.ID = ids[0]
	found := readSource(filepath.Join(in.ProjectRoot, ".evolve", "inbox"), ids[0])
	if found.Kind == "" || found.PriorityClass == "" || found.Weight <= 0 {
		return fallback
	}
	return found
}

func readSource(inboxDir, id string) followupSource {
	loc, err := inboxmover.Locate(inboxDir, id)
	if err != nil {
		return followupSource{}
	}
	raw, err := os.ReadFile(loc.Path)
	var src followupSource
	if err != nil || json.Unmarshal(raw, &src) != nil {
		return followupSource{}
	}
	return src
}

func (t acMaterialization) companion(src followupSource, cycle int, row []string) (string, []byte) {
	ac := kebab(row[0])
	if !strings.HasPrefix(ac, "ac") {
		ac = "ac-" + ac
	}
	id := kebab(src.ID + "-" + ac + "-console")
	needs := cellAt(row, t.needs)
	criterion := cellAt(row, t.criterion)
	if criterion == "" {
		criterion = strings.Join(row, " | ")
	}
	item := map[string]any{
		"id":             id,
		"kind":           src.Kind,
		"priority_class": src.PriorityClass,
		"weight":         src.Weight,
		"route":          inboxmover.RouteConsoleValue,
		"title":          fmt.Sprintf("Console part of %s: AC %s, which needs a protected path changed", src.ID, row[0]),
		"summary":        fmt.Sprintf("The TDD phase of cycle %d gave AC %s of %s the console-followup disposition. A lane builder can never change the paths it needs (%s). The lane part ships without it, so the console owns this part.", cycle, row[0], src.ID, needs),
		"fix":            fmt.Sprintf("Make the change that AC %s needs on the protected path(s) %s, then prove the criterion with a test.", row[0], needs),
		"acceptance":     []string{criterion},
		"source":         fmt.Sprintf("cycle %d, tdd test-report.md AC %s (console-followup)", cycle, row[0]),
	}
	if paths := guards.ProtectedPathsIn(needs); len(paths) > 0 {
		item["files"] = paths
	}
	raw, _ := json.Marshal(item)
	return id, raw
}

var nonKebab = regexp.MustCompile(`[^a-z0-9]+`)

func kebab(s string) string {
	return strings.Trim(nonKebab.ReplaceAllString(strings.ToLower(s), "-"), "-")
}
