package claude

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"mime"
	"net/http"
	"strings"

	"ds2api/internal/auth"
	dsclient "ds2api/internal/deepseek/client"
)

const maxClaudeInlineImages = 50

type claudeImageUploadState struct {
	ctx          context.Context
	ds           DeepSeekCaller
	auth         *auth.RequestAuth
	uploadedByID map[string]string
	uploadCount  int
}

// preprocessClaudeImageInputs walks the Claude request messages, finds image
// blocks with base64 data (type: "image", source.type: "base64"), uploads them
// to the DeepSeek file API, and replaces them with input_image blocks containing
// file_id references. The collected file IDs are stored in req["ref_file_ids"].
func preprocessClaudeImageInputs(ctx context.Context, ds DeepSeekCaller, a *auth.RequestAuth, req map[string]any) error {
	if ds == nil || len(req) == 0 {
		return nil
	}
	messagesRaw, ok := req["messages"].([]any)
	if !ok || len(messagesRaw) == 0 {
		return nil
	}
	state := &claudeImageUploadState{
		ctx:          ctx,
		ds:           ds,
		auth:         a,
		uploadedByID: map[string]string{},
	}
	out := make([]any, 0, len(messagesRaw))
	var fileIDs []string
	seenIDs := map[string]struct{}{}
	for _, item := range messagesRaw {
		msg, ok := item.(map[string]any)
		if !ok {
			out = append(out, item)
			continue
		}
		newMsg, ids, err := state.walkMessage(msg)
		if err != nil {
			return err
		}
		out = append(out, newMsg)
		for _, id := range ids {
			if _, seen := seenIDs[id]; !seen {
				seenIDs[id] = struct{}{}
				fileIDs = append(fileIDs, id)
			}
		}
	}
	req["messages"] = out
	if len(fileIDs) > 0 {
		req["ref_file_ids"] = toAnySlice(fileIDs)
	}
	return nil
}

func (s *claudeImageUploadState) walkMessage(msg map[string]any) (map[string]any, []string, error) {
	content, ok := msg["content"].([]any)
	if !ok {
		return msg, nil, nil
	}
	newContent := make([]any, 0, len(content))
	var fileIDs []string
	for _, block := range content {
		b, ok := block.(map[string]any)
		if !ok {
			newContent = append(newContent, block)
			continue
		}
		if !isClaudeImageBlock(b) {
			newContent = append(newContent, b)
			continue
		}
		if s.uploadCount >= maxClaudeInlineImages {
			return nil, nil, fmt.Errorf("exceeded maximum of %d inline images per request", maxClaudeInlineImages)
		}
		data, mediaType, err := extractClaudeImageData(b)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid image input: %w", err)
		}
		fileID, err := s.uploadImage(data, mediaType)
		if err != nil {
			return nil, nil, err
		}
		s.uploadCount++
		fileIDs = append(fileIDs, fileID)
		newContent = append(newContent, map[string]any{
			"type":    "input_image",
			"file_id": fileID,
		})
	}
	newMsg := cloneMap(msg)
	newMsg["content"] = newContent
	return newMsg, fileIDs, nil
}

func isClaudeImageBlock(block map[string]any) bool {
	if block == nil {
		return false
	}
	blockType := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", block["type"])))
	if blockType != "image" {
		return false
	}
	source, ok := block["source"].(map[string]any)
	if !ok {
		return false
	}
	sourceType := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", source["type"])))
	return sourceType == "base64"
}

func extractClaudeImageData(block map[string]any) ([]byte, string, error) {
	source, _ := block["source"].(map[string]any)
	if source == nil {
		return nil, "", fmt.Errorf("missing source")
	}
	dataStr := strings.TrimSpace(fmt.Sprintf("%v", source["data"]))
	if dataStr == "" {
		return nil, "", fmt.Errorf("missing source.data")
	}
	decoded, err := decodeBase64Flexible(dataStr)
	if err != nil {
		return nil, "", fmt.Errorf("invalid base64 data: %w", err)
	}
	mediaType := strings.TrimSpace(fmt.Sprintf("%v", source["media_type"]))
	if mediaType == "" {
		mediaType = http.DetectContentType(decoded)
	}
	return decoded, mediaType, nil
}

func (s *claudeImageUploadState) uploadImage(data []byte, mediaType string) (string, error) {
	sum := sha256.Sum256(append([]byte(mediaType+"\x00"), data...))
	cacheKey := hex.EncodeToString(sum[:])
	if fileID, ok := s.uploadedByID[cacheKey]; ok && strings.TrimSpace(fileID) != "" {
		return fileID, nil
	}
	ext := ".bin"
	if parsedType := strings.TrimSpace(mediaType); parsedType != "" {
		if comma := strings.Index(parsedType, ";"); comma >= 0 {
			parsedType = strings.TrimSpace(parsedType[:comma])
		}
		if exts, err := mime.ExtensionsByType(parsedType); err == nil && len(exts) > 0 {
			ext = exts[0]
		}
	}
	filename := "image" + ext
	result, err := s.ds.UploadFile(s.ctx, s.auth, dsclient.UploadFileRequest{
		Filename:    filename,
		ContentType: mediaType,
		ModelType:   "vision",
		Data:        data,
	}, 3)
	if err != nil {
		return "", err
	}
	fileID := strings.TrimSpace(result.ID)
	if fileID == "" {
		return "", fmt.Errorf("upload succeeded without file id")
	}
	s.uploadedByID[cacheKey] = fileID
	return fileID, nil
}

func toAnySlice(items []string) []any {
	out := make([]any, 0, len(items))
	for _, item := range items {
		out = append(out, item)
	}
	return out
}

// decodeBase64Flexible tries standard, raw-std, URL, and raw-URL base64 decodings.
func decodeBase64Flexible(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	for _, enc := range []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	} {
		decoded, err := enc.DecodeString(raw)
		if err == nil {
			return decoded, nil
		}
	}
	return nil, fmt.Errorf("invalid base64 payload")
}
