package skillcheck

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

const (
	instructionsBegin = "<!-- GENERATED:instructions BEGIN"
	instructionsEnd   = "<!-- GENERATED:instructions END -->"
	sharedSource      = "AGENTS.md"
)

type instructionRender struct {
	target string
	source string
	body   func(string) (string, error)
}

func instructionRenders() []instructionRender {
	renders := []instructionRender{
		{target: "CLAUDE.md", source: sharedSource, body: sharedBody},
		{target: "GEMINI.md", source: sharedSource, body: sharedBody},
	}
	for _, reviewer := range []string{"architecture-reviewer", "code-reviewer", "code-simplifier", "go-reviewer"} {
		renders = append(renders, instructionRender{
			target: ".claude/agents/" + reviewer + ".md",
			source: "agents/evolve-diff-" + reviewer + ".md",
			body:   personaBody,
		})
	}
	return renders
}

func sharedBody(source string) (string, error) {
	at := strings.Index(source, "\n## ")
	if at < 0 {
		return "", errors.New("no level-2 heading starts the shared body")
	}
	return source[at+1:], nil
}

func personaBody(source string) (string, error) {
	_, body, err := prompts.ParseFrontmatter(source)
	return strings.TrimLeft(body, "\n"), err
}

func instructionBlock(source, body string) string {
	return instructionsBegin + " from " + source + " by `evolve skills generate`; edit the source, not this region -->\n" +
		strings.TrimRight(body, "\n") + "\n" + instructionsEnd + "\n"
}

func instructionDiffs(projectRoot string) ([]commandDiff, error) {
	return instructionDiffsFor(projectRoot, instructionRenders())
}

func renderedFileDiffs(projectRoot string) ([]commandDiff, error) {
	codex, err := codexManifestDiffs(projectRoot)
	if err != nil {
		return nil, err
	}
	instructions, err := instructionDiffs(projectRoot)
	if err != nil {
		return nil, err
	}
	return append(codex, instructions...), nil
}

func instructionDiffsFor(projectRoot string, renders []instructionRender) ([]commandDiff, error) {
	diffs := make([]commandDiff, 0, len(renders))
	for _, r := range renders {
		source, sourceErr := os.ReadFile(filepath.Join(projectRoot, filepath.FromSlash(r.source)))
		path := filepath.Join(projectRoot, filepath.FromSlash(r.target))
		current, targetErr := os.ReadFile(path)
		if errors.Is(sourceErr, os.ErrNotExist) || errors.Is(targetErr, os.ErrNotExist) {
			continue
		}
		if err := errors.Join(sourceErr, targetErr); err != nil {
			return nil, fmt.Errorf("read the render %s from %s: %w", r.target, r.source, err)
		}
		d, err := renderInstruction(r, path, string(source), string(current))
		if err != nil {
			return nil, err
		}
		diffs = append(diffs, d)
	}
	return diffs, nil
}

func renderInstruction(r instructionRender, path, source, current string) (commandDiff, error) {
	body, err := r.body(source)
	if err != nil {
		return commandDiff{}, fmt.Errorf("%s: %w", r.source, err)
	}
	next, err := SpliceMarkedRegion(current, instructionBlock(r.source, body), instructionsBegin, instructionsEnd, "")
	if err != nil {
		return commandDiff{}, fmt.Errorf("%s: %w", r.target, err)
	}
	return commandDiff{rel: r.target, path: path, next: next, drifted: next != current}, nil
}
