package wellknown_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sdobberstein/contacthub/internal/wellknown"
)

func TestHandler_RedirectStatus(t *testing.T) {
	r, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/.well-known/carddav", http.NoBody)
	w := httptest.NewRecorder()
	wellknown.Handler(3600)(w, r)

	if w.Code != http.StatusMovedPermanently {
		t.Errorf("want 301, got %d", w.Code)
	}
}

func TestHandler_RedirectTarget(t *testing.T) {
	r, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/.well-known/carddav", http.NoBody)
	w := httptest.NewRecorder()
	wellknown.Handler(3600)(w, r)

	loc := w.Header().Get("Location")
	if loc == "" {
		t.Fatal("want Location header, got none")
	}
	// Location may be absolute or relative; it must end with /dav/
	if len(loc) < len("/dav/") || loc[len(loc)-len("/dav/"):] != "/dav/" {
		t.Errorf("Location %q does not end with /dav/", loc)
	}
}

func TestHandler_NoAuthRequired(t *testing.T) {
	// Verify the redirect works without any credentials — no auth middleware applied.
	r, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/.well-known/carddav", http.NoBody)
	w := httptest.NewRecorder()
	wellknown.Handler(3600)(w, r) // must not panic or 401

	if w.Code == http.StatusUnauthorized {
		t.Error("well-known redirect must not require authentication")
	}
}

func TestHandler_AllMethodsRedirect(t *testing.T) {
	for _, method := range []string{http.MethodGet, "PROPFIND"} {
		t.Run(method, func(t *testing.T) {
			r, _ := http.NewRequestWithContext(context.Background(), method, "/.well-known/carddav", http.NoBody)
			w := httptest.NewRecorder()
			wellknown.Handler(3600)(w, r)
			if w.Code != http.StatusMovedPermanently {
				t.Errorf("%s: want 301, got %d", method, w.Code)
			}
		})
	}
}

func TestHandler_CacheControl_PositiveAge(t *testing.T) {
	for _, tt := range []struct {
		age  int
		want string
	}{
		{3600, "max-age=3600, public"},
		{86400, "max-age=86400, public"},
		{999999, "max-age=86400, public"}, // capped at 86400
	} {
		t.Run(fmt.Sprintf("age=%d", tt.age), func(t *testing.T) {
			r, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/.well-known/carddav", http.NoBody)
			w := httptest.NewRecorder()
			wellknown.Handler(tt.age)(w, r)
			if got := w.Header().Get("Cache-Control"); got != tt.want {
				t.Errorf("Cache-Control: want %q, got %q", tt.want, got)
			}
		})
	}
}

func TestHandler_CacheControl_NoCacheWhenZeroOrNegative(t *testing.T) {
	for _, age := range []int{0, -1} {
		t.Run(fmt.Sprintf("age=%d", age), func(t *testing.T) {
			r, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/.well-known/carddav", http.NoBody)
			w := httptest.NewRecorder()
			wellknown.Handler(age)(w, r)
			if got := w.Header().Get("Cache-Control"); got != "no-cache, must-revalidate" {
				t.Errorf("Cache-Control: want %q, got %q", "no-cache, must-revalidate", got)
			}
		})
	}
}
