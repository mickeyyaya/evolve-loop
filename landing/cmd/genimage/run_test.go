package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// imageResponseJSON builds a valid generateContent response carrying one inline
// image whose raw bytes are img.
func imageResponseJSON(t *testing.T, mime string, img []byte) []byte {
	t.Helper()
	resp := response{}
	resp.Candidates = make([]struct {
		Content content `json:"content"`
	}, 1)
	resp.Candidates[0].Content.Parts = []part{
		{InlineData: &inlineData{MimeType: mime, Data: base64.StdEncoding.EncodeToString(img)}},
	}
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal image response: %v", err)
	}
	return b
}

// pointEndpointAt overrides apiEndpoint to target srvURL and restores it after
// the test. The %s/%s placeholders for model and key are preserved so the
// production formatting path is exercised.
func pointEndpointAt(t *testing.T, srvURL string) {
	t.Helper()
	prev := apiEndpoint
	apiEndpoint = srvURL + "/v1beta/models/%s:generateContent?key=%s"
	t.Cleanup(func() { apiEndpoint = prev })
}

// clearKeyEnv unsets both API-key env vars for the duration of the test.
func clearKeyEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("GOOGLE_API_KEY", "")
}

func TestRun_Success(t *testing.T) {
	clearKeyEnv(t)
	t.Setenv("GEMINI_API_KEY", "test-key")

	wantImg := []byte("\x89PNG\r\n\x1a\nthe-real-image-bytes")
	var gotPath, gotMethod, gotContentType string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(imageResponseJSON(t, "image/png", wantImg))
	}))
	defer srv.Close()
	pointEndpointAt(t, srv.URL)

	outPath := filepath.Join(t.TempDir(), "hero.png")
	var stdout, stderr bytes.Buffer
	code := run([]string{"--prompt", "a banana", "--out", outPath, "--model", "gemini-3-pro-image", "--aspect", "16:9"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("run() = %d, want 0; stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want empty on success", stderr.String())
	}
	if !strings.Contains(stdout.String(), "OK "+outPath) {
		t.Errorf("stdout = %q, want OK line referencing %q", stdout.String(), outPath)
	}
	if !strings.Contains(stdout.String(), "gemini-3-pro-image") {
		t.Errorf("stdout = %q, want it to report the model", stdout.String())
	}

	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	if !bytes.Equal(got, wantImg) {
		t.Errorf("written file bytes = %q, want %q", got, wantImg)
	}

	// Verify the request the server actually received.
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotContentType != "application/json" {
		t.Errorf("content-type = %q, want application/json", gotContentType)
	}
	if !strings.Contains(gotPath, "gemini-3-pro-image:generateContent") {
		t.Errorf("path = %q, want it to embed the model", gotPath)
	}
	var sentReq request
	if err := json.Unmarshal(gotBody, &sentReq); err != nil {
		t.Fatalf("unmarshal sent body: %v (raw %s)", err, gotBody)
	}
	if len(sentReq.Contents) != 1 || len(sentReq.Contents[0].Parts) != 1 || sentReq.Contents[0].Parts[0].Text != "a banana" {
		t.Errorf("sent request prompt not propagated: %#v", sentReq.Contents)
	}
	if sentReq.GenerationConfig.ImageConfig == nil || sentReq.GenerationConfig.ImageConfig.AspectRatio != "16:9" {
		t.Errorf("aspect not propagated to request: %#v", sentReq.GenerationConfig.ImageConfig)
	}
}

