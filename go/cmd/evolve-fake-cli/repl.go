package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// replBootMarkers is the single line the fake prints when it is REPL-ready.
// It deliberately contains EVERY shipped tmux driver's hardcoded boot marker
// (claude ❯, codex ›, agy "? for shortcuts", ollama ">>> ") so one fake binary
// satisfies any driver's boot-ready capture-pane check. The drivers hardcode
// these markers and IGNORE the manifest prompt_marker (see the TODO in
// driver_claudetmux.go and the literals in driver_{codex,agy,ollama}tmux.go),
// so matching them here — not via a manifest override — is the only thing that
// works. The trailing sentinel is a stable, greppable token for unit tests.
// It is fixed (never env-passed, since env does not propagate into tmux) and
// is not a substring of any launch flag, so the echoed launch command cannot
// trip a false boot-ready (the hazard documented in tmux_repl_integration_test.go).
const replBootMarkers = "❯ › ? for shortcuts >>> evolve-fake-repl-ready"

// runREPL serves a persistent REPL for the tmux drivers. It prints the boot
// marker, then reads the pasted prompt line-by-line from stdin. Once it has
// both an agent heading (phase) AND the "- workspace:" cycle-context line, it
// resolves the artifact path from workspace+basename and writes the phase
// artifact(s), then reprints the marker (ready for a possible next turn on a
// named session). It loops until stdin closes (the driver kills the session).
//
// Triggering on the workspace line — not on any absolute path in the body —
// guarantees we write to workspace/<basename> rather than to a stray upstream
// path the prompt may mention (e.g. a builder prompt that references
// scout-report.md). EOF fallback handles a prompt with no workspace line.
func runREPL(stdin io.Reader, stdout, stderr io.Writer, verdict string) int {
	fmt.Fprintln(stdout, replBootMarkers)

	var buf strings.Builder
	acted := false
	sc := bufio.NewScanner(stdin)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20) // tolerate long pasted prompt lines
	for sc.Scan() {
		line := sc.Text()
		buf.WriteString(line)
		buf.WriteString("\n")

		phase := detectPhaseFromPrompt(buf.String())
		if phase == "" || !workspaceLineRE.MatchString(buf.String()) {
			continue
		}
		artifactPath := resolveArtifactPath(buf.String(), phase)
		if artifactPath == "" {
			continue
		}
		if emitREPLArtifacts(phase, artifactPath, verdict, stdout, stderr) {
			acted = true
		}
		fmt.Fprintln(stdout, replBootMarkers) // ready for the next turn
		buf.Reset()
	}

	// EOF fallback: a prompt that never carried a workspace line but did
	// mention an absolute artifact path still gets served once.
	if !acted {
		if phase := detectPhaseFromPrompt(buf.String()); phase != "" {
			if artifactPath := resolveArtifactPath(buf.String(), phase); artifactPath != "" {
				emitREPLArtifacts(phase, artifactPath, verdict, stdout, stderr)
			}
		}
	}
	return 0
}

// emitREPLArtifacts writes the phase artifact(s) for one REPL turn. Failures
// are logged but never abort the REPL (the driver's artifact-wait timeout is
// the real backstop). Returns true if at least one artifact was written.
func emitREPLArtifacts(phase, artifactPath, verdict string, stdout, stderr io.Writer) bool {
	files, err := artifactsFor(phase, artifactPath, verdict)
	if err != nil {
		fmt.Fprintf(stderr, "fake-cli(repl): artifactsFor(%s): %v\n", phase, err)
		return false
	}
	if err := writeArtifacts(files); err != nil {
		fmt.Fprintf(stderr, "fake-cli(repl): %v\n", err)
		return false
	}
	fmt.Fprintf(stdout, "fake-cli(repl): wrote %d artifact(s) for phase=%s\n", len(files), phase)
	return true
}

// writeArtifacts creates parent dirs and writes each artifact. Shared by the
// headless (run) and REPL paths.
func writeArtifacts(files map[string]string) error {
	for path, content := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	return nil
}
