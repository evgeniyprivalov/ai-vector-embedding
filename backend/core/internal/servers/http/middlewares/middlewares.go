package server

import (
	"gitlab.com/evgeniyprivalov/golib/observability/log"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"

	api "github.com/evgeniyprivalov/ai-vector-embedding/backend/core/internal/contracts/public/v1/openapi"
)

type (
	ClientIPKey  struct{}
	OperationKey struct{}
	UserAgentKey struct{}
)

// Middlewares is a struct that contains the request and handler middlewares.
type Middlewares struct {
	Request []api.MiddlewareFunc
	Handler []api.MiddlewareFunc
}

// MiddlewareDependencies is a struct that contains the dependencies for the middlewares.
type MiddlewareDependencies struct {
	ServiceName string
	Logger      *log.Logger
}

// NewMiddlewares creates a new Middlewares struct.
func NewMiddlewares(deps MiddlewareDependencies) Middlewares {
	return Middlewares{
		Request: []api.MiddlewareFunc{
			api.MiddlewareFunc(RequestSizeMiddleware()),
			api.MiddlewareFunc(otelmux.Middleware(deps.ServiceName)),
			api.MiddlewareFunc(LoggingMiddleware(deps.Logger)),
			api.MiddlewareFunc(PanicRecoveryMiddleware(deps.Logger)),
			api.MiddlewareFunc(CheckRequestIDMiddleware()), // first
		},
		Handler: []api.MiddlewareFunc{},
	}
}
