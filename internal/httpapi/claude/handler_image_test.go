package claude

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"ds2api/internal/auth"
	dsclient "ds2api/internal/deepseek/client"
)

type claudeImageDSStub struct {
	uploadCalls []dsclient.UploadFileRequest
	uploadErr   error
}

func (s *claudeImageDSStub) CreateSession(_ context.Context, _ *auth.RequestAuth, _ int) (string, error) {
	return "session", nil
}

func (s *claudeImageDSStub) GetPow(_ context.Context, _ *auth.RequestAuth, _ int) (string, error) {
	return "pow", nil
}

func (s *claudeImageDSStub) UploadFile(_ context.Context, _ *auth.RequestAuth, req dsclient.UploadFileRequest, _ int) (*dsclient.UploadFileResult, error) {
	s.uploadCalls = append(s.uploadCalls, req)
	if s.uploadErr != nil {
		return nil, s.uploadErr
	}
	id := fmt.Sprintf("file-%d", len(s.uploadCalls))
	return &dsclient.UploadFileResult{
		ID:       id,
		Filename: req.Filename,
		Bytes:    int64(len(req.Data)),
		Status:   "uploaded",
	}, nil
}

func (s *claudeImageDSStub) CallCompletion(_ context.Context, _ *auth.RequestAuth, _ map[string]any, _ string, _ int) (*http.Response, error) {
	return nil, nil
}

func (s *claudeImageDSStub) DeleteSessionForToken(_ context.Context, _ string, _ string) (*dsclient.DeleteSessionResult, error) {
	return &dsclient.DeleteSessionResult{Success: true}, nil
}

func (s *claudeImageDSStub) DeleteAllSessionsForToken(_ context.Context, _ string) error {
	return nil
}

func TestPreprocessClaudeImageInputsReplacesBase64Image(t *testing.T) {
	ds := &claudeImageDSStub{}
	b64 := base64.StdEncoding.EncodeToString([]byte("fake-image-data"))
	req := map[string]any{
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "text", "text": "describe this"},
					map[string]any{
						"type": "image",
						"source": map[string]any{
							"type":       "base64",
							"media_type": "image/jpeg",
							"data":       b64,
						},
					},
				},
			},
		},
	}

	err := preprocessClaudeImageInputs(context.Background(), ds, &auth.RequestAuth{DeepSeekToken: "token"}, req)
	if err != nil {
		t.Fatalf("preprocess failed: %v", err)
	}

	if len(ds.uploadCalls) != 1 {
		t.Fatalf("expected 1 upload, got %d", len(ds.uploadCalls))
	}
	if ds.uploadCalls[0].ContentType != "image/jpeg" {
		t.Fatalf("expected image/jpeg, got %q", ds.uploadCalls[0].ContentType)
	}
	if ds.uploadCalls[0].ModelType != "vision" {
		t.Fatalf("expected vision model type, got %q", ds.uploadCalls[0].ModelType)
	}

	messages, _ := req["messages"].([]any)
	msg, _ := messages[0].(map[string]any)
	content, _ := msg["content"].([]any)
	if len(content) != 2 {
		t.Fatalf("expected 2 content blocks, got %d", len(content))
	}

	imgBlock, _ := content[1].(map[string]any)
	if imgBlock["type"] != "input_image" {
		t.Fatalf("expected input_image type, got %v", imgBlock["type"])
	}
	if imgBlock["file_id"] != "file-1" {
		t.Fatalf("expected file-1, got %v", imgBlock["file_id"])
	}

	refIDs, ok := req["ref_file_ids"].([]any)
	if !ok || len(refIDs) != 1 || refIDs[0] != "file-1" {
		t.Fatalf("expected ref_file_ids=[file-1], got %#v", req["ref_file_ids"])
	}
}

func TestPreprocessClaudeImageInputsDeduplicatesSameImage(t *testing.T) {
	ds := &claudeImageDSStub{}
	b64 := base64.StdEncoding.EncodeToString([]byte("same-image"))
	req := map[string]any{
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{
						"type": "image",
						"source": map[string]any{"type": "base64", "media_type": "image/png", "data": b64},
					},
					map[string]any{
						"type": "image",
						"source": map[string]any{"type": "base64", "media_type": "image/png", "data": b64},
					},
				},
			},
		},
	}

	err := preprocessClaudeImageInputs(context.Background(), ds, &auth.RequestAuth{}, req)
	if err != nil {
		t.Fatalf("preprocess failed: %v", err)
	}
	if len(ds.uploadCalls) != 1 {
		t.Fatalf("expected deduplicated single upload, got %d", len(ds.uploadCalls))
	}
	refIDs, _ := req["ref_file_ids"].([]any)
	if len(refIDs) != 1 {
		t.Fatalf("expected 1 ref_file_id, got %d", len(refIDs))
	}
}

