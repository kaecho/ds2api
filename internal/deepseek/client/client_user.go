package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"ds2api/internal/auth"
	dsprotocol "ds2api/internal/deepseek/protocol"
)

// CurrentUser is the chat.deepseek.com /api/v0/users/current payload we care about.
type CurrentUser struct {
	ID            string
	Email         string
	Status        int
	IsMuted       bool
	MuteUntilUnix int64
}

func (c *Client) GetCurrentUser(ctx context.Context, token string) (*CurrentUser, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("missing token")
	}
	clients := c.requestClientsFromContext(ctx)
	var authCtx *auth.RequestAuth
	if a, ok := auth.FromContext(ctx); ok {
		authCtx = a
	}
	headers := c.authHeaders(token, authCtx)
	resp, status, err := c.getJSONWithStatus(ctx, clients.regular, dsprotocol.DeepSeekCurrentUserURL, headers)
	if err != nil {
		return nil, err
	}
	code, bizCode, msg, bizMsg := extractResponseStatus(resp)
	if status != http.StatusOK || code != 0 || bizCode != 0 {
		combined := failureMessage(msg, bizMsg, fmt.Sprintf("status=%d code=%d biz_code=%d", status, code, bizCode))
		if isBannedError(combined) {
			return nil, &RequestFailure{Op: "current_user", Kind: FailureBanned, Message: combined}
		}
		return nil, fmt.Errorf("current user failed: %s", combined)
	}
	user := parseCurrentUser(resp)
	if user == nil {
		return nil, errors.New("current user: missing biz_data")
	}
	return user, nil
}

func parseCurrentUser(resp map[string]any) *CurrentUser {
	if resp == nil {
		return nil
	}
	data, _ := resp["data"].(map[string]any)
	bizData, _ := data["biz_data"].(map[string]any)
	if bizData == nil {
		return nil
	}
	user := &CurrentUser{
		ID:     stringFromMap(bizData, "id"),
		Email:  stringFromMap(bizData, "email"),
		Status: intFrom(bizData["status"]),
	}
	if chat, ok := bizData["chat"].(map[string]any); ok {
		user.IsMuted = truthyJSON(chat["is_muted"])
		user.MuteUntilUnix = unixJSON(chat["mute_until"])
	}
	return user
}

func truthyJSON(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case float64:
		return t != 0
	case int:
		return t != 0
	case int64:
		return t != 0
	case json.Number:
		f, err := t.Float64()
		return err == nil && f != 0
	case string:
		s := strings.TrimSpace(strings.ToLower(t))
		return s == "1" || s == "true"
	default:
		return false
	}
}

func unixJSON(v any) int64 {
	switch t := v.(type) {
	case float64:
		return int64(t)
	case int:
		return int64(t)
	case int64:
		return t
	case json.Number:
		f, err := t.Float64()
		if err != nil {
			return 0
		}
		return int64(f)
	default:
		return 0
	}
}
