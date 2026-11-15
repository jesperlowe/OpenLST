package openlst_test

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"testing"
)

// extensionID beregner Chrome-extension-ID ud fra manifestets "key".
func extensionID(t *testing.T, key string) string {
	der, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(der)
	h := hex.EncodeToString(sum[:16])
	out := make([]byte, len(h))
	for i := range h {
		v := h[i] - '0'
		if h[i] >= 'a' {
			v = h[i] - 'a' + 10
		}
		out[i] = 'a' + v
	}
	return string(out)
}
