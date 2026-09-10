package smid

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func md5Hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

func jsString(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case string:
		return x
	case bool:
		if x {
			return "true"
		}
		return "false"
	case map[string]any, []kv:
		return "[object Object]"
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return jsNumberString(x)
	default:
		return fmt.Sprint(v)
	}
}

func jsNumberString(f float64) string {
	if f == 0 {
		return "0"
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

func asNumber(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case float64:
		return n, true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

func isEmpty(v any) bool {
	if v == nil {
		return true
	}
	s, ok := v.(string)
	return ok && s == ""
}

func smUserAgent() string {
	if v := strings.TrimSpace(os.Getenv("SM_UA")); v != "" {
		return v
	}
	return defaultUA
}

func smPlatform() string {
	if v := strings.TrimSpace(os.Getenv("SM_PLATFORM")); v != "" {
		return v
	}
	return defaultPlatform
}

func smPrevBox() string {
	return os.Getenv("SM_PREV_BOX")
}

func chargingBit() int {
	if randN(100) < 55 {
		return 1
	}
	return 0
}

func randN(n int) int {
	if n <= 0 {
		return 0
	}
	var b [1]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0
	}
	return int(b[0]) % n
}

func randFixed2(min, max float64) float64 {
	var b [2]byte
	if _, err := rand.Read(b[:]); err != nil {
		return min
	}
	u := float64(int(b[0])<<8|int(b[1])) / 65535.0
	v := min + u*(max-min)
	s := strconv.FormatFloat(v, 'f', 2, 64)
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return min
	}
	return f
}
