package bridge

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"path"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/attemptpostmortem"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
)

const (
	ParserPane      = "pane"
	CollectorTokens = "tokens"
	CapabilityTools = "tools"
	CapabilityEdit  = "edit"
)

type TranscriptSpec struct {
	Format string `json:"format"`
}

var parserFormats = map[string]func(string) bool{
	ParserPane: knownPaneFormat,
}

var collectorFormats = map[string]func(string) bool{
	CollectorTokens: tokenusage.HasCollector,
}

var knownCapabilities = []string{CapabilityTools, CapabilityEdit}

func knownPaneFormat(format string) bool {
	_, ok := panestream.Profiles[format]
	return ok
}

func (m Manifest) TranscriptFormat() string {
	if m.Transcript == nil {
		return ""
	}
	return m.Transcript.Format
}

func (m Manifest) HasCapability(capability string) bool {
	return slices.Contains(m.Capabilities, capability)
}

func checkFormats(cli string, m Manifest, stderr io.Writer) error {
	if m.Capabilities == nil {
		fmt.Fprintf(stderr, "[bridge:manifest] cli=%s declares no capabilities: it has no tool use. A manifest written before X9 also lacks parsers, collectors, transcript and context_window; add them or name a base.\n", cli)
	}
	return validateFormats(cli, m)
}

func validateFormats(cli string, m Manifest) error {
	return errors.Join(
		validateFormatMap(cli, "parsers", m.Parsers, parserFormats),
		validateFormatMap(cli, "collectors", m.Collectors, collectorFormats),
		validateTranscript(cli, m.Transcript),
		validateContextWindow(cli, m.ContextWindow),
		validateCapabilities(cli, m.Capabilities),
		validateHomeReads(cli, m.HomeReads),
	)
}

func validateTranscript(cli string, spec *TranscriptSpec) error {
	if spec == nil || attemptpostmortem.KnownTranscriptFormat(spec.Format) {
		return nil
	}
	return fmt.Errorf("bridge:manifest: transcript.format %q for cli=%s is not a known format id", spec.Format, cli)
}

func validateContextWindow(cli string, window int) error {
	if window >= 0 {
		return nil
	}
	return fmt.Errorf("bridge:manifest: context_window for cli=%s is %d; it counts tokens, so it cannot be negative", cli, window)
}

func validateFormatMap(cli, field string, declared map[string]string, known map[string]func(string) bool) error {
	var errs []error
	for _, kind := range slices.Sorted(maps.Keys(declared)) {
		isKnown, ok := known[kind]
		switch {
		case !ok:
			errs = append(errs, fmt.Errorf("bridge:manifest: %s.%s for cli=%s is not a known kind; the kinds are %s", field, kind, cli, strings.Join(slices.Sorted(maps.Keys(known)), ", ")))
		case !isKnown(declared[kind]):
			errs = append(errs, fmt.Errorf("bridge:manifest: %s.%s for cli=%s names %q, which is not a known format id", field, kind, cli, declared[kind]))
		}
	}
	return errors.Join(errs...)
}

func validateCapabilities(cli string, capabilities []string) error {
	for _, c := range capabilities {
		if !slices.Contains(knownCapabilities, c) {
			return fmt.Errorf("bridge:manifest: capabilities entry %q for cli=%s is not a known capability; the capabilities are %s", c, cli, strings.Join(knownCapabilities, ", "))
		}
	}
	return nil
}

func validateHomeReads(cli string, reads []string) error {
	for _, r := range reads {
		rel, ok := strings.CutPrefix(r, "~/")
		if !ok || path.Clean(rel) == "." || slices.Contains(strings.Split(rel, "/"), "..") {
			return fmt.Errorf("bridge:manifest: home_reads entry %q for cli=%s must start with ~/, name a path below it and carry no .. segment", r, cli)
		}
		if secret, overlaps := secretOverlap(strings.ToLower(path.Clean(rel))); overlaps {
			return fmt.Errorf("bridge:manifest: home_reads entry %q for cli=%s overlaps the secret path ~/%s; the sandbox denies each secret before it grants a read", r, cli, secret)
		}
	}
	return nil
}

func secretOverlap(rel string) (string, bool) {
	for _, secret := range homeSecrets {
		if rel == secret || strings.HasPrefix(rel, secret+"/") || strings.HasPrefix(secret, rel+"/") {
			return secret, true
		}
	}
	return "", false
}
