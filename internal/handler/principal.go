package handler

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/sdobberstein/contacthub/internal/auth"
	davxml "github.com/sdobberstein/contacthub/internal/dav/xml"
	"github.com/sdobberstein/contacthub/internal/store"
)

// PrincipalPropfind handles PROPFIND /dav/principals/users/{username}/.
// Returns DAV live properties including carddav:addressbook-home-set (RFC 6352 §6.2.1).
func PrincipalPropfind(st store.UserStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := auth.CurrentPrincipal(r.Context())
		if p == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		username := chi.URLParam(r, "username")

		// Only the principal themselves (or an admin) may read this resource.
		if p.Username != username && !p.IsAdmin {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		user, err := st.GetUserByUsername(r.Context(), username)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				http.NotFound(w, r)
				return
			}
			http.Error(w, "internal server error", http.StatusInternalServerError)
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

		principalURL := fmt.Sprintf("/dav/principals/users/%s/", username)
		homeSet := fmt.Sprintf("/dav/addressbooks/%s/", username)
		displayName := user.DisplayName
		if displayName == "" {
			displayName = user.Username
		}

		props := []liveProp{
			{
				davxml.NSdav, "resourcetype",
				func(b *davxml.PropBuilder) { b.AddDAVResourceType("collection", "principal") },
			},
			{
				davxml.NSdav, "displayname",
				func(b *davxml.PropBuilder) { b.AddDAVText("displayname", displayName) },
			},
			{
				davxml.NSdav, "current-user-principal",
				func(b *davxml.PropBuilder) { b.AddDAVHref("current-user-principal", principalURL) },
			},
			{
				davxml.NSdav, "principal-URL",
				func(b *davxml.PropBuilder) { b.AddDAVHref("principal-URL", principalURL) },
			},
			{
				davxml.NScarddav, "addressbook-home-set",
				func(b *davxml.PropBuilder) { b.AddCardDAVHref("addressbook-home-set", homeSet) },
			},
		}

		ms := davxml.NewMultistatus()
		ms.AddResponse(r.URL.Path, buildPropResponse(pf, props)...)
		writeMultistatus(w, ms)
	}
}
