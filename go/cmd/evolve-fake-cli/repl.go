package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const replBootMarkers = "❯ › ? for shortcuts >>> evolve-fake-repl-ready"

const maxPastedPromptLineBytes = 1 << 20

func runREPL(stdin io.Reader, stdout, stderr io.Writer, verdict string) int {
	fmt.Fprintln(stdout, replBootMarkers)

	var buf strings.Builder
	servedAnyTurn := false
	sc := bufio.NewScanner(stdin)
	sc.Buffer(make([]byte, 0, 64*1024), maxPastedPromptLineBytes)
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
			servedAnyTurn = true
		}
		fmt.Fprintln(stdout, replBootMarkers)
		buf.Reset()
	}

	if !servedAnyTurn {
		serveOnceAtEOF(buf.String(), verdict, stdout, stderr)
	}
	return 0
}

func serveOnceAtEOF(prompt, verdict string, stdout, stderr io.Writer) {
	if phase := detectPhaseFromPrompt(prompt); phase != "" {
		if artifactPath := resolveArtifactPath(prompt, phase); artifactPath != "" {
			emitREPLArtifacts(phase, artifactPath, verdict, stdout, stderr)
		}
	}
}

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
