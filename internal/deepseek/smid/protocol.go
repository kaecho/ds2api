package smid

import (
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	smOrg           = "P9usCUBauxft8eAmUXaZ"
	smApp           = "default"
	smVersion       = "3.0.0"
	defaultUA       = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
	defaultPlatform = "MacIntel"
)

var screenProfiles = []struct {
	res, clientSize string
}{
	{"1920_1080_1920_1040", "1920_937_1920_1040"},
	{"1920_1080_1920_1040", "1536_791_1536_864"},
	{"2560_1440_2560_1400", "2560_1295_2560_1400"},
	{"2560_1440_2560_1400", "1920_1009_1920_1080"},
	{"1440_900_1440_860", "1440_757_1440_860"},
	{"1536_864_1536_824", "1536_721_1536_824"},
	{"1680_1050_1680_1010", "1680_907_1680_1010"},
	{"1366_768_1366_728", "1366_625_1366_728"},
	{"2880_1800_2880_1740", "1440_821_1440_900"},
	{"3024_1964_3024_1904", "1512_916_1512_982"},
}

var cpuCounts = []int{4, 6, 8, 10, 12, 16}

type kv struct {
	k string
	v any
}

func buildBox(now time.Time, prev string) ([]kv, string, error) {
	conf, err := loadConf()
	if err != nil {
		return nil, "", err
	}
	uid := uuid.NewString()
	ms := now.UnixMilli()
	screen := screenProfiles[randN(len(screenProfiles))]
	ua := smUserAgent()
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, "", err
	}
	canvas := md5Hex(ua + "|" + uid + "|" + hex.EncodeToString(salt))
	if len(canvas) > 32 {
		canvas = canvas[:32]
	}
	_, tzOff := now.Zone()
	box := []kv{
		{"protocol", conf.Protocol},
		{"organization", smOrg},
		{"appId", smApp},
		{"os", "web"},
		{"version", smVersion},
		{"sdkver", smVersion},
		{"box", prev},
		{"rtype", "all"},
		{"smid", localSMID(now)},
		{"subVersion", "1.0.0"},
		{"time", float64(ms) / 1000},
		{"plugins", "Chrome PDF Viewer::application/pdf~pdf,Chromium PDF Viewer::application/pdf~pdf,Microsoft Edge PDF Viewer::application/pdf~pdf,PDF Viewer::application/pdf~pdf,WebKit built-in PDF::application/pdf~pdf"},
		{"ua", ua},
		{"canvas", canvas},
		{"timezone", tzOff / 60},
		{"platform", smPlatform()},
		{"url", "https://platform.deepseek.com/sign_up"},
		{"referer", ""},
		{"res", screen.res},
		{"clientSize", screen.clientSize},
		{"status", "true"},
		{"vpw", uid},
		{"svm", float64(ms)},
		{"trees", uuid.NewString()},
		{"pmf", float64(ms)},
		{"cdp", 0},
		{"maxTouchPoints", 0},
		{"connectionRtt", 20 + randN(101)},
		{"cpucount", cpuCounts[randN(len(cpuCounts))]},
		{"battery", map[string]any{
			"charging": chargingBit(),
			"level":    randFixed2(0.28, 1.0),
		}},
	}
	tn := tnOf(box)
	box = append(box, kv{"tn", tn})
	box[len(box)-1].v = tnOf(box)
	return box, uid, nil
}

func confuse(box []kv) ([]kv, error) {
	conf, err := loadConf()
	if err != nil {
		return nil, err
	}
	out := make([]kv, 0, len(box))
	for _, p := range box {
		rule, ok := conf.ConfusionInfo.Data[p.k]
		if !ok {
			out = append(out, p)
			continue
		}
		val := p.v
		if rule.IsEncrypt != 0 && !isEmpty(val) && rule.Cipher == "DES" {
			enc, err := desField(rule.Key, jsString(val))
			if err != nil {
				return nil, err
			}
			val = enc
		}
		out = append(out, kv{rule.ObfuscatedName, val})
	}
	return out, nil
}

func tnOf(v any) string {
	return md5Hex(walkTN(v))
}

func walkTN(v any) string {
	switch o := v.(type) {
	case []kv:
		m := make(map[string]any, len(o))
		for _, p := range o {
			m[p.k] = p.v
		}
		return walkTN(m)
	case map[string]any:
		keys := make([]string, 0, len(o))
		for k := range o {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var b strings.Builder
		for _, k := range keys {
			child := o[k]
			if n, ok := asNumber(child); ok {
				b.WriteString(walkTN(jsNumberString(10000 * n)))
			} else {
				b.WriteString(walkTN(jsString(child)))
			}
		}
		return b.String()
	case string:
		if o == "" {
			return ""
		}
		return o
	default:
		s := jsString(v)
		if s == "" {
			return ""
		}
		return s
	}
}

func localSMID(now time.Time) string {
	ts := now.Format("20060102150405")
	u := uuid.NewString()
	mid := ts + md5Hex(u) + "00"
	return mid + md5Hex("smsk_web_" + mid)[:14] + "0"
}

func aesEncryptHex(plaintext, priID string) (string, error) {
	var gz bytes.Buffer
	w := gzip.NewWriter(&gz)
	if _, err := w.Write([]byte(plaintext)); err != nil {
		_ = w.Close()
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}
	gzB64 := base64.StdEncoding.EncodeToString(gz.Bytes())
	padded := zeroPad([]byte(gzB64), aes.BlockSize)
	block, err := aes.NewCipher([]byte(priID))
	if err != nil {
		return "", err
	}
	mode := cipher.NewCBCEncrypter(block, []byte("0102030405060708"))
	out := make([]byte, len(padded))
	mode.CryptBlocks(out, padded)
	return hex.EncodeToString(out), nil
}

func rsaEncryptUID(pub *rsa.PublicKey, uid string) (string, error) {
	b, err := rsa.EncryptPKCS1v15(rand.Reader, pub, []byte(uid))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

func marshalObject(pairs []kv) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, p := range pairs {
		if i > 0 {
			buf.WriteByte(',')
		}
		kb, err := json.Marshal(p.k)
		if err != nil {
			return nil, err
		}
		buf.Write(kb)
		buf.WriteByte(':')
		var vb bytes.Buffer
		enc := json.NewEncoder(&vb)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(p.v); err != nil {
			return nil, err
		}
		buf.Write(bytes.TrimSpace(vb.Bytes()))
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}
