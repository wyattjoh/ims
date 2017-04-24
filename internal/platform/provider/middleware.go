package provider

import (
	"context"
	"fmt"
	"net/http"

	"github.com/wyattjoh/ims/internal/image/provider"
)

type keyValue int

// ContextKey is the key for the provider.Provider value in the context.
const ContextKey keyValue = 1

// Middleware attaches the correct provider.Provider to the request so that
// the next handler can use it.
func Middleware(providers map[string]provider.Provider, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := providers[r.Host]
		if !ok {
			http.Error(w, fmt.Sprintf("No such host: %s", r.Host), http.StatusBadRequest)
			return
		}

		// Add the value to the context.
		ctx := context.WithValue(r.Context(), ContextKey, p)

		// Merge the context onto the request.
		r = r.WithContext(ctx)

		// Pass the request down to the next handler
		next(w, r)
	}
}
