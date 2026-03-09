package handler

import (
	"errors"
	"html/template"
	"net/http"

	"github.com/sdobberstein/contacthub/internal/auth/local"
	"github.com/sdobberstein/contacthub/internal/store"
)

// SetupHandler returns GET+POST handlers for /setup (first-run admin creation).
// The SetupGuard middleware ensures this is only reachable when no users exist.
func SetupHandler(st store.Store, tmpl *template.Template) http.HandlerFunc {
	type data struct {
		Error string
	}

	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			renderTemplate(w, tmpl, "setup.html", data{})

		case http.MethodPost:
			r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
			if err := r.ParseForm(); err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}

			username := r.FormValue("username")
			password := r.FormValue("password")
			displayName := r.FormValue("display_name")

			if username == "" || password == "" {
				renderTemplate(w, tmpl, "setup.html", data{Error: "Username and password are required."})
				return
			}
			if len(password) < 8 {
				renderTemplate(w, tmpl, "setup.html", data{Error: "Password must be at least 8 characters."})
				return
			}

			_, err := local.CreateUser(r.Context(), st, username, displayName, password, true)
			if err != nil {
				if errors.Is(err, store.ErrConflict) {
					renderTemplate(w, tmpl, "setup.html", data{Error: "That username is already taken."})
					return
				}
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}

			http.Redirect(w, r, "/auth/login", http.StatusSeeOther)

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}
