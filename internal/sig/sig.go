package sig

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// Verify will check that the signature matches what we expected.
func Verify(signature, value, secret string) bool {
	token := hmac.New(sha256.New, []byte(secret))
	token.Write([]byte(value))
	expected := strings.ToLower(hex.EncodeToString(token.Sum(nil)))

	return signature == expected
}
