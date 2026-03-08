package middleware

import (
	"context"
	"net"
	"net/http"
	"strings"
)

type contextKey int

const requestBaseURLKey contextKey = iota

// RequestInfo holds the derived base URL components for a request.
type RequestInfo struct {
	Scheme     string
	Host       string
	PathPrefix string
}

// BaseURL returns the full base URL (e.g. "https://contacts.example.com").
func (r RequestInfo) BaseURL() string {
	return r.Scheme + "://" + r.Host + r.PathPrefix
}

// RequestInfoFromContext retrieves the RequestInfo stored in the context.
func RequestInfoFromContext(ctx context.Context) RequestInfo {
	if v, ok := ctx.Value(requestBaseURLKey).(RequestInfo); ok {
		return v
	}
	return RequestInfo{Scheme: "http", Host: "localhost"}
}

// ProxyHeaders returns middleware that derives the canonical request base URL
// from RFC 7239 Forwarded / X-Forwarded-* headers, but only when the request
// originates from a trusted proxy IP or CIDR.
//
// If a static baseURL is configured it takes precedence over all headers.
func ProxyHeaders(trustedCIDRs []string, staticBaseURL, staticPathPrefix string) func(http.Handler) http.Handler {
	nets := parseCIDRs(trustedCIDRs)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			info := derive(r, nets, staticBaseURL, staticPathPrefix)
			ctx := context.WithValue(r.Context(), requestBaseURLKey, info)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func derive(r *http.Request, trusted []*net.IPNet, staticBaseURL, staticPathPrefix string) RequestInfo {
	if staticBaseURL != "" {
		scheme, host, _ := splitBaseURL(staticBaseURL)
		return RequestInfo{Scheme: scheme, Host: host, PathPrefix: staticPathPrefix}
	}

	info := RequestInfo{Scheme: "http", Host: r.Host, PathPrefix: staticPathPrefix}

	remoteIP := parseIP(r.RemoteAddr)
	if !isTrusted(remoteIP, trusted) {
		return info
	}

	// RFC 7239 Forwarded header takes precedence.
	if fwd := r.Header.Get("Forwarded"); fwd != "" {
		parseForwarded(fwd, &info)
		return info
	}

	// X-Forwarded-* fallback.
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		info.Scheme = strings.ToLower(strings.TrimSpace(proto))
	}
	if host := r.Header.Get("X-Forwarded-Host"); host != "" {
		info.Host = strings.TrimSpace(host)
	}
	if port := r.Header.Get("X-Forwarded-Port"); port != "" {
		// Only append port if not already in host and not the default for the scheme.
		if !strings.Contains(info.Host, ":") {
			if (info.Scheme != "https" || port != "443") && (info.Scheme != "http" || port != "80") {
				info.Host = info.Host + ":" + strings.TrimSpace(port)
			}
		}
	}
	if prefix := r.Header.Get("X-Forwarded-Prefix"); prefix != "" {
		info.PathPrefix = strings.TrimRight(strings.TrimSpace(prefix), "/")
	}

	return info
}

// parseForwarded parses a simple RFC 7239 Forwarded header (first directive only).
func parseForwarded(header string, info *RequestInfo) {
	// Take the first element (comma-separated list).
	first := strings.SplitN(header, ",", 2)[0]
	for _, part := range strings.Split(first, ";") {
		part = strings.TrimSpace(part)
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		v = strings.Trim(strings.TrimSpace(v), `"`)
		switch strings.ToLower(strings.TrimSpace(k)) {
		case "proto":
			info.Scheme = strings.ToLower(v)
		case "host":
			info.Host = v
		}
	}
}

func parseCIDRs(cidrs []string) []*net.IPNet {
	var nets []*net.IPNet
	for _, cidr := range cidrs {
		if !strings.Contains(cidr, "/") {
			cidr += "/32"
			if strings.Contains(cidr, ":") {
				cidr = strings.TrimSuffix(cidr, "/32") + "/128"
			}
		}
		_, n, err := net.ParseCIDR(cidr)
		if err == nil {
			nets = append(nets, n)
		}
	}
	return nets
}

func isTrusted(ip net.IP, nets []*net.IPNet) bool {
	if ip == nil {
		return false
	}
	for _, n := range nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

func parseIP(addr string) net.IP {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	return net.ParseIP(host)
}

func splitBaseURL(u string) (scheme, host, path string) {
	rest, ok := strings.CutPrefix(u, "https://")
	if ok {
		scheme = "https"
	} else {
		rest, _ = strings.CutPrefix(u, "http://")
		scheme = "http"
	}
	host, path, _ = strings.Cut(rest, "/")
	path = "/" + strings.TrimRight(path, "/")
	if path == "/" {
		path = ""
	}
	return scheme, host, path
}
