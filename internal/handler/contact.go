package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/sdobberstein/contacthub/internal/auth"
	davxml "github.com/sdobberstein/contacthub/internal/dav/xml"
	"github.com/sdobberstein/contacthub/internal/store"
	"github.com/sdobberstein/contacthub/internal/vcard"
)

// ContactGet handles GET /dav/addressbooks/{username}/{book}/{filename}.
func ContactGet(st store.Store) http.HandlerFunc {
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

		ab, contact, ok := resolveContact(w, r, st)
		if !ok {
			return
		}
		_ = ab

		// Conditional GET: If-None-Match.
		if inm := r.Header.Get("If-None-Match"); inm != "" {
			if inm == fmt.Sprintf("%q", contact.ETag) {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}

		w.Header().Set("Content-Type", "text/vcard; charset=utf-8")
		w.Header().Set("ETag", fmt.Sprintf("%q", contact.ETag))
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(contact.VCard)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(contact.VCard)) //nolint:errcheck // write to ResponseWriter, error unrecoverable
	}
}

// ContactPut handles PUT /dav/addressbooks/{username}/{book}/{filename}.
// Creates a new contact (201) or updates an existing one (204).
func ContactPut(st store.Store) http.HandlerFunc {
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
		filename := chi.URLParam(r, "filename")

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

		blob := string(body)
		etag := vcard.ComputeETag(blob)
		uid := vcard.ExtractUID(blob)
		if uid == "" {
			// Assign a UID if the client didn't provide one.
			uid = vcard.NewUID()
			blob = vcard.ReplaceUID(blob, uid)
			etag = vcard.ComputeETag(blob)
		}

		now := time.Now().UTC()

		// Look for an existing contact.
		existing, err := st.GetContactByFilename(r.Context(), ab.ID, filename)
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		if existing != nil {
			// Update: check If-Match precondition.
			if ifMatch := r.Header.Get("If-Match"); ifMatch != "" {
				if ifMatch != fmt.Sprintf("%q", existing.ETag) {
					http.Error(w, "precondition failed", http.StatusPreconditionFailed)
					return
				}
			}

			existing.UID = uid
			existing.ETag = etag
			existing.VCard = blob
			existing.FN = vcard.ExtractFN(blob)
			existing.Kind = vcard.ExtractKind(blob)
			existing.Organization = vcard.ExtractOrg(blob)
			existing.Birthday = vcard.ExtractBDay(blob)
			existing.Anniversary = vcard.ExtractAnniversary(blob)
			existing.UpdatedAt = now

			if err := st.UpdateContact(r.Context(), existing); err != nil {
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
		} else {
			// If-None-Match: * means refuse if the resource already exists.
			if r.Header.Get("If-None-Match") == "*" {
				http.Error(w, "precondition failed", http.StatusPreconditionFailed)
				return
			}

			c := &store.Contact{
				ID:            uuid.New().String(),
				UID:           uid,
				AddressBookID: ab.ID,
				Filename:      filename,
				ETag:          etag,
				VCard:         blob,
				FN:            vcard.ExtractFN(blob),
				Kind:          vcard.ExtractKind(blob),
				Organization:  vcard.ExtractOrg(blob),
				Birthday:      vcard.ExtractBDay(blob),
				Anniversary:   vcard.ExtractAnniversary(blob),
				CreatedAt:     now,
				UpdatedAt:     now,
			}
			if err := st.CreateContact(r.Context(), c); err != nil {
				if errors.Is(err, store.ErrConflict) {
					http.Error(w, "conflict", http.StatusConflict)
					return
				}
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
		}

		if _, err := st.BumpSyncToken(r.Context(), ab.ID); err != nil {
			// Non-fatal: sync token failure doesn't affect the client's contact.
			_ = err
		}

		w.Header().Set("ETag", fmt.Sprintf("%q", etag))
		if existing != nil {
			w.WriteHeader(http.StatusNoContent)
		} else {
			w.WriteHeader(http.StatusCreated)
		}
	}
}

// ContactDelete handles DELETE /dav/addressbooks/{username}/{book}/{filename}.
func ContactDelete(st store.Store) http.HandlerFunc {
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

		ab, contact, ok := resolveContact(w, r, st)
		if !ok {
			return
		}

		// If-Match precondition.
		if ifMatch := r.Header.Get("If-Match"); ifMatch != "" {
			if ifMatch != fmt.Sprintf("%q", contact.ETag) {
				http.Error(w, "precondition failed", http.StatusPreconditionFailed)
				return
			}
		}

		if err := st.DeleteContact(r.Context(), contact.ID); err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		if _, err := st.BumpSyncToken(r.Context(), ab.ID); err != nil {
			_ = err
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// ContactPropfind handles PROPFIND /dav/addressbooks/{username}/{book}/{filename}.
func ContactPropfind(st store.Store) http.HandlerFunc {
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

		depth := r.Header.Get("Depth")
		if depth == "infinity" {
			http.Error(w, "depth infinity not supported", http.StatusForbidden)
			return
		}

		_, contact, ok := resolveContact(w, r, st)
		if !ok {
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
		propstats := buildContactProps(pf, contact)

		// Include dead properties (RFC 4918 §9.2): for allprop list all stored
		// properties; for a specific request look up each non-live property.
		deadProps, deadErr := deadPropertyPropstats(r.Context(), st, r.URL.Path, pf)
		if deadErr != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		propstats = append(propstats, deadProps...)

		ms.AddResponse(r.URL.Path, propstats...)
		writeMultistatus(w, ms)
	}
}

// ContactCopy handles COPY /dav/addressbooks/{username}/{book}/{filename}.
func ContactCopy(st store.Store) http.HandlerFunc {
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

		ab, contact, ok := resolveContact(w, r, st)
		if !ok {
			return
		}

		dstPath, err := destinationPath(r)
		if err != nil {
			http.Error(w, "bad destination header", http.StatusBadRequest)
			return
		}

		dstBookName, dstFilename, err := parseContactPath(dstPath)
		if err != nil {
			http.Error(w, "invalid destination path", http.StatusBadRequest)
			return
		}

		// Overwrite header: "T" (default) or "F".
		overwrite := strings.ToUpper(r.Header.Get("Overwrite")) != "F"

		// Resolve destination address book (may be same or different book).
		dstUser, err := st.GetUserByUsername(r.Context(), username)
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		dstAB, err := st.GetAddressBookByName(r.Context(), dstUser.ID, dstBookName)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				http.NotFound(w, r)
				return
			}
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		// Check for existing destination.
		existing, err := st.GetContactByFilename(r.Context(), dstAB.ID, dstFilename)
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if existing != nil && !overwrite {
			http.Error(w, "destination exists", http.StatusPreconditionFailed)
			return
		}

		// For copies within the same address book, generate a new UID to satisfy
		// the UNIQUE(address_book_id, uid) constraint.
		newBlob := contact.VCard
		newUID := contact.UID
		if dstAB.ID == ab.ID {
			newUID = vcard.NewUID()
			newBlob = vcard.ReplaceUID(newBlob, newUID)
		}
		newETag := vcard.ComputeETag(newBlob)
		now := time.Now().UTC()

		if existing != nil {
			// Overwrite existing destination.
			existing.UID = newUID
			existing.ETag = newETag
			existing.VCard = newBlob
			existing.FN = vcard.ExtractFN(newBlob)
			existing.Kind = vcard.ExtractKind(newBlob)
			existing.Organization = vcard.ExtractOrg(newBlob)
			existing.Birthday = vcard.ExtractBDay(newBlob)
			existing.Anniversary = vcard.ExtractAnniversary(newBlob)
			existing.UpdatedAt = now
			if err := st.UpdateContact(r.Context(), existing); err != nil {
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
			if _, err := st.BumpSyncToken(r.Context(), dstAB.ID); err != nil {
				_ = err
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}

		dst := &store.Contact{
			ID:            uuid.New().String(),
			UID:           newUID,
			AddressBookID: dstAB.ID,
			Filename:      dstFilename,
			ETag:          newETag,
			VCard:         newBlob,
			FN:            vcard.ExtractFN(newBlob),
			Kind:          vcard.ExtractKind(newBlob),
			Organization:  vcard.ExtractOrg(newBlob),
			Birthday:      vcard.ExtractBDay(newBlob),
			Anniversary:   vcard.ExtractAnniversary(newBlob),
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		if err := st.CreateContact(r.Context(), dst); err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if _, err := st.BumpSyncToken(r.Context(), dstAB.ID); err != nil {
			_ = err
		}
		w.WriteHeader(http.StatusCreated)
	}
}

// ContactMove handles MOVE /dav/addressbooks/{username}/{book}/{filename}.
func ContactMove(st store.Store) http.HandlerFunc {
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

		ab, contact, ok := resolveContact(w, r, st)
		if !ok {
			return
		}

		dstPath, err := destinationPath(r)
		if err != nil {
			http.Error(w, "bad destination header", http.StatusBadRequest)
			return
		}

		_, dstFilename, err := parseContactPath(dstPath)
		if err != nil {
			http.Error(w, "invalid destination path", http.StatusBadRequest)
			return
		}

		overwrite := strings.ToUpper(r.Header.Get("Overwrite")) != "F"

		// Check if destination already exists.
		existing, err := st.GetContactByFilename(r.Context(), ab.ID, dstFilename)
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if existing != nil && !overwrite {
			http.Error(w, "destination exists", http.StatusPreconditionFailed)
			return
		}
		if existing != nil {
			if err := st.DeleteContact(r.Context(), existing.ID); err != nil {
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
		}

		contact.Filename = dstFilename
		contact.UpdatedAt = time.Now().UTC()
		if err := st.UpdateContact(r.Context(), contact); err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		if _, err := st.BumpSyncToken(r.Context(), ab.ID); err != nil {
			_ = err
		}
		w.WriteHeader(http.StatusCreated)
	}
}

// --- helpers ---

// resolveContact looks up the address book and contact for a request using
// the {username}, {book}, and {filename} URL parameters.
// On error it writes an appropriate HTTP response and returns ok=false.
func resolveContact(w http.ResponseWriter, r *http.Request, st store.Store) (*store.AddressBook, *store.Contact, bool) {
	username := chi.URLParam(r, "username")
	bookName := chi.URLParam(r, "book")
	filename := chi.URLParam(r, "filename")

	user, err := st.GetUserByUsername(r.Context(), username)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.NotFound(w, r)
		} else {
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return nil, nil, false
	}

	ab, err := st.GetAddressBookByName(r.Context(), user.ID, bookName)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.NotFound(w, r)
		} else {
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return nil, nil, false
	}

	contact, err := st.GetContactByFilename(r.Context(), ab.ID, filename)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.NotFound(w, r)
		} else {
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return nil, nil, false
	}

	return ab, contact, true
}

// destinationPath extracts the URL path from the Destination request header.
// The Destination may be an absolute URL or an absolute path.
func destinationPath(r *http.Request) (string, error) {
	dst := r.Header.Get("Destination")
	if dst == "" {
		return "", fmt.Errorf("missing Destination header")
	}
	if strings.HasPrefix(dst, "http://") || strings.HasPrefix(dst, "https://") {
		u, err := url.Parse(dst)
		if err != nil {
			return "", fmt.Errorf("parse Destination URL: %w", err)
		}
		return u.Path, nil
	}
	return dst, nil
}

// deadPropertyPropstats fetches stored dead properties and returns them as PropStatData.
// For allprop: all stored properties are returned in a 200 propstat.
// For a specific prop request: each requested non-live property is looked up;
// found ones go into 200, missing ones into 404.
func deadPropertyPropstats(ctx context.Context, st store.PropertyStore, resource string, pf *davxml.PropFind) ([]davxml.PropStatData, error) {
	if pf.IsAllProp() || pf.IsPropName() {
		props, err := st.ListProperties(ctx, resource)
		if err != nil {
			return nil, err
		}
		if len(props) == 0 {
			return nil, nil
		}
		var b davxml.PropBuilder
		for _, p := range props {
			b.AddCustomPropValue(p.Namespace, p.Name, p.Value)
		}
		return []davxml.PropStatData{davxml.OK(b.InnerXML())}, nil
	}

	// Specific prop request — check each requested prop against the live prop set,
	// then look up unknowns in the property store.
	liveProps := map[string]bool{
		davxml.NSdav + " resourcetype":      true,
		davxml.NSdav + " getetag":           true,
		davxml.NSdav + " getcontenttype":    true,
		davxml.NSdav + " getcontentlength":  true,
		davxml.NSdav + " getlastmodified":   true,
		davxml.NSdav + " displayname":       true,
	}

	var found, missing davxml.PropBuilder
	for _, name := range pf.RequestedProps() {
		if liveProps[name.Space+" "+name.Local] {
			continue // handled by buildContactProps
		}
		p, err := st.GetProperty(ctx, resource, name.Space, name.Local)
		if errors.Is(err, store.ErrNotFound) {
			missing.AddCustomProp(name.Space, name.Local)
			continue
		}
		if err != nil {
			return nil, err
		}
		found.AddCustomPropValue(p.Namespace, p.Name, p.Value)
	}

	var out []davxml.PropStatData
	if inner := found.InnerXML(); len(inner) > 0 {
		out = append(out, davxml.OK(inner))
	}
	if inner := missing.InnerXML(); len(inner) > 0 {
		out = append(out, davxml.NotFoundRaw(inner))
	}
	return out, nil
}

// parseContactPath extracts the address book name and filename from a contact path.
// Expects the form /dav/addressbooks/{username}/{book}/{filename}.
func parseContactPath(path string) (bookName, filename string, err error) {
	// Strip leading slash and split.
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	// Expected: ["dav", "addressbooks", username, book, filename]
	if len(parts) < 5 || parts[0] != "dav" || parts[1] != "addressbooks" {
		return "", "", fmt.Errorf("unexpected contact path: %q", path)
	}
	return parts[3], parts[4], nil
}
