// Package wellknown implements the /.well-known/carddav discovery endpoint (RFC 6764 §5).
package wellknown

import (
	"fmt"
	"net/http"
)

const maxCacheAgeSeconds = 86400 // 1 day cap

// Handler returns an http.HandlerFunc that permanently redirects requests for
// /.well-known/carddav to the CardDAV context path /dav/. No authentication
// is required — the redirect target is a fixed, public path per RFC 6764 §5.
//
// cacheMaxAgeSeconds controls the Cache-Control header (RFC 6764 §5 SHOULD):
//   - > 0: "max-age=<n>, public" (capped at 86400 seconds)
//   - <= 0: "no-cache, must-revalidate"
func Handler(cacheMaxAgeSeconds int) http.HandlerFunc {
	var cacheControl string
	if cacheMaxAgeSeconds > 0 {
		age := cacheMaxAgeSeconds
		if age > maxCacheAgeSeconds {
			age = maxCacheAgeSeconds
		}
		cacheControl = fmt.Sprintf("max-age=%d, public", age)
	} else {
		cacheControl = "no-cache, must-revalidate"
	}

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", cacheControl)
		http.Redirect(w, r, "/dav/", http.StatusMovedPermanently)
	}
}
