# Evolve Loop — Landing Page

Five distinct, elegant ("Steve Jobs style") landing-page concepts for the Evolve Loop
product, plus a gallery to compare them. Built as a small, **tested, Go-only static-site
generator**: one content source of truth → bespoke per-style templates → static HTML.

## The five versions

| Slug | Style | Feel |
|------|-------|------|
| `luminous` | Luminous Minimal | Light, Apple-white, calm authority |
| `noir` | Keynote Noir | Dark, cinematic spotlight (the flagship) |
| `editorial` | Editorial Serif | Warm magazine / field-guide |
| `blueprint` | Technical Blueprint | Terminal-grid, engineer-native |
| `aurora-glass` | Aurora Glass | visionOS / Liquid Glass, modern Apple |

Each is a **distinct layout** telling the same story (hero → the bottleneck moved → AI-for-judgment
/ code-for-the-verdict → the 5 pillars → comparison → the Cycle-61 incident → quick start → CTA),
driven entirely by the content model so facts never drift across versions.

## Architecture (why it's trustworthy)

```
shared/content.json   ← single source of truth (facts, copy, links)
   │  internal/content     typed model + Load()/Validate()  (tested)
   ▼
templates/*.html      ← one {{define}} per version + shared partials (copyjs/reveal/verdictjs)
   │  internal/render      template engine, strict missing-key (tested)
   │  internal/buildsite   renders each version + gallery, copies assets (tested)
   ▼
dist/                 ← self-contained static pages you open in a browser
```

- **TDD throughout** — `internal/content`, `internal/render`, `internal/buildsite`, and the
  `genimage` pure logic were written test-first (`go test ./...`).
- **No duplication** — every page binds `.Site.*`; copy/facts live once in `content.json`.
- **Strict rendering** — `missingkey=error`, so a typo'd field fails the build, never ships `<no value>`.

## Commands (run from this `landing/` directory)

```bash
go test ./...                 # all unit tests
go run ./cmd/build            # render content + templates → dist/
go run ./cmd/serve            # preview dist/ at http://127.0.0.1:8077
GEMINI_API_KEY=… go run ./cmd/genimage --prompt "…" --out assets/<v>/hero.png --aspect 16:9
```

Imagery is generated with Google's **Nano Banana** image models (`gemini-3-pro-image`) via
`cmd/genimage`. Assets live in `assets/<version>/`.

## Status

Preview deliverable on the `landing-page` branch (isolated worktree). Pick a winning version,
then it gets final polish + a deploy on a follow-up.
