package main

import (
	"context"
	"errors"
	"fmt"
	api2 "github.com/evgeniyprivalov/ai-vector-embedding/backend/core/internal/contracts/public/sse/v1/openapi"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/joho/godotenv/autoload"
	"github.com/kelseyhightower/envconfig"
	"github.com/sashabaranov/go-openai"
	"gitlab.com/evgeniyprivalov/golib/observability/log"
	"gitlab.com/evgeniyprivalov/golib/observability/metric"
	"gitlab.com/evgeniyprivalov/golib/observability/trace"
	"gitlab.com/evgeniyprivalov/golib/pg"
	"golang.org/x/sync/errgroup"

	"github.com/evgeniyprivalov/ai-vector-embedding/backend/core/internal/config"
	"github.com/evgeniyprivalov/ai-vector-embedding/backend/core/internal/contracts/chunker/v1"
	api "github.com/evgeniyprivalov/ai-vector-embedding/backend/core/internal/contracts/public/v1/openapi"
	"github.com/evgeniyprivalov/ai-vector-embedding/backend/core/internal/repository"
	server "github.com/evgeniyprivalov/ai-vector-embedding/backend/core/internal/servers/http"
	"github.com/evgeniyprivalov/ai-vector-embedding/backend/core/internal/services/document_handler"
	"github.com/evgeniyprivalov/ai-vector-embedding/backend/core/internal/services/embedding"
	"github.com/evgeniyprivalov/ai-vector-embedding/backend/core/internal/services/search"
	server2 "github.com/evgeniyprivalov/ai-vector-embedding/backend/core/pkg/grpc"
	"github.com/evgeniyprivalov/ai-vector-embedding/backend/core/pkg/servers/healthserver"
)

const (
	healthReadTimeout = 10 * time.Second
)

var signalError = errors.New("signal caught")

func main() {
	g, ctx := errgroup.WithContext(context.Background())

	var cfg config.Config
	//nolint:forbidigo
	if err := envconfig.Process("", &cfg); err != nil {
		fmt.Println(fmt.Errorf("can not parse configs: %w", err))
		os.Exit(1) //nolint:gocritic
	}

	logger := log.New(
		log.WithEnvironment(cfg.Environment),
		log.WithVersion(cfg.Version),
		log.WithLevel(log.DebugLevel),
		log.WithCtxAttribute("x-request-id"),
	)

	// Metrics
	mp, err := metric.NewMetricProvider(ctx)
	if err != nil {
		logger.Error("can not init metric provider", "error", err)
		os.Exit(1)
	}

	// tracing
	tp, err := trace.NewProvider(ctx)
	if err != nil {
		logger.Error("can not init trace provider", "error", err)
		os.Exit(1)
	}

	db, err := pg.NewClient(
		ctx,
		pg.WithDSN(cfg.PostgreSQLDSN),
	)
	if err != nil {
		logger.Error("can not init postgresql", "error", err)
		os.Exit(1)
	}

	// GRPC Clients
	chunkerServiceConn, err := server2.InitGRPCClient(cfg.ChunkerHost, cfg.ChunkerWithSecure)
	if err != nil {
		logger.Fatal("can not init grpc client for accounts")
		os.Exit(1)
	}
	chunkerService := chunker.NewChunkerServiceClient(chunkerServiceConn)

	// Repositories
	documentRepository := repository.NewDocumentsRepository(db)

	// Services
	aiClient := newOpenAIClient(cfg)
	embeddingService := embedding.NewEmbeddingService(aiClient, "nomic-embed-text")

	documentHandler := document_handler.NewDocumentHandler(
		logger,
		embeddingService,
		chunkerService,
		documentRepository,
	)

	searchService := search.NewSearchService(
		embeddingService,
		nil,
		logger,
		documentRepository,
		aiClient,
	)

	// Set up main server
	router := mux.NewRouter()

	// REST
	ss := api.StrictServer{
		//
		// Documents
		//
		// POST /v1/documents/upload
		DocumentUploadHandler: documentHandler.DocumentUpload(),
		//
		// Search
		// GET /v1/search
		SearchHandler: searchService.SearchHandler(),
	}
	server.NewHTTPServer(
		router,
		ss,
		&server.Dependencies{
			Logger:      logger,
			ServiceName: cfg.ServiceName,
		},
	)

	// SSE server
	sseRouter := router.PathPrefix("/stream/v1").Name("sse.v1").Subrouter()
	sse := &SSEServer{
		ask: searchService.Ask,
	}
	api2.HandlerFromMux(sse, sseRouter)

	// Main server
	mainServer := server.NewServer(router, cfg.ServerHTTPAddr, cfg.AllowedOrigins)

	// Start the server
	g.Go(func() error {
		logger.Info(fmt.Sprintf("starting main HTTP server on %s", cfg.ServerHTTPAddr))
		if err := mainServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			logger.Error(fmt.Sprintf("main server error: %v", err))
			return err
		}
		return nil
	})

	healthServer := healthserver.MakeHealthServer(cfg.HealthServerAddr, cfg.Environment, healthReadTimeout, logger, db, nil)

	// Start the health check server
	g.Go(func() error {
		logger.Info("Starting health check HTTP server", "addr", healthServer.Addr)

		return healthServer.ListenAndServe()
	})

	g.Go(func() error {
		<-ctx.Done()

		_ = healthServer.Shutdown(ctx)
		_ = mainServer.Shutdown(ctx)
		_ = mp.Shutdown(ctx)
		_ = tp.Shutdown(ctx)
		return nil
	})

	g.Go(func() error {
		// Set up signal handling
		signalCh := make(chan os.Signal, 1)
		signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)

		select {
		case <-signalCh:
			return signalError
		case <-ctx.Done():
			return nil

		}
	})

	// Wait for all goroutines to complete
	if err := g.Wait(); err != nil {
		if errors.Is(err, signalError) {
			logger.Info("got signal to shutdown")
		} else {
			logger.Fatal(fmt.Sprintf("server error: %v", err))
			os.Exit(1)
		}
	}

	logger.Info("all servers gracefully stopped")
}

func newOpenAIClient(cfg config.Config) *openai.Client {
	aiClientConfig := openai.DefaultConfig(cfg.AiAPIKey)
	aiClientConfig.BaseURL = fmt.Sprintf("%s/v1", cfg.AiHost)

	return openai.NewClientWithConfig(aiClientConfig)
}

type SSEServer struct {
	ask func(http.ResponseWriter, *http.Request, api2.AskParams)
}

func (s SSEServer) Ask(
	w http.ResponseWriter,
	r *http.Request,
	params api2.AskParams,
) {
	s.ask(w, r, params)
}
