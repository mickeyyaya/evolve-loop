// Command genimage generates an image via Google's Gemini image models
// (the "nano-banana" family) using the first-party Generative Language API.
//
// Usage:
//
//	GEMINI_API_KEY=... go run ./cmd/genimage \
//	  --prompt "..." --out hero.png \
//	  [--model gemini-3-pro-image] [--aspect 16:9] [--ref ref1.png --ref ref2.png]
//
// The pure request/response logic (buildRequestBody, extractImageBytes) is unit
// tested; run() is the thin I/O shell (flags, HTTP, files). It fails loudly so a
// caller never gets a silently-empty file. Stdlib only — no dependencies.
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const endpoint = "https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s"

// --- wire types --------------------------------------------------------------

type inlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

type part struct {
	Text       string      `json:"text,omitempty"`
	InlineData *inlineData `json:"inlineData,omitempty"`
}

type content struct {
	Parts []part `json:"parts"`
}

type imageConfig struct {
	AspectRatio string `json:"aspectRatio,omitempty"`
}

type generationConfig struct {
	ResponseModalities []string     `json:"responseModalities"`
	ImageConfig        *imageConfig `json:"imageConfig,omitempty"`
}

type request struct {
	Contents         []content        `json:"contents"`
	GenerationConfig generationConfig `json:"generationConfig"`
}

type response struct {
	Candidates []struct {
		Content content `json:"content"`
	} `json:"candidates"`
}

// refImage is a reference image to condition generation on.
type refImage struct {
	Mime string
	Data []byte
}

// --- pure core (unit tested) -------------------------------------------------

// buildRequestBody assembles the generateContent JSON: the prompt as text, any
// reference images base64-encoded as inline parts, and an aspect ratio when set.
func buildRequestBody(prompt, aspect string, refs []refImage) ([]byte, error) {
	parts := []part{{Text: prompt}}
	for _, r := range refs {
		parts = append(parts, part{InlineData: &inlineData{
			MimeType: r.Mime,
			Data:     base64.StdEncoding.EncodeToString(r.Data),
		}})
	}
	req := request{
		Contents:         []content{{Parts: parts}},
		GenerationConfig: generationConfig{ResponseModalities: []string{"IMAGE"}},
	}
	if aspect != "" {
		req.GenerationConfig.ImageConfig = &imageConfig{AspectRatio: aspect}
	}
	return json.Marshal(req)
}

// extractImageBytes decodes the first inline image in the response, or returns a
// descriptive error (no candidates, or the model returned only text).
func extractImageBytes(body []byte) ([]byte, error) {
	var parsed response
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decode response: %w (raw: %s)", err, truncate(string(body), 400))
	}
	if len(parsed.Candidates) == 0 {
		return nil, fmt.Errorf("no candidates returned (raw: %s)", truncate(string(body), 600))
	}
	var textOut []string
	for _, p := range parsed.Candidates[0].Content.Parts {
		if p.InlineData != nil && p.InlineData.Data != "" {
			data, err := base64.StdEncoding.DecodeString(p.InlineData.Data)
			if err != nil {
				return nil, fmt.Errorf("decode image data: %w", err)
			}
			return data, nil
		}
		if p.Text != "" {
			textOut = append(textOut, p.Text)
		}
	}
	return nil, fmt.Errorf("no image in response. Text: %s", truncate(strings.Join(textOut, " "), 600))
}

// --- I/O shell ---------------------------------------------------------------

type repeatable []string

func (r *repeatable) String() string { return strings.Join(*r, ",") }
func (r *repeatable) Set(v string) error {
	*r = append(*r, v)
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		prompt     = flag.String("prompt", "", "image prompt text")
		promptFile = flag.String("prompt-file", "", "read prompt from this file instead")
		out        = flag.String("out", "", "output PNG path (required)")
		model      = flag.String("model", "gemini-3-pro-image", "Gemini image model")
		aspect     = flag.String("aspect", "", "aspect ratio, e.g. 16:9, 1:1, 4:5")
		refs       repeatable
	)
	flag.Var(&refs, "ref", "reference image path (repeatable)")
	flag.Parse()

	if *out == "" {
		return fmt.Errorf("--out is required")
	}
	key := firstNonEmpty(os.Getenv("GEMINI_API_KEY"), os.Getenv("GOOGLE_API_KEY"))
	if key == "" {
		return fmt.Errorf("GEMINI_API_KEY / GOOGLE_API_KEY not set")
	}

	text := *prompt
	if *promptFile != "" {
		b, err := os.ReadFile(*promptFile)
		if err != nil {
			return fmt.Errorf("read prompt-file: %w", err)
		}
		text = string(b)
	}
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("no prompt provided (use --prompt or --prompt-file)")
	}

	refImages, err := loadRefs(refs)
	if err != nil {
		return err
	}
	body, err := buildRequestBody(text, *aspect, refImages)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	client := &http.Client{Timeout: 240 * time.Second}
	resp, err := client.Post(fmt.Sprintf(endpoint, *model, key), "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(respBody), 800))
	}

	img, err := extractImageBytes(respBody)
	if err != nil {
		return err
	}
	if err := os.WriteFile(*out, img, 0o644); err != nil {
		return fmt.Errorf("write %q: %w", *out, err)
	}
	fmt.Printf("OK %s (%d bytes) via %s\n", *out, len(img), *model)
	return nil
}

func loadRefs(paths []string) ([]refImage, error) {
	var refs []refImage
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("read ref %q: %w", p, err)
		}
		mime := "image/jpeg"
		if strings.HasSuffix(strings.ToLower(p), ".png") {
			mime = "image/png"
		}
		refs = append(refs, refImage{Mime: mime, Data: b})
	}
	return refs, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