func TestRun_SuccessWithPromptFileAndRef(t *testing.T) {
	clearKeyEnv(t)
	// Exercise the GOOGLE_API_KEY fallback path (GEMINI unset, GOOGLE set).
	t.Setenv("GOOGLE_API_KEY", "google-key")

	dir := t.TempDir()
	promptPath := filepath.Join(dir, "prompt.txt")
	if err := os.WriteFile(promptPath, []byte("  prompt from file  "), 0o644); err != nil {
		t.Fatalf("write prompt file: %v", err)
	}
	refPath := filepath.Join(dir, "ref.png")
	refBytes := []byte("REFPNGDATA")
	if err := os.WriteFile(refPath, refBytes, 0o644); err != nil {
		t.Fatalf("write ref file: %v", err)
	}

	wantImg := []byte("imgbytes")
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		_, _ = w.Write(imageResponseJSON(t, "image/png", wantImg))
	}))
	defer srv.Close()
	pointEndpointAt(t, srv.URL)

	outPath := filepath.Join(dir, "out.png")
	var stdout, stderr bytes.Buffer
	code := run([]string{"--prompt-file", promptPath, "--ref", refPath, "--out", outPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() = %d, want 0; stderr=%q", code, stderr.String())
	}

	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !bytes.Equal(got, wantImg) {
		t.Errorf("output bytes = %q, want %q", got, wantImg)
	}

	var sentReq request
	if err := json.Unmarshal(gotBody, &sentReq); err != nil {
		t.Fatalf("unmarshal sent body: %v", err)
	}
	if got := sentReq.Contents[0].Parts[0].Text; got != "prompt from file" {
		// run() does not trim the body it sends; it only trims for the
		// emptiness check. So the file content is sent verbatim.
		if strings.TrimSpace(got) != "prompt from file" {
			t.Errorf("prompt-file content not propagated, got %q", got)
		}
	}
	if len(sentReq.Contents[0].Parts) != 2 {
		t.Fatalf("want text + ref parts, got %d", len(sentReq.Contents[0].Parts))
	}
	ref := sentReq.Contents[0].Parts[1].InlineData
	if ref == nil || ref.MimeType != "image/png" {
		t.Fatalf("ref inlineData missing/wrong mime: %#v", ref)
	}
	if ref.Data != base64.StdEncoding.EncodeToString(refBytes) {
		t.Errorf("ref data not base64 of file bytes")
	}
}

func TestRun_MissingOut(t *testing.T) {
	clearKeyEnv(t)
	t.Setenv("GEMINI_API_KEY", "k")
	var stdout, stderr bytes.Buffer
	code := run([]string{"--prompt", "x"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "--out is required") {
		t.Errorf("stderr = %q, want it to mention --out", stderr.String())
	}
}

func TestRun_MissingKey(t *testing.T) {
	clearKeyEnv(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{"--prompt", "x", "--out", filepath.Join(t.TempDir(), "o.png")}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "GEMINI_API_KEY") {
		t.Errorf("stderr = %q, want it to mention the key", stderr.String())
	}
}

func TestRun_EmptyPrompt(t *testing.T) {
	clearKeyEnv(t)
	t.Setenv("GEMINI_API_KEY", "k")
	var stdout, stderr bytes.Buffer
	// Whitespace-only prompt must be rejected as empty.
	code := run([]string{"--prompt", "   ", "--out", filepath.Join(t.TempDir(), "o.png")}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "no prompt provided") {
		t.Errorf("stderr = %q, want 'no prompt provided'", stderr.String())
	}
}

func TestRun_NonexistentPromptFile(t *testing.T) {
	clearKeyEnv(t)
	t.Setenv("GEMINI_API_KEY", "k")
	var stdout, stderr bytes.Buffer
	missing := filepath.Join(t.TempDir(), "nope.txt")
	code := run([]string{"--prompt-file", missing, "--out", filepath.Join(t.TempDir(), "o.png")}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "read prompt-file") {
		t.Errorf("stderr = %q, want 'read prompt-file'", stderr.String())
	}
}

func TestRun_NonexistentRef(t *testing.T) {
	clearKeyEnv(t)
	t.Setenv("GEMINI_API_KEY", "k")
	var stdout, stderr bytes.Buffer
	missingRef := filepath.Join(t.TempDir(), "no-ref.png")
	code := run([]string{"--prompt", "p", "--ref", missingRef, "--out", filepath.Join(t.TempDir(), "o.png")}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "read ref") {
		t.Errorf("stderr = %q, want 'read ref'", stderr.String())
	}
}

func TestRun_HTTP500(t *testing.T) {
	clearKeyEnv(t)
	t.Setenv("GEMINI_API_KEY", "k")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("upstream exploded"))
	}))
	defer srv.Close()
	pointEndpointAt(t, srv.URL)

	outPath := filepath.Join(t.TempDir(), "o.png")
	var stdout, stderr bytes.Buffer
	code := run([]string{"--prompt", "p", "--out", outPath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "HTTP 500") {
		t.Errorf("stderr = %q, want 'HTTP 500'", stderr.String())
	}
	if _, err := os.Stat(outPath); !os.IsNotExist(err) {
		t.Errorf("output file should not exist after HTTP error, stat err=%v", err)
	}
}

