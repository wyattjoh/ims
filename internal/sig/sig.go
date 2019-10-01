package sig

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strings"
)

// Verify will check that the signature matches what we expected.
func Verify(signature, value, secret string) bool {
	token := hmac.New(sha256.New, []byte(secret))
	if _, err := token.Write([]byte(value)); err != nil {
		return false
	}

	expected := strings.ToLower(hex.EncodeToString(token.Sum(nil)))

	return subtle.ConstantTimeCompare([]byte(signature), []byte(expected)) == 1
}
