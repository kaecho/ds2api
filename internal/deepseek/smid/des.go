package smid

import (
	"crypto/des"
	"encoding/base64"
	"fmt"
)

// desField matches js/sm_des.js DES(key, value, 1, 0): DES-ECB, zero-pad
// without an extra block when the plaintext is already aligned.
func desField(key, value string) (string, error) {
	k := []byte(key)
	if len(k) != des.BlockSize {
		return "", fmt.Errorf("des key length %d", len(k))
	}
	block, err := des.NewCipher(k)
	if err != nil {
		return "", err
	}
	plain := zeroPad([]byte(value), des.BlockSize)
	out := make([]byte, len(plain))
	for i := 0; i < len(plain); i += des.BlockSize {
		block.Encrypt(out[i:i+des.BlockSize], plain[i:i+des.BlockSize])
	}
	return base64.StdEncoding.EncodeToString(out), nil
}

func zeroPad(b []byte, bs int) []byte {
	if bs <= 0 || len(b)%bs == 0 {
		return b
	}
	n := bs - len(b)%bs
	out := make([]byte, len(b)+n)
	copy(out, b)
	return out
}
