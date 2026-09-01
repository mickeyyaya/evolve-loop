package phasecoherence

import (
	"errors"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// tracked_artifact_hygiene_test.go — the "code and docs only" upstream contract
// (2026-08-27 repo-hygiene sweep).
//
// The failure class this test pins: the pipeline writes its runtime records
// WORKSPACE-RELATIVE (sessionrecord.FileName, bridge/tmux_inject.go's
// .bridge-inbox/<agent>-inject.txt, cmd_models_live.go's model-classifier-*,
// the ACS predicates' `go test -coverprofile=coverage.<pkg><cycle>.txt`), and
// the resolved workspace is SOMETIMES THE REPO ROOT — a console run, a nested
// sandbox, a lane whose worktree is the project dir. A broad `git add -A` then
// sweeps the artifact into a commit, where it ships to every clone and every
// plugin-marketplace install, forever. Twenty such files were found tracked on
// 2026-08-27 (~655 KB), the oldest dating to cycle-104.
//
// WHY A TEST AND NOT JUST A .gitignore RULE: an ignore rule only governs a
// FUTURE birth. It is silent about a file that was committed BEFORE the rule
// existed (git tracks it regardless) or that arrives via `git add -f`. Every
// artifact below was already matched-or-matchable by the spirit of the ignore
// ladder and still sat in the tree for months. The ladder states the intent;
// this test is the enforcement, and it runs in the ship-time repo-contract pack
// (ship/repocontract.go repoContractPackages includes ./internal/phasecoherence/...)
// so a regression fails the cycle that introduced it rather than a later audit.
//
// Kept deliberately narrow: only artifact classes with a KNOWN production
// writer are listed. This is not a general "no binaries" or "no large files"
// guard — staged-binary policy already lives in ship/binary_staging_guard.go
// (which allowlists go/bin/** and the marketplace-distributed go/evolve), and
// duplicating it here would give two places to update and one to forget.
func TestNoRuntimeArtifactsTracked(t *testing.T) {
	t.Parallel()
	root := repoRootForPairing(t)

	// rules are the runtime-artifact classes. Every entry names the production
	// writer, because a rule whose writer nobody can point at is a rule nobody
	// can safely delete later.
	rules := []struct {
		re   *regexp.Regexp
		why  string
		rule string // the .gitignore line that governs new births of this class
	}{
		{
			re:   regexp.MustCompile(`\.log$`),
			why:  "run log (loop/release/bridge stdout capture)",
			rule: "/*.log",
		},
		{
			re:   regexp.MustCompile(`(^|/)coverage\.[^/]*\.txt$`),
			why:  "ACS coverage output — `go test -coverprofile=` + `go tool cover -func` inside go/acs/cycle*/predicates_test.go regenerate it",
			rule: "coverage.*.txt",
		},
		{
			re:   regexp.MustCompile(`(^|/)tmux-sessions\.jsonl$`),
			why:  "per-workspace tmux session registry — sessionrecord.FileName",
			rule: "/tmux-sessions.jsonl",
		},
		{
			re:   regexp.MustCompile(`(^|/)model-classifier-`),
			why:  "model-classifier probe artifact — cmd_models_live.go writes it to the agent workspace",
			rule: "/model-classifier-*",
		},
		{
			// Exact root path, not a class: found tracked in the 2026-08-27
			// sweep with NO identified production writer (2026-09-01 review
			// re-confirmed: zero references across go/, skills/, .github/).
			// Kept as a rule anyway — the guard's contract is "every artifact
			// this sweep removed stays removed", and an unmatched file was
			// re-addable without any test going red.
			re:   regexp.MustCompile(`^lint-baseline\.txt$`),
			why:  "lint-tool baseline snapshot at the repo root — no production writer identified; operator/tool-generated",
			rule: "/lint-baseline.txt",
		},
		{
			re:   regexp.MustCompile(`(^|/)\.bridge-inbox/`),
			why:  "bridge inject scratch — bridge/tmux_inject.go writes <workspace>/.bridge-inbox/<agent>-inject.txt",
			rule: ".bridge-inbox/",
		},
		{
			// The ROOT .evolve/ corpus is governed by its own re-include ladder
			// (profiles, phases, evals, inbox, policy.json — see
			// gitignore_birth_test.go) and legitimately ships. A NESTED one is
			// always runtime state that a phase created in its cwd.
			re:   regexp.MustCompile(`/\.evolve/`),
			why:  "nested .evolve/ runtime state created in a phase's cwd",
			rule: "*/**/.evolve/",
		},
	}

	// allowed are exact paths that match a rule above but are genuine repo
	// content. Every entry carries its reason — an unexplained allowlist entry
	// is indistinguishable from an unnoticed regression.
	allowed := map[string]string{
		"go/internal/logfilter/testdata/streamjson-input.log": "test fixture: the recorded stream-json the logfilter tests parse, not a run log",
	}

	out, err := exec.Command("git", "-C", root, "ls-files").Output()
	if err != nil {
		// exec.ExitError.Error() is just "exit status N"; the reason an
		// operator needs is on stderr (same idiom as repostate/tracked.go).
		var ee *exec.ExitError
		if errors.As(err, &ee) && len(ee.Stderr) > 0 {
			t.Fatalf("git ls-files: %v: %s", err, ee.Stderr)
		}
		t.Fatalf("git ls-files: %v", err)
	}
	tracked := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(tracked) < 100 {
		t.Fatalf("git ls-files returned only %d paths — the probe itself is broken", len(tracked))
	}

	var violations []string
	usedAllowance := map[string]bool{}
	for _, p := range tracked {
		if p == "" {
			continue
		}
		for _, r := range rules {
			if !r.re.MatchString(p) {
				continue
			}
			if _, ok := allowed[p]; ok {
				usedAllowance[p] = true
				break
			}
			violations = append(violations, p+"\n      class: "+r.why+
				"\n      fix:   git rm --cached "+p+"   (ignore ladder: "+r.rule+")")
			break
		}
	}

	if len(violations) > 0 {
		sort.Strings(violations)
		t.Errorf("%d runtime artifact(s) are tracked — the upstream tree must hold only code and docs.\n"+
			"These are regenerated by the pipeline and ship to every clone:\n\n  - %s\n\n"+
			"If one of these is genuine repo content, add it to `allowed` in this file WITH its reason.",
			len(violations), strings.Join(violations, "\n  - "))
	}

	// A stale allowance is its own defect: it silently pre-approves a future
	// regression at that exact path. Same self-pruning discipline as the
	// persona-strip guard's exception list.
	for p := range allowed {
		if !usedAllowance[p] {
			t.Errorf("allowlist entry %q no longer matches any tracked artifact path — delete it "+
				"(a stale allowance pre-approves a future regression at that path)", p)
		}
	}
}
