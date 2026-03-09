package handler

import (
	"fmt"
	"net/http"

	"github.com/sdobberstein/contacthub/internal/auth"
	davxml "github.com/sdobberstein/contacthub/internal/dav/xml"
)

// DAVRootPropfind handles PROPFIND /dav/ — the CardDAV context path (RFC 6764 §5).
// Returns DAV:current-user-principal so clients can discover the principal URL
// after following the /.well-known/carddav redirect.
func DAVRootPropfind(w http.ResponseWriter, r *http.Request) {
	p := auth.CurrentPrincipal(r.Context())
	if p == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	body, err := readBody(r)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	pf, err := davxml.ParsePropFind(body)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	principalURL := fmt.Sprintf("/dav/principals/users/%s/", p.Username)

	props := []liveProp{
		{
			davxml.NSdav, "resourcetype",
			func(b *davxml.PropBuilder) { b.AddDAVResourceType("collection") },
		},
		{
			davxml.NSdav, "current-user-principal",
			func(b *davxml.PropBuilder) { b.AddDAVHref("current-user-principal", principalURL) },
		},
		{
			davxml.NSdav, "principal-URL",
			func(b *davxml.PropBuilder) { b.AddDAVHref("principal-URL", principalURL) },
		},
	}

	ms := davxml.NewMultistatus()
	ms.AddResponse(r.URL.Path, buildPropResponse(pf, props)...)
	writeMultistatus(w, ms)
}
