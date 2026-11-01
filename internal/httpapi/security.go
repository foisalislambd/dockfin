package httpapi

import (
	"fmt"
	"log/slog"
	"net/http"

	chimw "github.com/go-chi/chi/v5/middleware"
)

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
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
