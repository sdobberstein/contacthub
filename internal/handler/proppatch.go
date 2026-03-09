package handler

import (
	"errors"
	"net/http"

	"github.com/sdobberstein/contacthub/internal/auth"
	davxml "github.com/sdobberstein/contacthub/internal/dav/xml"
	"github.com/sdobberstein/contacthub/internal/store"
)

// PropPatchHandler handles PROPPATCH on any /dav/addressbooks/ resource.
// Only dead properties are supported; attempts to set live properties return 403.
func PropPatchHandler(st store.PropertyStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := auth.CurrentPrincipal(r.Context())
		if p == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		body, err := readBody(r)
		if err != nil || len(body) == 0 {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		pp, err := davxml.ParsePropPatch(body)
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		resource := r.URL.Path
		ms := davxml.NewMultistatus()
		var hit davxml.PropBuilder

		for _, op := range pp.Ops {
			// Refuse to set/remove DAV: live properties.
			if op.NS == davxml.NSdav {
				http.Error(w, "cannot set live DAV properties", http.StatusForbidden)
				return
			}

			var opErr error
			if op.Remove {
				opErr = st.DeleteProperty(r.Context(), resource, op.NS, op.Local)
				if errors.Is(opErr, store.ErrNotFound) {
					opErr = nil // deleting a non-existent property is a no-op
				}
			} else {
				opErr = st.SetProperty(r.Context(), &store.Property{
					Resource:  resource,
					Namespace: op.NS,
					Name:      op.Local,
					Value:     op.Value,
				})
			}

			if opErr != nil {
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}

			hit.AddCustomProp(op.NS, op.Local)
		}

		ms.AddResponse(resource, davxml.OK(hit.InnerXML()))
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write(ms.Bytes()) //nolint:errcheck // write to ResponseWriter, error unrecoverable
	}
}
