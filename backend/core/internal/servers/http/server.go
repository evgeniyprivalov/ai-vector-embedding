package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"gitlab.com/evgeniyprivalov/golib/observability/log"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	api "github.com/evgeniyprivalov/ai-vector-embedding/backend/core/internal/contracts/public/v1/openapi"
	server "github.com/evgeniyprivalov/ai-vector-embedding/backend/core/internal/servers/http/middlewares"
)

const (
	writeTimeout = 0
	readTimeout  = 45 * time.Second
	idleTimeout  = 60 * time.Second
)

// ServerHandlerOptions is a struct that contains the options for the server handler.
type ServerHandlerOptions struct {
	Router      *mux.Router
	Middlewares server.Middlewares
}

// Dependencies is a struct that contains the dependencies for the server.
type Dependencies struct {
	Logger      *log.Logger
	ServiceName string
}

// NewHTTPServer registers the server handler.
func NewHTTPServer(router *mux.Router, srv api.StrictServerInterface, dependencies *Dependencies) http.Handler {
	return NewServerHandler(
		srv,
		dependencies.Logger,
		ServerHandlerOptions{
			Router: router,
			Middlewares: server.NewMiddlewares(server.MiddlewareDependencies{
				Logger:      dependencies.Logger,
				ServiceName: dependencies.ServiceName,
			}),
		},
	)
}

// NewServerHandler creates a new server handler.
func NewServerHandler(server api.StrictServerInterface, logger *log.Logger, opts ServerHandlerOptions) http.Handler {
	return api.HandlerWithOptions(
		api.NewStrictHandlerWithOptions(
			server,
			nil,
			api.StrictHTTPServerOptions{
				RequestErrorHandlerFunc:  requestErrorHandler(logger),
				ResponseErrorHandlerFunc: responseErrorHandler(logger),
			},
		),
		api.GorillaServerOptions{
			BaseURL:            "/v1",
			BaseRouter:         opts.Router,
			Middlewares:        opts.Middlewares.Handler,
			ErrorHandlerFunc:   requestErrorHandler(logger),
			RequestMiddlewares: opts.Middlewares.Request,
		},
	)
}

// requestErrorHandler is a function that handles request errors.
func requestErrorHandler(logger *log.Logger) func(w http.ResponseWriter, r *http.Request, err error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		logger.WithCtx(r.Context()).Error("Error on request", "error", err.Error())

		resp, code := mapRequestError(err)
		respBody, _ := json.Marshal(resp)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_, _ = w.Write([]byte(respBody))
	}
}

// mapRequestError is a function that maps an error to a response and status code.
func mapRequestError(err error) (api.BaseResponse, int) {
	return api.BaseResponse{
		Message: "Bad Request",
	}, http.StatusBadRequest
}

// responseErrorHandler is a function that handles response errors.
func responseErrorHandler(logger *log.Logger) func(w http.ResponseWriter, r *http.Request, err error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		logger.WithCtx(r.Context()).Error("Error on response", "error", err.Error())

		resp, code := mapResponseError(err)
		respBody, _ := json.Marshal(resp)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_, _ = w.Write([]byte(respBody))
	}
}

// mapResponseError is a function that maps an error to a response and status code.
func mapResponseError(err error) (api.BaseResponse, int) {
	if errors.Is(err, context.DeadlineExceeded) {
		return api.BaseResponse{
			Message: "Gateway Timeout",
		}, http.StatusGatewayTimeout
	}

	var maxBytesError *http.MaxBytesError
	if errors.As(err, &maxBytesError) {
		return api.BaseResponse{
			Message: "Bad Request",
		}, http.StatusRequestEntityTooLarge
	}

	return api.BaseResponse{
		Message: err.Error(),
	}, http.StatusInternalServerError
}

// Server is a struct that contains the server.
type Server struct {
	addr   string
	server *http.Server
}

// CORS middleware to allow all origins.
func corsMiddleware(next http.Handler, allowedOrigins []string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		allowedOriginsMap := make(map[string]struct{}, len(allowedOrigins))
		for _, o := range allowedOrigins {
			allowedOriginsMap[o] = struct{}{}
		}

		if _, ok := allowedOriginsMap[origin]; ok {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		allowHeaders := []string{
			"Content-Type",
			"x-request-id",
		}

		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", strings.Join(allowHeaders, ","))

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// NewServer creates a new server.
func NewServer(h http.Handler, addr string, allowedOrigins []string) *Server {
	srv := &http.Server{
		WriteTimeout: writeTimeout,
		ReadTimeout:  readTimeout,
		IdleTimeout:  idleTimeout,
	}

	srv.Handler = otelhttp.NewHandler(corsMiddleware(h, allowedOrigins), "")

	httpServer := &Server{
		server: srv,
		addr:   addr,
	}

	return httpServer
}

// ListenAndServe listens and serves the server.
func (s *Server) ListenAndServe() error {
	s.server.Addr = s.addr

	return s.server.ListenAndServe()
}

// Shutdown shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}
