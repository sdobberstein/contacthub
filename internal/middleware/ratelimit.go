package middleware

import (
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/sdobberstein/contacthub/internal/config"
)

type ipLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// LoginRateLimiter returns middleware that rate-limits POST requests by client IP.
// It uses a token-bucket with burst=MaxAttempts and a refill rate derived from
// MaxAttempts/Window seconds.
func LoginRateLimiter(cfg config.RateLimitConfig) func(http.Handler) http.Handler {
	var (
		mu       sync.Mutex
		limiters = make(map[string]*ipLimiter)
	)

	r := rate.Every(time.Duration(cfg.Window) * time.Second / time.Duration(cfg.MaxAttempts))
	burst := cfg.MaxAttempts

	// Background cleanup of stale entries.
	go func() {
		ticker := time.NewTicker(time.Duration(cfg.Window) * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			mu.Lock()
			for ip, il := range limiters {
				if time.Since(il.lastSeen) > time.Duration(cfg.Window)*time.Second*2 {
					delete(limiters, ip)
				}
			}
			mu.Unlock()
		}
	}()

	getLimiter := func(ip string) *rate.Limiter {
		mu.Lock()
		defer mu.Unlock()
		il, ok := limiters[ip]
		if !ok {
			il = &ipLimiter{limiter: rate.NewLimiter(r, burst)}
			limiters[ip] = il
		}
		il.lastSeen = time.Now()
		return il.limiter
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				ip := clientIP(r)
				if !getLimiter(ip).Allow() {
					w.Header().Set("Retry-After", "60")
					http.Error(w, "too many requests", http.StatusTooManyRequests)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// clientIP extracts the request IP, preferring the value set by ProxyHeaders middleware.
func clientIP(r *http.Request) string {
	// ProxyHeaders middleware sets RemoteAddr to the real client IP when trusted.
	ip := r.RemoteAddr
	// Strip port if present.
	if i := len(ip) - 1; i >= 0 {
		for i >= 0 && ip[i] != ':' && ip[i] != ']' {
			i--
		}
		if i >= 0 && ip[i] == ':' {
			ip = ip[:i]
		}
	}
	return ip
}
