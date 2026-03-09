// Package wellknown implements the /.well-known/carddav discovery endpoint (RFC 6764 §5).
package wellknown

import "net/http"

// Handler permanently redirects any request for /.well-known/carddav to the
// CardDAV context path /dav/. No authentication is required — the redirect
// target is a fixed, public path per RFC 6764 §5.
func Handler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/dav/", http.StatusMovedPermanently)
}
