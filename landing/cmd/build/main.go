// Command build renders the five landing-page versions + gallery into dist/.
// Run from the landing/ directory: `go run ./cmd/build`.
//
// All logic lives in the tested internal/buildsite package; this is just the
// configuration of which versions exist and where files live.
package main

import (
	"fmt"
	"io"
	"os"

	"evolve-loop-landing/internal/buildsite"
)

// config returns the build configuration: the five landing-page versions plus
// the gallery, and where content/templates/assets/output live.
func config() buildsite.Config {
	return buildsite.Config{
		ContentPath:  "shared/content.json",
		TemplateGlob: "templates/*.html",
		AssetsDir:    "assets",
		OutDir:       "dist",
		Gallery:      "gallery",
		// Serve the one-line installer at /install.sh (single source of truth:
		// the repo-root install.sh; build runs from landing/, so ../ reaches it).
		RootFiles: []buildsite.RootFile{{Src: "../install.sh", Dst: "install.sh"}, {Src: "shared/llms.txt", Dst: "llms.txt"}},
		Versions: []buildsite.Version{
			{Slug: "luminous", Title: "Luminous Minimal", Tagline: "Light, Apple-white, calm authority.", Template: "luminous"},
			{Slug: "noir", Title: "Keynote Noir", Tagline: "Dark, cinematic spotlight.", Template: "noir"},
			{Slug: "editorial", Title: "Editorial Serif", Tagline: "A warm, thoughtful manifesto.", Template: "editorial"},
			{Slug: "blueprint", Title: "Technical Blueprint", Tagline: "Terminal-grid, engineer-native.", Template: "blueprint"},
			{Slug: "aurora-glass", Title: "Aurora Glass", Tagline: "Liquid-glass, modern Apple.", Template: "aurora"},
		},
	}
}

// run builds the site, prints the written file list to stdout, and returns a
// process exit code (1 on error, 0 on success). Build errors are reported to
// stderr to match the original command behavior.
func run(stdout io.Writer) int {
	cfg := config()
	written, err := buildsite.Build(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "build error:", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "built %d files into %s/\n", len(written), cfg.OutDir)
	for _, w := range written {
		_, _ = fmt.Fprintln(stdout, "  ", w)
	}
	return 0
}

// osExit is a seam so a test can verify main() forwards run()'s exit code
// without terminating the test process. At runtime it is exactly os.Exit.
var osExit = os.Exit

func main() {
	osExit(run(os.Stdout))
}
