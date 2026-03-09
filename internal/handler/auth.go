// Package handler contains HTTP request handlers.
package handler

import (
	"errors"
	"html/template"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/sdobberstein/contacthub/internal/auth"
	"github.com/sdobberstein/contacthub/internal/config"
	"github.com/sdobberstein/contacthub/internal/middleware"
	"github.com/sdobberstein/contacthub/internal/store"
)

const maxFormBytes = 1 << 20 // 1 MiB

// LoginHandler returns GET+POST handlers for /auth/login.
func LoginHandler(st store.Store, provider auth.Provider, cfg config.AuthConfig, tmpl *template.Template) http.HandlerFunc {
	type data struct {
		Error string
	}

	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			renderTemplate(w, tmpl, "login.html", data{})

		case http.MethodPost:
			r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
			if err := r.ParseForm(); err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}

			username := r.FormValue("username")
			password := r.FormValue("password")

			user, err := provider.Authenticate(r.Context(), username, password)
			if err != nil {
				if errors.Is(err, auth.ErrInvalidCredentials) {
					renderTemplate(w, tmpl, "login.html", data{Error: "Invalid username or password."})
					return
				}
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}

			sessionID := uuid.NewString()
			now := time.Now()
			session := &store.Session{
				ID:        sessionID,
				UserID:    user.ID,
				IPAddress: r.RemoteAddr,
				UserAgent: r.UserAgent(),
				CreatedAt: now,
				ExpiresAt: now.Add(time.Duration(cfg.Session.MaxAge) * time.Second),
			}
			if err := st.CreateSession(r.Context(), session); err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}

			http.SetCookie(w, &http.Cookie{
				Name:     middleware.SessionCookieName,
				Value:    sessionID,
				Path:     "/",
				MaxAge:   cfg.Session.MaxAge,
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
			})

			http.Redirect(w, r, "/", http.StatusSeeOther)

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// LogoutHandler returns a POST handler for /auth/logout.
func LogoutHandler(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		cookie, err := r.Cookie(middleware.SessionCookieName)
		if err == nil {
			_ = st.DeleteSession(r.Context(), cookie.Value) //nolint:errcheck // best-effort; cookie cleared regardless
		}

		http.SetCookie(w, &http.Cookie{
			Name:     middleware.SessionCookieName,
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})

		http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
	}
}

func renderTemplate(w http.ResponseWriter, tmpl *template.Template, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}
