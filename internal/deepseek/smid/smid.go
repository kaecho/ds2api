package smid

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultProfileHost = "fp-it-acc.portal101.cn"
	pubPEM             = `-----BEGIN PUBLIC KEY-----
MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDetfEgYD4aE1ZjmWJ6/jnPurhzI+ye
RoJHWrnNtQMte3stQ4VjG3yu21FuN75E6cDpA9KtDXwcB2M/FiGUAe3G0rNotbWI8+Sj
ZfUbW/OILFTzY0uaeEkmVGW5WyJ6weQbbr1xTCPa2OO3YIMeZljWUYHG5h21WAm/PATg
8im8cQIDAQAB
-----END PUBLIC KEY-----`
)

var (
	profileURL = "https://" + defaultProfileHost + "/deviceprofile/v4"
	httpClient = &http.Client{Timeout: 20 * time.Second}

	pubOnce sync.Once
	pubKey  *rsa.PublicKey
	pubErr  error
)

func init() {
	if h := strings.TrimSpace(os.Getenv("SM_API_HOST")); h != "" {
		profileURL = joinProfileURL(h)
	}
}

func joinProfileURL(host string) string {
	host = strings.TrimRight(strings.TrimSpace(host), "/")
	if strings.Contains(host, "://") {
		if strings.HasSuffix(host, "/deviceprofile/v4") {
			return host
		}
		return host + "/deviceprofile/v4"
	}
	return "https://" + host + "/deviceprofile/v4"
}

// IsValid reports a 数美 protocol device_id as used by DeepSeek login (B... from deviceprofile/v4).
func IsValid(id string) bool {
	id = strings.TrimSpace(id)
	return strings.HasPrefix(id, "B") && len(id) >= 40
}

// FetchProtocolDeviceID asks 数美 deviceprofile/v4 for a web-protocol SMID.
// DeepSeek Android login currently accepts this B-prefixed value as device_id.
func FetchProtocolDeviceID(ctx context.Context) (string, error) {
	pub, err := rsaPublicKey()
	if err != nil {
		return "", err
	}
	box, uid, err := buildBox(time.Now(), smPrevBox())
	if err != nil {
		return "", err
	}
	priID := md5Hex(uid)
	if len(priID) < 16 {
		return "", fmt.Errorf("priId too short")
	}
	priID = priID[:16]
	ep, err := rsaEncryptUID(pub, uid)
	if err != nil {
		return "", fmt.Errorf("rsa ep: %w", err)
	}
	confused, err := confuse(box)
	if err != nil {
		return "", err
	}
	raw, err := marshalObject(confused)
	if err != nil {
		return "", err
	}
	dataHex, err := aesEncryptHex(string(raw), priID)
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(map[string]any{
		"appId":        smApp,
		"organization": smOrg,
		"ep":           ep,
		"data":         dataHex,
		"os":           "web",
		"encode":       5,
		"compress":     2,
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, profileURL, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json;charset=UTF-8")
	req.Header.Set("Origin", "https://platform.deepseek.com")
	req.Header.Set("Referer", "https://platform.deepseek.com/")
	req.Header.Set("User-Agent", smUserAgent())

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("deviceprofile: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("deviceprofile read: %w", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("deviceprofile status=%d body=%q", resp.StatusCode, trimBody(body))
	}
	if jsonNumber(parsed["code"]) != 1100 {
		return "", fmt.Errorf("deviceprofile: %s", trimBody(body))
	}
	detail, _ := parsed["detail"].(map[string]any)
	did, _ := detail["deviceId"].(string)
	did = strings.TrimSpace(did)
	if did == "" {
		return "", fmt.Errorf("deviceprofile: missing deviceId")
	}
	id := "B" + did
	if !IsValid(id) {
		return "", fmt.Errorf("deviceprofile returned invalid id")
	}
	return id, nil
}

func rsaPublicKey() (*rsa.PublicKey, error) {
	pubOnce.Do(func() {
		block, _ := pem.Decode([]byte(pubPEM))
		if block == nil {
			pubErr = fmt.Errorf("rsa pem")
			return
		}
		key, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			pubErr = err
			return
		}
		rsaKey, ok := key.(*rsa.PublicKey)
		if !ok {
			pubErr = fmt.Errorf("rsa public key type")
			return
		}
		pubKey = rsaKey
	})
	return pubKey, pubErr
}

func jsonNumber(v any) float64 {
	if n, ok := asNumber(v); ok {
		return n
	}
	s, _ := v.(string)
	f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f
}

func trimBody(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 512 {
		return s[:512]
	}
	return s
}
