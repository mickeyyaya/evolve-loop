package scopedelta

import (
	"fmt"
	"path"
	"strings"
)

type Surface string

const (
	SurfaceSubject Surface = "subject"
	SurfaceSignal  Surface = "signal"
)

type Effect string

const (
	EffectUnknown  Effect = ""
	EffectTightens Effect = "tightens"
	EffectLoosens  Effect = "loosens"
)

type Corroboration struct {
	FailsWithout bool   `json:"fails_without,omitempty"`
	Command      string `json:"command,omitempty"`
	QueuedItemID string `json:"queued_item_id,omitempty"`
	CoverageLine string `json:"coverage_line,omitempty"`
}

func (c Class) isMechanicallyEstablished() bool {
	return c == ClassClosure || c == ClassInScope
}

func (c Corroboration) present() bool {
	return (c.FailsWithout && strings.TrimSpace(c.Command) != "") ||
		strings.TrimSpace(c.QueuedItemID) != "" ||
		strings.TrimSpace(c.CoverageLine) != ""
}

var signalDirs = []string{
	"go/acs/",
	"agents/", ".agents/",
	"skills/",
	".evolve/policy",
	".evolve/profiles",
	".evolve/runs/",
	".evolve/evals/",
	"go/internal/policy/",
	"go/internal/config/",
	"go/internal/guards/",
	"go/internal/commentaudit/",
	"go/internal/commitgate/",
	"go/internal/core/",
	"go/internal/phases/audit/",
	"go/internal/phases/ship/",
	".github/workflows/",
}

var signalFileHints = []string{"_test.go", ".jsonl", "-baseline.json", "predicates_test.go",
	// Gate configuration is apparatus: an enrollment edit can disable another package's coverage gate.
	".apicover-enforce", "go.mod", "go.sum"}

func SurfaceOf(p string) Surface {
	for _, d := range signalDirs {
		if strings.HasPrefix(p, d) {
			return SurfaceSignal
		}
	}
	base := path.Base(p)
	for _, h := range signalFileHints {
		if strings.HasSuffix(base, h) {
			return SurfaceSignal
		}
	}
	isGateImplementation := strings.Contains(p, "/gate") || strings.HasSuffix(base, "_gate.go") ||
		strings.Contains(p, "/phases/ship/") || strings.Contains(p, "repocontract")
	if isGateImplementation {
		return SurfaceSignal
	}
	return SurfaceSubject
}

func Admissible(e Entry) error {
	if e.Disposition != DispositionKeep {
		return nil
	}
	if e.Class.isMechanicallyEstablished() {
		return nil
	}
	if SurfaceOf(e.Path) == SurfaceSignal && e.Effect == EffectUnknown {
		return fmt.Errorf("scopedelta: %s: a KEEP on the judging apparatus must DECLARE its direction (tightens|loosens) — an unset direction is a claim of safety nobody made", e.Path)
	}
	if SurfaceOf(e.Path) == SurfaceSignal && e.Effect == EffectLoosens {
		return fmt.Errorf("scopedelta: %s: refusing to KEEP a change that LOOSENS the apparatus judging this cycle — carve it instead so the decision is made by a pass with nothing riding on it (the change may well be right; the producer is the wrong adjudicator)", e.Path)
	}
	if !e.Corroboration.present() {
		return fmt.Errorf("scopedelta: %s: a KEEP needs corroboration the author did not write — an executed counterfactual (fails_without + command), a pre-existing queued item, or a coverage line proving the branch runs; narrative alone is what an agent can always produce", e.Path)
	}
	if e.Corroboration.FailsWithout && strings.TrimSpace(e.Corroboration.Command) == "" {
		return fmt.Errorf("scopedelta: %s: fails_without with no command is an assertion in the costume of evidence — name what was run", e.Path)
	}
	return nil
}

func GamingSignals(entries []Entry) []string {
	const minEntriesForMajority = 3
	if len(entries) < minEntriesForMajority {
		return nil
	}
	var signalPaths, loosening, uncorroboratedKeeps int
	for _, e := range entries {
		if e.Class.isMechanicallyEstablished() {
			continue
		}
		if SurfaceOf(e.Path) == SurfaceSignal {
			signalPaths++
		}
		if e.Effect == EffectLoosens {
			loosening++
		}
		if e.Disposition == DispositionKeep && !e.Corroboration.present() {
			uncorroboratedKeeps++
		}
	}
	var out []string
	if isMajority(signalPaths, len(entries)) {
		out = append(out, fmt.Sprintf("signal-heavy delta: %d of %d out-of-scope paths edit the judging apparatus rather than the code being judged", signalPaths, len(entries)))
	}
	if isMajority(loosening, len(entries)) {
		out = append(out, fmt.Sprintf("loosening pattern: %d of %d changes move the bar DOWN — the direction nobody takes to make their own work harder", loosening, len(entries)))
	}
	if isMajority(uncorroboratedKeeps, len(entries)) {
		out = append(out, fmt.Sprintf("narrative-only deltas: %d of %d keeps rest on the author's account with nothing outside it", uncorroboratedKeeps, len(entries)))
	}
	return out
}
