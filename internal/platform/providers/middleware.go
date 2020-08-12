package providers

import (
	"context"
	"fmt"
	"net/http"
)

type keyValue int

// ContextKey is the key for the provider.Provider value in the context.
const ContextKey keyValue = 1

// Middleware attaches the correct provider.Provider to the request so that
// the next handler can use it.
func Middleware(providers *Providers, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		provider := providers.Get(r.Host)
		if provider == nil {
			http.Error(w, fmt.Sprintf("No such host: %s", r.Host), http.StatusBadRequest)
			return
		}

		// Add the value to the context.
		ctx := context.WithValue(r.Context(), ContextKey, provider)

		// Merge the context onto the request.
		r = r.WithContext(ctx)

		// Pass the request down to the next handler
		next(w, r)
	}
}
