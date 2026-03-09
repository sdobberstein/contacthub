package handler

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/sdobberstein/contacthub/internal/auth"
	davxml "github.com/sdobberstein/contacthub/internal/dav/xml"
	"github.com/sdobberstein/contacthub/internal/store"
)

// HomeSetPropfind handles PROPFIND /dav/addressbooks/{username}/.
// Depth 0: returns properties of the home set collection.
// Depth 1: returns properties of the home set + each address book.
func HomeSetPropfind(st store.Store) http.HandlerFunc {
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

		depth := r.Header.Get("Depth")
		if depth == "infinity" {
			http.Error(w, "depth infinity not supported", http.StatusForbidden)
			return
		}

		ms := davxml.NewMultistatus()

		// Self response.
		selfProps := []liveProp{
			{davxml.NSdav, "resourcetype", func(b *davxml.PropBuilder) { b.AddDAVResourceType("collection") }},
			{davxml.NSdav, "displayname", func(b *davxml.PropBuilder) { b.AddDAVText("displayname", username) }},
		}
		ms.AddResponse(r.URL.Path, buildPropResponse(pf, selfProps)...)

		if depth == "1" {
			books, err := st.ListAddressBooks(r.Context(), user.ID)
			if err != nil {
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
			for _, ab := range books {
				bookURL := fmt.Sprintf("/dav/addressbooks/%s/%s/", username, ab.Name)
				ms.AddResponse(bookURL, buildAddressBookProps(pf, ab)...)
			}
		}

		writeMultistatus(w, ms)
	}
}

// AddressBookMkcol handles MKCOL /dav/addressbooks/{username}/{book}/.
// Creates a new CardDAV address book collection.
func AddressBookMkcol(st store.Store) http.HandlerFunc {
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

		// RFC 4918 §9.3: MKCOL body must be absent for simple MKCOL.
		// Extended MKCOL (RFC 5689) with a body is not implemented in Phase 6.
		body, _ := readBody(r) //nolint:errcheck // body read error handled by empty body check
		if len(body) > 0 {
			http.Error(w, "request body not supported for MKCOL", http.StatusUnsupportedMediaType)
			return
		}

		now := time.Now().UTC()
		ab := &store.AddressBook{
			ID:          uuid.New().String(),
			UserID:      user.ID,
			Name:        bookName,
			DisplayName: bookName,
			SyncToken:   0,
			CTag:        "0",
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		if err := st.CreateAddressBook(r.Context(), ab); err != nil {
			if errors.Is(err, store.ErrConflict) {
				// RFC 4918 §9.3: "405 (Method Not Allowed) — MKCOL can only be executed on an unmapped URL."
				http.Error(w, "collection already exists", http.StatusMethodNotAllowed)
				return
			}
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
	}
}

// AddressBookPropfind handles PROPFIND /dav/addressbooks/{username}/{book}/.
// Depth 0: returns properties of the address book.
// Depth 1: returns the address book + all its contacts.
func AddressBookPropfind(st store.Store) http.HandlerFunc {
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

		depth := r.Header.Get("Depth")
		if depth == "infinity" {
			http.Error(w, "depth infinity not supported", http.StatusForbidden)
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
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		pf, err := davxml.ParsePropFind(body)
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		ms := davxml.NewMultistatus()
		ms.AddResponse(r.URL.Path, buildAddressBookProps(pf, ab)...)

		if depth == "1" {
			contacts, err := st.ListContacts(r.Context(), ab.ID)
			if err != nil {
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
			for _, c := range contacts {
				contactURL := fmt.Sprintf("/dav/addressbooks/%s/%s/%s", username, bookName, c.Filename)
				ms.AddResponse(contactURL, buildContactProps(pf, c)...)
			}
		}

		writeMultistatus(w, ms)
	}
}

// AddressBookDelete handles DELETE /dav/addressbooks/{username}/{book}/.
func AddressBookDelete(st store.Store) http.HandlerFunc {
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

		if err := st.DeleteAddressBook(r.Context(), ab.ID); err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// --- helpers ---

func buildAddressBookProps(pf *davxml.PropFind, ab *store.AddressBook) []davxml.PropStatData {
	props := []liveProp{
		{
			davxml.NSdav, "resourcetype",
			func(b *davxml.PropBuilder) { b.AddAddressbookResourceType() },
		},
		{
			davxml.NSdav, "displayname",
			func(b *davxml.PropBuilder) { b.AddDAVText("displayname", ab.DisplayName) },
		},
		{
			davxml.NSdav, "sync-token",
			func(b *davxml.PropBuilder) {
				b.AddDAVText("sync-token", fmt.Sprintf("%d", ab.SyncToken))
			},
		},
		{
			davxml.NScarddav, "addressbook-description",
			func(b *davxml.PropBuilder) { b.AddDAVText("addressbook-description", ab.Description) },
		},
		{
			davxml.NSdav, "getlastmodified",
			func(b *davxml.PropBuilder) {
				b.AddDAVText("getlastmodified", ab.UpdatedAt.UTC().Format(http.TimeFormat))
			},
		},
	}
	return buildPropResponse(pf, props)
}

func buildContactProps(pf *davxml.PropFind, c *store.Contact) []davxml.PropStatData {
	size := len(c.VCard)
	props := []liveProp{
		{
			davxml.NSdav, "resourcetype",
			func(b *davxml.PropBuilder) { b.AddDAVResourceType() },
		},
		{
			davxml.NSdav, "getetag",
			func(b *davxml.PropBuilder) { b.AddDAVText("getetag", fmt.Sprintf("%q", c.ETag)) },
		},
		{
			davxml.NSdav, "getcontenttype",
			func(b *davxml.PropBuilder) { b.AddDAVText("getcontenttype", "text/vcard; charset=utf-8") },
		},
		{
			davxml.NSdav, "getcontentlength",
			func(b *davxml.PropBuilder) { b.AddDAVText("getcontentlength", fmt.Sprintf("%d", size)) },
		},
		{
			davxml.NSdav, "getlastmodified",
			func(b *davxml.PropBuilder) {
				b.AddDAVText("getlastmodified", c.UpdatedAt.UTC().Format(http.TimeFormat))
			},
		},
		{
			davxml.NSdav, "displayname",
			func(b *davxml.PropBuilder) { b.AddDAVText("displayname", c.FN) },
		},
	}
	return buildPropResponse(pf, props)
}
