package main

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildRequestBody_BasicShape(t *testing.T) {
	body, err := buildRequestBody("a prompt", "", nil)
	if err != nil {
		t.Fatalf("buildRequestBody: %v", err)
	}
	var got request
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got.Contents) != 1 || len(got.Contents[0].Parts) != 1 {
		t.Fatalf("contents/parts shape = %#v", got.Contents)
	}
	if got.Contents[0].Parts[0].Text != "a prompt" {
		t.Errorf("text = %q, want 'a prompt'", got.Contents[0].Parts[0].Text)
	}
	if len(got.GenerationConfig.ResponseModalities) != 1 || got.GenerationConfig.ResponseModalities[0] != "IMAGE" {
		t.Errorf("responseModalities = %v, want [IMAGE]", got.GenerationConfig.ResponseModalities)
	}
	if got.GenerationConfig.ImageConfig != nil {
		t.Errorf("imageConfig should be omitted when no aspect, got %#v", got.GenerationConfig.ImageConfig)
	}
}

func TestBuildRequestBody_WithAspect(t *testing.T) {
	body, _ := buildRequestBody("p", "16:9", nil)
	var got request
	_ = json.Unmarshal(body, &got)
	if got.GenerationConfig.ImageConfig == nil || got.GenerationConfig.ImageConfig.AspectRatio != "16:9" {
		t.Errorf("aspectRatio not set to 16:9: %#v", got.GenerationConfig.ImageConfig)
	}
}

func TestBuildRequestBody_WithRefEncodesBase64(t *testing.T) {
	raw := []byte("PNGBYTES")
	body, err := buildRequestBody("p", "", []refImage{{Mime: "image/png", Data: raw}})
	if err != nil {
		t.Fatalf("buildRequestBody: %v", err)
	}
	var got request
	_ = json.Unmarshal(body, &got)
	if len(got.Contents[0].Parts) != 2 {
		t.Fatalf("want 2 parts (text + ref), got %d", len(got.Contents[0].Parts))
	}
	ref := got.Contents[0].Parts[1].InlineData
	if ref == nil || ref.MimeType != "image/png" {
		t.Fatalf("ref inlineData missing/wrong mime: %#v", ref)
	}
	if ref.Data != base64.StdEncoding.EncodeToString(raw) {
		t.Errorf("ref data not base64-encoded correctly")
	}
}

func TestExtractImageBytes_Success(t *testing.T) {
	want := []byte("\x89PNG-the-image")
	resp := response{}
	resp.Candidates = make([]struct {
		Content content `json:"content"`
	}, 1)
	resp.Candidates[0].Content.Parts = []part{
		{InlineData: &inlineData{MimeType: "image/png", Data: base64.StdEncoding.EncodeToString(want)}},
	}
	body, _ := json.Marshal(resp)

	got, err := extractImageBytes(body)
	if err != nil {
		t.Fatalf("extractImageBytes: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("decoded bytes = %q, want %q", got, want)
	}
}

func TestExtractImageBytes_NoCandidatesFailsLoudly(t *testing.T) {
	if _, err := extractImageBytes([]byte(`{"candidates":[]}`)); err == nil {
		t.Fatal("extractImageBytes(no candidates) = nil error, want error")
	}
}

func TestExtractImageBytes_TextOnlyReturnsItInError(t *testing.T) {
	resp := response{}
	resp.Candidates = make([]struct {
		Content content `json:"content"`
	}, 1)
	resp.Candidates[0].Content.Parts = []part{{Text: "refused for safety"}}
	body, _ := json.Marshal(resp)

	_, err := extractImageBytes(body)
	if err == nil {
		t.Fatal("extractImageBytes(text only) = nil error, want error")
	}
	if !strings.Contains(err.Error(), "refused for safety") {
		t.Errorf("error %q should surface the model's text", err)
	}
}