func TestRun_TextOnlyResponse(t *testing.T) {
	clearKeyEnv(t)
	t.Setenv("GEMINI_API_KEY", "k")
	resp := response{}
	resp.Candidates = make([]struct {
		Content content `json:"content"`
	}, 1)
	resp.Candidates[0].Content.Parts = []part{{Text: "refused for safety reasons"}}
	body, _ := json.Marshal(resp)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	defer srv.Close()
	pointEndpointAt(t, srv.URL)

	outPath := filepath.Join(t.TempDir(), "o.png")
	var stdout, stderr bytes.Buffer
	code := run([]string{"--prompt", "p", "--out", outPath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "no image in response") {
		t.Errorf("stderr = %q, want 'no image in response'", stderr.String())
	}
	if !strings.Contains(stderr.String(), "refused for safety reasons") {
		t.Errorf("stderr = %q, want it to surface the model's text", stderr.String())
	}
	if _, err := os.Stat(outPath); !os.IsNotExist(err) {
		t.Errorf("output file should not exist after text-only response")
	}
}

func TestRun_RequestError_BadEndpoint(t *testing.T) {
	clearKeyEnv(t)
	t.Setenv("GEMINI_API_KEY", "k")
	// Point at a server that is immediately closed so the POST fails to connect.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()
	prev := apiEndpoint
	apiEndpoint = url + "/v1beta/models/%s:generateContent?key=%s"
	t.Cleanup(func() { apiEndpoint = prev })

	var stdout, stderr bytes.Buffer
	code := run([]string{"--prompt", "p", "--out", filepath.Join(t.TempDir(), "o.png")}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "request:") {
		t.Errorf("stderr = %q, want a 'request:' transport error", stderr.String())
	}
}

func TestRun_UnwritableOut(t *testing.T) {
	clearKeyEnv(t)
	t.Setenv("GEMINI_API_KEY", "k")
	wantImg := []byte("img")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(imageResponseJSON(t, "image/png", wantImg))
	}))
	defer srv.Close()
	pointEndpointAt(t, srv.URL)

	// out path lives under a non-existent directory => os.WriteFile fails.
	outPath := filepath.Join(t.TempDir(), "no-such-dir", "o.png")
	var stdout, stderr bytes.Buffer
	code := run([]string{"--prompt", "p", "--out", outPath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "write ") {
		t.Errorf("stderr = %q, want a 'write' error", stderr.String())
	}
}

func TestRun_BadFlag(t *testing.T) {
	clearKeyEnv(t)
	t.Setenv("GEMINI_API_KEY", "k")
	var stdout, stderr bytes.Buffer
	code := run([]string{"--nonexistent-flag"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "parse flags") {
		t.Errorf("stderr = %q, want 'parse flags'", stderr.String())
	}
}

func TestRun_TruncatedResponseBody(t *testing.T) {
	clearKeyEnv(t)
	t.Setenv("GEMINI_API_KEY", "k")
	// Advertise a large body via Content-Length, then write fewer bytes and
	// hijack/close the connection so io.ReadAll fails with unexpected EOF.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "4096")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("partial"))
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Fatalf("response writer does not support hijacking")
		}
		conn, _, err := hj.Hijack()
		if err != nil {
			t.Fatalf("hijack: %v", err)
		}
		_ = conn.Close() // drop the connection mid-body
	}))
	defer srv.Close()
	pointEndpointAt(t, srv.URL)

	var stdout, stderr bytes.Buffer
	code := run([]string{"--prompt", "p", "--out", filepath.Join(t.TempDir(), "o.png")}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() = %d, want 1; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "read response") {
		t.Errorf("stderr = %q, want 'read response'", stderr.String())
	}
}

// forceMarshalError overrides jsonMarshal to fail, then restores it. This makes
// the otherwise-unreachable "marshal failed" defensive branch reachable. Runtime
// behavior is unaffected — jsonMarshal defaults to json.Marshal in main().
func forceMarshalError(t *testing.T) {
	t.Helper()
	prev := jsonMarshal
	jsonMarshal = func(v any) ([]byte, error) {
		return nil, errForcedMarshal
	}
	t.Cleanup(func() { jsonMarshal = prev })
}

var errForcedMarshal = errForced("forced marshal failure")

type errForced string

func (e errForced) Error() string { return string(e) }

func TestBuildRequestBody_MarshalError(t *testing.T) {
	forceMarshalError(t)
	_, err := buildRequestBody("p", "", nil)
	if err == nil {
		t.Fatal("buildRequestBody with failing marshal = nil error, want error")
	}
	if !strings.Contains(err.Error(), "forced marshal failure") {
		t.Errorf("error = %q, want the forced marshal error surfaced", err)
	}
}

