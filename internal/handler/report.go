package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/sdobberstein/contacthub/internal/auth"
	davxml "github.com/sdobberstein/contacthub/internal/dav/xml"
	"github.com/sdobberstein/contacthub/internal/store"
)

// AddressBookReport handles REPORT /dav/addressbooks/{username}/{book}/.
// Dispatches to addressbook-query (RFC 6352 §8.6) or addressbook-multiget (RFC 6352 §8.7).
func AddressBookReport(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := auth.CurrentPrincipal(r.Context())
		if p == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		username := chi.URLParam(r, "username")
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

		bookName := chi.URLParam(r, "book")
		ab, err := st.GetAddressBookByName(r.Context(), user.ID, bookName)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				http.NotFound(w, r)
				return
			}
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		body, err := readBody(r)
		if err != nil || len(body) == 0 {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		query, multiget, err := davxml.ParseReport(body)
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		if query != nil {
			reportAddressbookQuery(w, r, st, ab, username, bookName, query)
			return
		}
		reportAddressbookMultiget(w, r, st, ab, multiget)
	}
}

// reportAddressbookQuery handles the addressbook-query report type (RFC 6352 §8.6).
func reportAddressbookQuery(
	w http.ResponseWriter, r *http.Request, st store.Store,
	ab *store.AddressBook, username, bookName string,
	q *davxml.AddressbookQuery,
) {
	pf := reportPropFind(q.Prop)
	filter := extractContactFilter(q.Filter)

	contacts, err := st.SearchContacts(r.Context(), ab.ID, filter)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	ms := davxml.NewMultistatus()
	for _, c := range contacts {
		contactURL := fmt.Sprintf("/dav/addressbooks/%s/%s/%s", username, bookName, c.Filename)
		ms.AddResponse(contactURL, buildContactReportProps(pf, c)...)
	}
	writeMultistatus(w, ms)
}

// reportAddressbookMultiget handles the addressbook-multiget report type (RFC 6352 §8.7).
func reportAddressbookMultiget(
	w http.ResponseWriter, r *http.Request, st store.Store,
	ab *store.AddressBook, m *davxml.AddressbookMultiget,
) {
	pf := reportPropFind(m.Prop)

	ms := davxml.NewMultistatus()
	for _, href := range m.Hrefs {
		filename := filenameFromHref(href)
		if filename == "" {
			ms.AddStatusResponse(href, http.StatusBadRequest)
			continue
		}
		c, err := st.GetContactByFilename(r.Context(), ab.ID, filename)
		if errors.Is(err, store.ErrNotFound) {
			// RFC 6352 §8.7: non-existent href MUST produce a 404 response entry.
			ms.AddStatusResponse(href, http.StatusNotFound)
			continue
		}
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		ms.AddResponse(href, buildContactReportProps(pf, c)...)
	}
	writeMultistatus(w, ms)
}

// --- helpers ---

// reportPropFind converts a REPORT's D:prop into a PropFind for buildPropResponse.
// A nil prop (absent from the request) is treated as allprop.
func reportPropFind(prop *davxml.ReqProp) *davxml.PropFind {
	if prop == nil {
		return &davxml.PropFind{AllProp: &struct{}{}}
	}
	return &davxml.PropFind{Prop: prop}
}

// extractContactFilter translates a parsed C:filter element into a store.ContactFilter.
// Returns nil when f is nil or contains no usable criteria.
func extractContactFilter(f *davxml.QueryFilter) *store.ContactFilter {
	if f == nil || len(f.PropFilters) == 0 {
		return nil
	}
	pf := f.PropFilters[0]
	if pf.TextMatch == nil || pf.TextMatch.Value == "" {
		return nil
	}
	return &store.ContactFilter{
		PropName:  pf.Name,
		TextMatch: pf.TextMatch.Value,
	}
}

// filenameFromHref extracts the last path component from an href.
func filenameFromHref(href string) string {
	href = strings.TrimSuffix(href, "/")
	idx := strings.LastIndex(href, "/")
	if idx < 0 {
		return href
	}
	return href[idx+1:]
}

// buildContactReportProps builds the prop response for a contact in a REPORT,
// extending the standard contact props with C:address-data (RFC 6352 §10.4).
func buildContactReportProps(pf *davxml.PropFind, c *store.Contact) []davxml.PropStatData {
	size := len(c.VCard)
	props := []liveProp{
		{davxml.NSdav, "resourcetype", func(b *davxml.PropBuilder) { b.AddDAVResourceType() }},
		{davxml.NSdav, "getetag", func(b *davxml.PropBuilder) {
			b.AddDAVText("getetag", fmt.Sprintf("%q", c.ETag))
		}},
		{davxml.NSdav, "getcontenttype", func(b *davxml.PropBuilder) {
			b.AddDAVText("getcontenttype", "text/vcard; charset=utf-8")
		}},
		{davxml.NSdav, "getcontentlength", func(b *davxml.PropBuilder) {
			b.AddDAVText("getcontentlength", fmt.Sprintf("%d", size))
		}},
		{davxml.NSdav, "getlastmodified", func(b *davxml.PropBuilder) {
			b.AddDAVText("getlastmodified", c.UpdatedAt.UTC().Format(http.TimeFormat))
		}},
		{davxml.NSdav, "displayname", func(b *davxml.PropBuilder) {
			b.AddDAVText("displayname", c.FN)
		}},
		{davxml.NScarddav, "address-data", func(b *davxml.PropBuilder) {
			b.AddAddressData(c.VCard)
		}},
	}
	return buildPropResponse(pf, props)
}
