package server

import (
	"context"
	"crypto/tls"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"

	server "github.com/evgeniyprivalov/ai-vector-embedding/backend/core/internal/servers/http/middlewares"
)

const (
	grpcClientTime    = 5 * time.Second
	grpcClientTimeout = 1 * time.Minute
)

// InitGRPCClient - инициализация GRPC-клиента.
func InitGRPCClient(
	host string,
	secure bool,
) (*grpc.ClientConn, error) {
	creds := credentials.NewTLS(&tls.Config{
		InsecureSkipVerify: false,
		MinVersion:         tls.VersionTLS12,
	})
	if !secure {
		creds = insecure.NewCredentials()
	}

	return grpc.NewClient(
		host,
		grpc.WithTransportCredentials(creds),
		grpc.WithDisableHealthCheck(),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                grpcClientTime,
			Timeout:             grpcClientTimeout,
			PermitWithoutStream: true,
		}),
		grpc.WithChainUnaryInterceptor(
			grpcClientUnaryRequestIDProvider(),
		),
		grpc.WithChainStreamInterceptor(
			grpcClientStreamRequestIDProvider(),
		),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
}

// grpcClientUnaryRequestIDProvider is a function that provides a request ID to the GRPC client.
func grpcClientUnaryRequestIDProvider() func(
	ctx context.Context,
	method string,
	req interface{},
	reply interface{},
	cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption,
) error {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if requestID, _ := ctx.Value(server.RequestIDKeyString).(string); requestID != "" {
			ctx = metadata.AppendToOutgoingContext(ctx, server.RequestIDKeyString, requestID)
		}

		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// grpcClientStreamRequestIDProvider is a function that provides a request ID to the GRPC client.
func grpcClientStreamRequestIDProvider() func(
	ctx context.Context,
	desc *grpc.StreamDesc,
	cc *grpc.ClientConn,
	method string,
	streamer grpc.Streamer,
	opts ...grpc.CallOption,
) (grpc.ClientStream, error) {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		if requestID, _ := ctx.Value(server.RequestIDKeyString).(string); requestID != "" {
			ctx = metadata.AppendToOutgoingContext(ctx, server.RequestIDKeyString, requestID)
		}

		return streamer(ctx, desc, cc, method, opts...)
	}
}