func TestRun_BuildRequestError(t *testing.T) {
	clearKeyEnv(t)
	t.Setenv("GEMINI_API_KEY", "k")
	forceMarshalError(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{"--prompt", "p", "--out", filepath.Join(t.TempDir(), "o.png")}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "build request") {
		t.Errorf("stderr = %q, want 'build request'", stderr.String())
	}
}

// --- unit tests for helpers --------------------------------------------------

func TestExtractImageBytes_InvalidJSON(t *testing.T) {
	_, err := extractImageBytes([]byte("this is not json{"))
	if err == nil {
		t.Fatal("extractImageBytes(invalid json) = nil error, want error")
	}
	if !strings.Contains(err.Error(), "decode response") {
		t.Errorf("error = %q, want 'decode response'", err)
	}
}

func TestExtractImageBytes_BadBase64(t *testing.T) {
	resp := response{}
	resp.Candidates = make([]struct {
		Content content `json:"content"`
	}, 1)
	// '!!!' is not valid base64, so base64.StdEncoding.DecodeString fails.
	resp.Candidates[0].Content.Parts = []part{
		{InlineData: &inlineData{MimeType: "image/png", Data: "!!!not-base64!!!"}},
	}
	body, _ := json.Marshal(resp)
	_, err := extractImageBytes(body)
	if err == nil {
		t.Fatal("extractImageBytes(bad base64) = nil error, want error")
	}
	if !strings.Contains(err.Error(), "decode image data") {
		t.Errorf("error = %q, want 'decode image data'", err)
	}
}

func TestLoadRefs_Success(t *testing.T) {
	dir := t.TempDir()
	pngPath := filepath.Join(dir, "a.PNG") // uppercase extension exercises ToLower
	jpgPath := filepath.Join(dir, "b.jpg")
	if err := os.WriteFile(pngPath, []byte("PNGDATA"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jpgPath, []byte("JPGDATA"), 0o644); err != nil {
		t.Fatal(err)
	}

	refs, err := loadRefs([]string{pngPath, jpgPath})
	if err != nil {
		t.Fatalf("loadRefs: %v", err)
	}
	if len(refs) != 2 {
		t.Fatalf("want 2 refs, got %d", len(refs))
	}
	if refs[0].Mime != "image/png" || string(refs[0].Data) != "PNGDATA" {
		t.Errorf("png ref wrong: %#v", refs[0])
	}
	if refs[1].Mime != "image/jpeg" || string(refs[1].Data) != "JPGDATA" {
		t.Errorf("jpg ref wrong: %#v", refs[1])
	}
}

func TestLoadRefs_Empty(t *testing.T) {
	refs, err := loadRefs(nil)
	if err != nil {
		t.Fatalf("loadRefs(nil): %v", err)
	}
	if refs != nil {
		t.Errorf("loadRefs(nil) = %#v, want nil", refs)
	}
}

func TestLoadRefs_MissingFile(t *testing.T) {
	_, err := loadRefs([]string{filepath.Join(t.TempDir(), "missing.png")})
	if err == nil {
		t.Fatal("loadRefs(missing) = nil error, want error")
	}
	if !strings.Contains(err.Error(), "read ref") {
		t.Errorf("error = %q, want 'read ref'", err)
	}
}

func TestFirstNonEmpty(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want string
	}{
		{"first wins", []string{"a", "b"}, "a"},
		{"skips empty", []string{"", "b"}, "b"},
		{"all empty", []string{"", ""}, ""},
		{"none", nil, ""},
		{"third", []string{"", "", "c"}, "c"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := firstNonEmpty(tc.in...); got != tc.want {
				t.Errorf("firstNonEmpty(%v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("short", 10); got != "short" {
		t.Errorf("truncate(short) = %q, want unchanged", got)
	}
	if got := truncate("exactly10!", 10); got != "exactly10!" {
		t.Errorf("truncate at boundary = %q, want unchanged", got)
	}
	got := truncate("abcdefghij", 3)
	if got != "abc…" {
		t.Errorf("truncate(abcdefghij,3) = %q, want 'abc…'", got)
	}
}

func TestRepeatable_SetAndString(t *testing.T) {
	var r repeatable
	if r.String() != "" {
		t.Errorf("empty String() = %q, want empty", r.String())
	}
	if err := r.Set("one"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := r.Set("two"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if len(r) != 2 || r[0] != "one" || r[1] != "two" {
		t.Errorf("after two Sets = %#v, want [one two]", r)
	}
	if r.String() != "one,two" {
		t.Errorf("String() = %q, want 'one,two'", r.String())
	}
}
