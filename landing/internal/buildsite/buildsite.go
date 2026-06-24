// Package buildsite renders the landing site: it loads the single content model,
// parses the shared templates, executes one template per version into
// dist/<slug>/index.html, renders the gallery, and copies image assets.
package buildsite

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"evolve-loop-landing/internal/content"
	"evolve-loop-landing/internal/render"
)

// Version is one style of the landing page.
type Version struct {
	Slug     string // url + asset folder, e.g. "noir"
	Title    string // display name, e.g. "Keynote Noir"
	Tagline  string // one-line description for the gallery
	Template string // name of the {{define}} block to execute
}

// RootFile is a static file copied verbatim into the dist root, e.g. the
// install.sh installer served at https://<pages>/install.sh.
type RootFile struct {
	Src string // path relative to the build working dir (landing/)
	Dst string // path relative to OutDir
}

// Config controls a build.
type Config struct {
	ContentPath  string
	TemplateGlob string
	AssetsDir    string
	OutDir       string
	Gallery      string     // gallery template name (optional)
	RootFiles    []RootFile // static files copied to the dist root (optional)
	Versions     []Version
}

// pageData is the model passed to every template.
type pageData struct {
	Site     *content.Site
	Version  Version
	Versions []Version
	Assets   string // relative path from the page to its assets folder
}

// Build renders all pages and returns the paths it wrote.
func Build(cfg Config) ([]string, error) {
	site, err := content.Load(cfg.ContentPath)
	if err != nil {
		return nil, err
	}
	tmpl, err := render.New().ParseGlob(cfg.TemplateGlob)
	if err != nil {
		return nil, fmt.Errorf("parse templates %q: %w", cfg.TemplateGlob, err)
	}

	var written []string
	for _, v := range cfg.Versions {
		html, err := exec(tmpl, v.Template, pageData{
			Site:     site,
			Version:  v,
			Versions: cfg.Versions,
			Assets:   "../assets/" + v.Slug,
		})
		if err != nil {
			return nil, fmt.Errorf("render version %q: %w", v.Slug, err)
		}
		path := filepath.Join(cfg.OutDir, v.Slug, "index.html")
		if err := writeFile(path, html); err != nil {
			return nil, err
		}
		written = append(written, path)
	}

	if cfg.Gallery != "" {
		html, err := exec(tmpl, cfg.Gallery, pageData{
			Site:     site,
			Versions: cfg.Versions,
			Assets:   "assets",
		})
		if err != nil {
			return nil, fmt.Errorf("render gallery: %w", err)
		}
		path := filepath.Join(cfg.OutDir, "index.html")
		if err := writeFile(path, html); err != nil {
			return nil, err
		}
		written = append(written, path)
	}

	if cfg.AssetsDir != "" {
		if err := copyDir(cfg.AssetsDir, filepath.Join(cfg.OutDir, "assets")); err != nil {
			return nil, fmt.Errorf("copy assets: %w", err)
		}
	}

	for _, rf := range cfg.RootFiles {
		dst := filepath.Join(cfg.OutDir, rf.Dst)
		if err := copyFile(rf.Src, dst); err != nil {
			return nil, fmt.Errorf("copy root file %s: %w", rf.Src, err)
		}
		written = append(written, dst)
	}
	return written, nil
}

func exec(tmpl interface {
	ExecuteTemplate(io.Writer, string, any) error
}, name string, data pageData) ([]byte, error) {
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir for %q: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %q: %w", path, err)
	}
	return nil
}

// relFn computes the path of an entry relative to the walk root. It is a
// package-level seam so a test can exercise copyDir's rel-error branch, which
// the real filesystem never reaches (both arguments are always absolute and
// share a root). Behaviour is identical to filepath.Rel at runtime.
var relFn = filepath.Rel

// copyDir recursively copies regular files from src into dst.
func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := relFn(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	// Close explicitly so a write-back/flush error (ENOSPC, NFS) surfaces — a
	// truncated install.sh must fail the build, not deploy silently.
	return out.Close()
}
