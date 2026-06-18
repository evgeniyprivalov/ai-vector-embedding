package server

import (
	"net/http"
	"runtime/debug"

	"github.com/gorilla/mux"
	"gitlab.com/evgeniyprivalov/golib/observability/log"
)

// PanicRecoveryMiddleware recovers from panics and logs the error.
func PanicRecoveryMiddleware(logger *log.Logger) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() { //nolint:contextcheck
				if err := recover(); err != nil {
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					logger.WithCtx(r.Context()).Error(
						"Panic recovered",
						"error", err,
						"stack", string(debug.Stack()),
					)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
