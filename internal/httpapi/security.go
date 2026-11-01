package httpapi

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	chimw "github.com/go-chi/chi/v5/middleware"
)

// contentSecurityPolicy is safe for the built dashboard and the web terminal.
// Scripts and connections stay on this origin. Styles allow the compiled CSS
// plus the inline rules xterm injects, and the Google font stylesheet the
// shell already loads. Workers may be blob URLs, which xterm uses.
func contentSecurityPolicy() string {
	return strings.Join([]string{
		"default-src 'self'",
		"script-src 'self'",
		"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com",
		"font-src 'self' https://fonts.gstatic.com data:",
		"img-src 'self' data: blob:",
		"connect-src 'self'",
		"worker-src 'self' blob:",
		"manifest-src 'self'",
		"object-src 'none'",
		"base-uri 'self'",
		"form-action 'self'",
		"frame-ancestors 'self'",
	}, "; ")
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Content-Security-Policy", contentSecurityPolicy())
		next.ServeHTTP(w, r)
	})
}

// recoverJSON turns panics into the same JSON error shape as the rest of the API.
// http.ErrAbortHandler is re-raised so a disconnected client stays a disconnect.
func recoverJSON(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}
				if rec == http.ErrAbortHandler {
					panic(rec)
				}
				if logger != nil {
					logger.Error("panic", "err", fmt.Sprint(rec), "path", r.URL.Path, "request_id", chimw.GetReqID(r.Context()))
				}
				writeError(w, http.StatusInternalServerError, "internal error")
			}()
			next.ServeHTTP(w, r)
		})
	}
}