func TestPreprocessClaudeImageInputsSkipsTextBlocks(t *testing.T) {
	ds := &claudeImageDSStub{}
	req := map[string]any{
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "text", "text": "hello"},
				},
			},
		},
	}

	err := preprocessClaudeImageInputs(context.Background(), ds, &auth.RequestAuth{}, req)
	if err != nil {
		t.Fatalf("preprocess failed: %v", err)
	}
	if len(ds.uploadCalls) != 0 {
		t.Fatalf("expected 0 uploads, got %d", len(ds.uploadCalls))
	}
	if _, ok := req["ref_file_ids"]; ok {
		t.Fatal("expected no ref_file_ids when no images")
	}
}

func TestPreprocessClaudeImageInputsSkipsNonBase64ImageSource(t *testing.T) {
	ds := &claudeImageDSStub{}
	req := map[string]any{
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{
						"type": "image",
						"source": map[string]any{
							"type": "url",
							"url":  "https://example.com/cat.jpg",
						},
					},
				},
			},
		},
	}

	err := preprocessClaudeImageInputs(context.Background(), ds, &auth.RequestAuth{}, req)
	if err != nil {
		t.Fatalf("preprocess failed: %v", err)
	}
	if len(ds.uploadCalls) != 0 {
		t.Fatalf("expected 0 uploads for URL source, got %d", len(ds.uploadCalls))
	}
}

func TestPreprocessClaudeImageInputsUploadError(t *testing.T) {
	ds := &claudeImageDSStub{uploadErr: fmt.Errorf("upload failed")}
	b64 := base64.StdEncoding.EncodeToString([]byte("image-data"))
	req := map[string]any{
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{
						"type": "image",
						"source": map[string]any{"type": "base64", "media_type": "image/png", "data": b64},
					},
				},
			},
		},
	}

	err := preprocessClaudeImageInputs(context.Background(), ds, &auth.RequestAuth{}, req)
	if err == nil {
		t.Fatal("expected error from upload failure")
	}
}

func TestPreprocessClaudeImageInputsNoMessages(t *testing.T) {
	ds := &claudeImageDSStub{}
	req := map[string]any{"model": "deepseek-v4-vision"}

	err := preprocessClaudeImageInputs(context.Background(), ds, &auth.RequestAuth{}, req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(ds.uploadCalls) != 0 {
		t.Fatalf("expected 0 uploads, got %d", len(ds.uploadCalls))
	}
}

func TestPreprocessClaudeImageInputsNilDS(t *testing.T) {
	b64 := base64.StdEncoding.EncodeToString([]byte("image-data"))
	req := map[string]any{
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{
						"type": "image",
						"source": map[string]any{"type": "base64", "media_type": "image/png", "data": b64},
					},
				},
			},
		},
	}

	err := preprocessClaudeImageInputs(context.Background(), nil, &auth.RequestAuth{}, req)
	if err != nil {
		t.Fatalf("expected no error with nil DS, got %v", err)
	}
}

func TestIsClaudeImageBlock(t *testing.T) {
	tests := []struct {
		name  string
		block map[string]any
		want  bool
	}{
		{"nil", nil, false},
		{"text block", map[string]any{"type": "text", "text": "hi"}, false},
		{"image with base64 source", map[string]any{
			"type":   "image",
			"source": map[string]any{"type": "base64", "data": "abc"},
		}, true},
		{"image with url source", map[string]any{
			"type":   "image",
			"source": map[string]any{"type": "url", "url": "https://example.com/img.png"},
		}, false},
		{"image without source", map[string]any{"type": "image"}, false},
		{"input_image block", map[string]any{
			"type": "input_image", "file_id": "file-1",
		}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isClaudeImageBlock(tt.block)
			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractClaudeImageData(t *testing.T) {
	b64 := base64.StdEncoding.EncodeToString([]byte("test-image"))
	block := map[string]any{
		"type": "image",
		"source": map[string]any{
			"type":       "base64",
			"media_type": "image/png",
			"data":       b64,
		},
	}
	data, mediaType, err := extractClaudeImageData(block)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "test-image" {
		t.Fatalf("expected 'test-image', got %q", string(data))
	}
	if mediaType != "image/png" {
		t.Fatalf("expected image/png, got %q", mediaType)
	}
}

func TestExtractClaudeImageDataAutoDetectMediaType(t *testing.T) {
	// Minimal PNG header
	pngData := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	b64 := base64.StdEncoding.EncodeToString(pngData)
	block := map[string]any{
		"type": "image",
		"source": map[string]any{
			"type": "base64",
			"data": b64,
		},
	}
	_, mediaType, err := extractClaudeImageData(block)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mediaType == "" {
		t.Fatal("expected auto-detected media type")
	}
}

func TestCollectRefFileIDsFromRequest(t *testing.T) {
	req := map[string]any{
		"ref_file_ids": []any{"file-1", "file-2"},
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "input_image", "file_id": "file-3"},
				},
			},
		},
	}
	ids := collectRefFileIDsFromRequest(req)
	if len(ids) != 3 {
		t.Fatalf("expected 3 file IDs, got %d: %v", len(ids), ids)
	}
}

