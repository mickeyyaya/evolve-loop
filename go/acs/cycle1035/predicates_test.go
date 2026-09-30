//go:build acs

package cycle1035

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

type settingsFile struct {
	Hooks struct {
		PreToolUse []struct {
			Matcher string `json:"matcher"`
			Hooks   []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"PreToolUse"`
	} `json:"hooks"`
}

var matcherToken = regexp.MustCompile(`[A-Za-z]+`)

func guardPhaseWiring(t *testing.T) (wired, coversAgent bool) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf(".claude/settings.json not readable (mandatory config artifact): %v", err)
	}
	var sf settingsFile
	if err := json.Unmarshal(raw, &sf); err != nil {
		t.Fatalf(".claude/settings.json is not valid JSON: %v", err)
	}
	for _, entry := range sf.Hooks.PreToolUse {
		invokesGuardPhase := false
		for _, h := range entry.Hooks {
			if strings.Contains(h.Command, "guard phase") {
				invokesGuardPhase = true
				break
			}
		}
		if !invokesGuardPhase {
			continue
		}
		wired = true
		for _, tok := range matcherToken.FindAllString(entry.Matcher, -1) {
			if tok == "Agent" || tok == "Task" {
				coversAgent = true
			}
		}
	}
	return wired, coversAgent
}

func phaseGuardSourcePresent(t *testing.T) bool {
	t.Helper()
	root := acsassert.RepoRoot(t)
	_, err := os.Stat(filepath.Join(root, "go", "internal", "guards", "phase.go"))
	return err == nil
}

func TestC1035_001_PhaseGuardNotWiredNoop(t *testing.T) {
	wired, coversAgent := guardPhaseWiring(t)
	sourcePresent := phaseGuardSourcePresent(t)

	switch {
	case wired && coversAgent:
		return
	case !wired && !sourcePresent:
		return
	case wired && !coversAgent:
		t.Errorf("phase guard is a WIRED NO-OP: `evolve guard phase` is wired in " +
			".claude/settings.json but NO matcher covers the Agent/Task tool it branches on — " +
			"its one real branch can never fire (rewire must add an Agent matcher; retire must unwire it)")
	case !wired && sourcePresent:
		t.Errorf("`evolve guard phase` is no longer wired but go/internal/guards/phase.go " +
			"still exists — retire must remove the orphaned inert guard source (or rewire must re-wire it)")
	default:
		t.Errorf("unreachable wiring state: wired=%v coversAgent=%v sourcePresent=%v", wired, coversAgent, sourcePresent)
	}
}

func findGuardPhaseADR(t *testing.T) (string, bool) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	matches, err := filepath.Glob(filepath.Join(root, "docs", "architecture", "adr", "*guard-phase*.md"))
	if err != nil {
		t.Fatalf("glob ADR dir: %v", err)
	}
	if len(matches) == 0 {
		return "", false
	}
	return matches[0], true
}

func TestC1035_002_DecisionRecordedAsADR(t *testing.T) {
	path, ok := findGuardPhaseADR(t)
	if !ok {
		t.Fatalf("no docs/architecture/adr/*guard-phase*.md ADR exists — the rewire-or-retire fix " +
			"must be recorded as a real architecture decision, not a silent patch")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read ADR %s: %v", filepath.Base(path), err)
	}
	body := strings.ToLower(string(raw))

	if !strings.Contains(body, "rewire") || !strings.Contains(body, "retire") {
		t.Errorf("ADR %s does not weigh BOTH alternatives (must mention 'rewire' AND 'retire' to be a real decision)", filepath.Base(path))
	}
	if !strings.Contains(body, "internal/core") && !strings.Contains(body, "state machine") && !strings.Contains(body, "statemachine") {
		t.Errorf("ADR %s does not name the state machine (internal/core) as the authoritative phase-order enforcement point", filepath.Base(path))
	}
	if !strings.Contains(body, "status") {
		t.Errorf("ADR %s has no Status section — not a well-formed ADR", filepath.Base(path))
	}
}

type docSurfaceClaim struct {
	rel     string
	needles []string
}

var wsRun = regexp.MustCompile(`\s+`)

func normWS(s string) string { return wsRun.ReplaceAllString(s, " ") }

func TestC1035_003_NoStaleEnforcementDocClaim(t *testing.T) {
	_, coversAgent := guardPhaseWiring(t)
	if coversAgent {
		return
	}

	surfaces := []docSurfaceClaim{
		{
			rel:     filepath.Join("docs", "operations", "runtime-reference.md"),
			needles: []string{"`evolve guard phase` precondition whenever `cycle-state.json` exists"},
		},
		{
			rel:     "CLAUDE.md",
			needles: []string{"Phase gate at every transition", "`evolve guard phase`"},
		},
		{
			rel:     filepath.Join("skills", "loop", "SKILL.md"),
			needles: []string{"Phase transitions are enforced by", "`evolve guard phase`"},
		},
	}

	root := acsassert.RepoRoot(t)
	for _, s := range surfaces {
		raw, err := os.ReadFile(filepath.Join(root, s.rel))
		if err != nil {
			t.Errorf("doc surface %s not readable: %v", s.rel, err)
			continue
		}
		content := normWS(string(raw))
		allPresent := true
		for _, n := range s.needles {
			if !strings.Contains(content, normWS(n)) {
				allPresent = false
				break
			}
		}
		if allPresent {
			t.Errorf("%s still asserts `evolve guard phase` enforces phase transitions, but the guard is "+
				"NOT wired to fire on the Agent tool — retire must correct this claim (needles: %v)", s.rel, s.needles)
		}
	}
}
