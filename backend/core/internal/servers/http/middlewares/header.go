package server

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

const (
	maxRequestContentLength = 1 << 28 // 28 MB
)

const (
	RequestIDKeyString = "x-request-id"
)

func CheckRequestIDMiddleware() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = uuid.New().String()
			}

			r = r.WithContext(
				context.WithValue(r.Context(), RequestIDKeyString, requestID), //nolint:staticcheck
			)
			w.Header().Set("X-Request-ID", requestID)

			next.ServeHTTP(w, r)
		})
	}
}

func RequestSizeMiddleware() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > maxRequestContentLength {
				w.WriteHeader(http.StatusRequestEntityTooLarge)
				return
			}

			r.Body = http.MaxBytesReader(w, r.Body, r.ContentLength)
			next.ServeHTTP(w, r)
		})
	}
}
