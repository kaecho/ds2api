package smid

import (
	"context"
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

//go:embed js/sm_device_id.js js/sm_des.js js/sm_conf.json
var assets embed.FS

var (
	extractOnce sync.Once
	scriptDir   string
	extractErr  error
)

// IsValid reports a 数美 protocol device_id as used by DeepSeek login (B... from deviceprofile/v4).
func IsValid(id string) bool {
	id = strings.TrimSpace(id)
	return strings.HasPrefix(id, "B") && len(id) >= 40
}

// FetchProtocolDeviceID asks 数美 deviceprofile/v4 for a web-protocol SMID.
// DeepSeek Android login currently accepts this B-prefixed value as device_id.
func FetchProtocolDeviceID(ctx context.Context) (string, error) {
	dir, err := extractedDir()
	if err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, "node", filepath.Join(dir, "sm_device_id.js"))
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if ee, ok := err.(*exec.ExitError); ok {
			msg = strings.TrimSpace(string(ee.Stderr))
			if msg == "" {
				msg = strings.TrimSpace(string(out))
			}
		}
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("node sm_device_id: %s", msg)
	}
	id := strings.TrimSpace(string(out))
	if !IsValid(id) {
		return "", fmt.Errorf("node sm_device_id returned invalid id")
	}
	return id, nil
}

func extractedDir() (string, error) {
	extractOnce.Do(func() {
		dir, err := os.MkdirTemp("", "ds2api-smid-")
		if err != nil {
			extractErr = err
			return
		}
		names := []string{"sm_device_id.js", "sm_des.js", "sm_conf.json"}
		for _, name := range names {
			data, readErr := assets.ReadFile("js/" + name)
			if readErr != nil {
				extractErr = readErr
				return
			}
			if writeErr := os.WriteFile(filepath.Join(dir, name), data, 0o644); writeErr != nil {
				extractErr = writeErr
				return
			}
		}
		scriptDir = dir
	})
	if extractErr != nil {
		return "", extractErr
	}
	return scriptDir, nil
}
