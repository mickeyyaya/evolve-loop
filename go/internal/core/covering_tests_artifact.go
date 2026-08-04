package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs"
)

// coveringTestsArtifact is the run-dir filename the test-amplification phase
// spec declares as an input (.evolve/phases/test-amplification/phase.json).
const coveringTestsArtifact = "covering-tests.md"

// coveringTestsMaxBytes caps the injected corpus. The artifact exists to SHRINK
// the phase's context (5.4M cache-read tokens/run from a blind whole-repo
// search); an unbounded list from a sweeping cycle would reintroduce the very
// cost it removes, so a large set is truncated with a loud note rather than
// injected whole. ~16 KiB is roughly 400 paths — far beyond any real cycle's
// changed-package footprint, so the cap is a backstop, not a routine trim.
const coveringTestsMaxBytes = 16 * 1024

// writeCoveringTests materialises the covering-test corpus for this cycle into
// the run dir, after the build worktree is normalized and before
// test-amplification runs.
//
// Why here: test-amplification is forbidden from reading the diff or the
// implementation, and its spec hands it only tdd-contract.md + build-report.md,
// so it must Grep/Glob the whole repo to find which existing tests cover the
// paths it was given — the pipeline's largest per-run context outlier. Deriving
// the set deterministically (changed packages → their _test.go files) is Rule-5
// work: mechanical, so it belongs in code, not in an agent's search budget. A
// list of test-file PATHS is not the diff and not the implementation, so the
// phase's black-box constraint is untouched.
//
// Best-effort throughout, like every other normalize step: no worktree, no
// workspace, an underivable diff, or an empty corpus writes nothing at all, and
// the phase then behaves exactly as it does today (its input resolver skips a
// missing file). A write failure is logged, never fatal.
func writeCoveringTests(ctx context.Context, worktree, workspace string) {
	if worktree == "" || workspace == "" {
		return
	}
	pkgs := changedGoTestPackages(changedWorktreePaths(ctx, worktree))
	// Widen to the packages that DIRECTLY import the changed ones: a package
	// whose _test.go imports the changed code IS its covering test, and walking
	// only the changed dirs cannot see it. Fail-open (nil on any derivation
	// problem), so the corpus degrades to the changed packages alone.
	pkgs = append(pkgs, changedpkgs.DirectImporters(worktree, pkgs)...)
	files := changedpkgs.CoveringTests(worktree, pkgs)
	if len(files) == 0 {
		return
	}
	body, omitted := renderCoveringTests(files)
	path := filepath.Join(workspace, coveringTestsArtifact)
	if err := atomicwrite.Bytes(path, []byte(body)); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN covering-tests: write %s: %v (test-amplification falls back to its own search)\n", path, err)
	}
	// No silent caps: the in-artifact note is visible only to the agent, so an
	// operator reading the cycle log could not tell a trimmed corpus from a
	// complete one — which makes the before/after token measurement this corpus
	// exists to prove uninterpretable. Silent when nothing was dropped: a
	// warning on every clean cycle trains the operator to ignore the real one.
	if omitted > 0 {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN covering-tests: corpus TRUNCATED — %d of %d covering-test paths omitted (cap %d bytes); test-amplification sees a partial corpus\n",
			omitted, len(files), coveringTestsMaxBytes)
	}
}

// coveringTestsDeriver is the derivation seam ensureCoveringTests calls. A
// package var (not a parameter) so the tests can substitute a deterministic
// corpus without standing up a git fixture, matching worktreeAddRetrySleep's
// precedent — unexported, so nothing outside this package can swap it.
var coveringTestsDeriver = writeCoveringTests

// ensureCoveringTests guarantees the corpus is either ON DISK or ANNOUNCED
// before test-amplification reads for it.
//
// The hole it closes: writeCoveringTests is called from inside
// `if completed == PhaseBuild` (phase_bindings.go), so on any path where
// test-amplification runs without a fresh build completion in the SAME process
// — a resume past build, or a future insertion after a different phase — the
// artifact is simply absent and the phase silently reverts to the whole-repo
// Grep the corpus exists to remove.
//
// SILENT is the defect, not slow. The corpus exists to make a before/after
// token measurement interpretable (5.4M cache-read tokens/run baseline); a run
// that quietly degrades to the unscoped behaviour produces a number nobody can
// read, and no operator signal says which run they are looking at. This is the
// same reasoning already applied one layer down, where a TRUNCATED corpus warns
// loudly for exactly that reason.
//
// Fail-open is preserved end to end: an underivable diff still leaves the phase
// working exactly as it does today. It just says so.
//
// A corpus already on disk is a silent no-op — the build path's fresh
// derivation stays authoritative, and this never pays a second `go list`.
func ensureCoveringTests(ctx context.Context, worktree, workspace string) {
	if worktree == "" || workspace == "" {
		return
	}
	path := filepath.Join(workspace, coveringTestsArtifact)
	if _, err := os.Stat(path); err == nil {
		return
	}
	coveringTestsDeriver(ctx, worktree, workspace)
	if _, err := os.Stat(path); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN covering-tests: no corpus at %s — test-amplification falls back to an UNSCOPED whole-repo search; this run's context-token figure is not comparable to a scoped one\n", path)
	}
}

// renderCoveringTests formats the corpus as the markdown the phase reads, byte-
// capped: paths are emitted until the cap would be exceeded, and a truncation
// note naming the omitted count is appended so a trimmed corpus can never be
// mistaken for a complete one (a silently short list would make the agent skip
// real covering tests it was supposed to amplify).
//
// It also RETURNS that omitted count — the cap arithmetic lives here and nowhere
// else, so this is the single source for both the in-artifact note and the
// operator's warning. Re-deriving it at the write seam (e.g. by scanning the
// rendered string for "TRUNCATED:") would put two answers to one question in the
// tree.
func renderCoveringTests(files []string) (string, int) {
	var b strings.Builder
	b.WriteString("# Covering Tests — changed packages\n\n")
	b.WriteString("Existing `_test.go` files in the packages this cycle changed. This is the\n")
	b.WriteString("in-scope test corpus: amplify against these instead of searching the repo.\n\n")
	header := b.Len()
	for i, f := range files {
		line := "- `" + sanitizeCorpusPath(f) + "`\n"
		if b.Len()+len(line) > coveringTestsMaxBytes && b.Len() > header {
			fmt.Fprintf(&b, "\n> TRUNCATED: %d of %d covering-test paths omitted (corpus exceeded %d bytes).\n",
				len(files)-i, len(files), coveringTestsMaxBytes)
			return b.String(), len(files) - i
		}
		b.WriteString(line)
	}
	return b.String(), 0
}

// sanitizeCorpusPath neutralizes a path before it is interpolated into
// covering-tests.md, a document .evolve/phases/test-amplification/agent.md
// declares AUTHORITATIVE.
//
// The path set is attacker-influenced by construction — it is derived from
// filenames in the worktree diff, and any commit can add a file with a hostile
// name. A backtick closes the code span; a newline injects top-level markdown
// (`# SYSTEM ...`) straight into a code-writing agent's authoritative input,
// subverting exactly the anti-bias isolation (no diff, no implementation) the
// corpus was introduced to preserve.
//
// The invariant is structural, not cosmetic: one list item per path, no
// breakout. Replacing rather than escaping keeps the byte-cap arithmetic above
// exact (one rune in, one rune out) and keeps ordinary Go test paths — which
// contain none of these runes — verbatim.
func sanitizeCorpusPath(p string) string {
	return strings.Map(func(r rune) rune {
		if r == '`' || unicode.IsControl(r) {
			return '_'
		}
		return r
	}, p)
}
