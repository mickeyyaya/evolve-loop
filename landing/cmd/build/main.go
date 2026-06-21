// Command build renders the five landing-page versions + gallery into dist/.
// Run from the landing/ directory: `go run ./cmd/build`.
//
// All logic lives in the tested internal/buildsite package; this is just the
// configuration of which versions exist and where files live.
package main

import (
	"fmt"
	"os"

	"evolveloop-landing/internal/buildsite"
)

func main() {
	cfg := buildsite.Config{
		ContentPath:  "shared/content.json",
		TemplateGlob: "templates/*.html",
		AssetsDir:    "assets",
		OutDir:       "dist",
		Gallery:      "gallery",
		Versions: []buildsite.Version{
			{Slug: "luminous", Title: "Luminous Minimal", Tagline: "Light, Apple-white, calm authority.", Template: "luminous"},
			{Slug: "noir", Title: "Keynote Noir", Tagline: "Dark, cinematic spotlight.", Template: "noir"},
			{Slug: "editorial", Title: "Editorial Serif", Tagline: "A warm, thoughtful manifesto.", Template: "editorial"},
			{Slug: "blueprint", Title: "Technical Blueprint", Tagline: "Terminal-grid, engineer-native.", Template: "blueprint"},
			{Slug: "aurora-glass", Title: "Aurora Glass", Tagline: "Liquid-glass, modern Apple.", Template: "aurora"},
		},
	}

	written, err := buildsite.Build(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "build error:", err)
		os.Exit(1)
	}
	fmt.Printf("built %d files into %s/\n", len(written), cfg.OutDir)
	for _, w := range written {
		fmt.Println("  ", w)
	}
}