func TestCollectRefFileIDsFromRequestDeduplicates(t *testing.T) {
	req := map[string]any{
		"ref_file_ids": []any{"file-1"},
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "input_image", "file_id": "file-1"},
				},
			},
		},
	}
	ids := collectRefFileIDsFromRequest(req)
	if len(ids) != 1 {
		t.Fatalf("expected 1 deduplicated file ID, got %d: %v", len(ids), ids)
	}
}

func TestCollectRefFileIDsFromRequestEmpty(t *testing.T) {
	req := map[string]any{
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "text", "text": "hello"},
				},
			},
		},
	}
	ids := collectRefFileIDsFromRequest(req)
	if len(ids) != 0 {
		t.Fatalf("expected 0 file IDs, got %d", len(ids))
	}
}

func TestPreprocessClaudeImageInputsMultipleMessages(t *testing.T) {
	ds := &claudeImageDSStub{}
	b64_1 := base64.StdEncoding.EncodeToString([]byte("image-1"))
	b64_2 := base64.StdEncoding.EncodeToString([]byte("image-2"))
	req := map[string]any{
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "text", "text": "first image:"},
					map[string]any{
						"type":   "image",
						"source": map[string]any{"type": "base64", "media_type": "image/jpeg", "data": b64_1},
					},
				},
			},
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "text", "text": "second image:"},
					map[string]any{
						"type":   "image",
						"source": map[string]any{"type": "base64", "media_type": "image/png", "data": b64_2},
					},
				},
			},
		},
	}

	err := preprocessClaudeImageInputs(context.Background(), ds, &auth.RequestAuth{}, req)
	if err != nil {
		t.Fatalf("preprocess failed: %v", err)
	}
	if len(ds.uploadCalls) != 2 {
		t.Fatalf("expected 2 uploads, got %d", len(ds.uploadCalls))
	}
	if ds.uploadCalls[0].ContentType != "image/jpeg" {
		t.Fatalf("expected image/jpeg for first upload, got %q", ds.uploadCalls[0].ContentType)
	}
	if ds.uploadCalls[1].ContentType != "image/png" {
		t.Fatalf("expected image/png for second upload, got %q", ds.uploadCalls[1].ContentType)
	}
	refIDs, _ := req["ref_file_ids"].([]any)
	if len(refIDs) != 2 {
		t.Fatalf("expected 2 ref_file_ids, got %d", len(refIDs))
	}
}

func TestNormalizeClaudeMessagesHandlesInputImageBlock(t *testing.T) {
	// After preprocessing, input_image blocks should pass through normalization
	// and end up as sanitized text in the prompt (with file_id preserved).
	msgs := []any{
		map[string]any{
			"role": "user",
			"content": []any{
				map[string]any{"type": "text", "text": "describe this"},
				map[string]any{"type": "input_image", "file_id": "file-123"},
			},
		},
	}
	got := normalizeClaudeMessages(msgs)
	if len(got) != 1 {
		t.Fatalf("expected 1 message, got %d", len(got))
	}
	m := got[0].(map[string]any)
	content, _ := m["content"].(string)
	if !strings.Contains(content, "describe this") {
		t.Fatalf("expected text preserved, got %q", content)
	}
	if !strings.Contains(content, "input_image") {
		t.Fatalf("expected input_image in normalized content, got %q", content)
	}
	if !strings.Contains(content, "file-123") {
		t.Fatalf("expected file_id preserved in normalized content, got %q", content)
	}
}
