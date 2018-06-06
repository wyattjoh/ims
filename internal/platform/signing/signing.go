package signing

import (
	"net/http"

	"github.com/wyattjoh/ims/internal/sig"
)

const sigKey = "sig"

func getValue(r *http.Request, includePath bool) string {
	// Get the values from the query.
	values := r.URL.Query()

	// Remove the signature from the query.
	values.Del(sigKey)

	// Return the encoded string without the signature.
	value := values.Encode()

	if includePath {
		// Optionally include the path in the signing value if requested.
		value = r.URL.Path + "?" + value
	}

	return value
}

// Middleware wraps the request to ensure that the request itself contains only
// those query parameters that are permitted and signed.
func Middleware(secret string, includePath bool, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get the signature for the request.
		signature := r.URL.Query().Get(sigKey)
		if signature == "" {
			http.Error(w, "Signature invalid", http.StatusUnauthorized)
			return
		}

		// Get the string that was signed by the client.
		value := getValue(r, includePath)

		// Verify that the signature is valid.
		if !sig.Verify(signature, value, secret) {
			http.Error(w, "Signature invalid", http.StatusUnauthorized)
			return
		}

		next(w, r)
	}
}
