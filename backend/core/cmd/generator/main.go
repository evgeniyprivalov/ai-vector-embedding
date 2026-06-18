package main

import (
	"context"
	"fmt"
	"os"

	_ "github.com/joho/godotenv/autoload"
	"github.com/kelseyhightower/envconfig"
	"github.com/sashabaranov/go-openai"
	"gitlab.com/evgeniyprivalov/golib/observability/log"
	"gitlab.com/evgeniyprivalov/golib/pg"

	"github.com/evgeniyprivalov/ai-vector-embedding/backend/core/internal/config"
	"github.com/evgeniyprivalov/ai-vector-embedding/backend/core/internal/repository"
	"github.com/evgeniyprivalov/ai-vector-embedding/backend/core/internal/services/documents_generator"
	"github.com/evgeniyprivalov/ai-vector-embedding/backend/core/internal/services/embedding"
)

func main() {
	var cfg config.Config
	if err := envconfig.Process("", &cfg); err != nil {
		fmt.Println(fmt.Errorf("can not parse configs: %w", err))
		os.Exit(1)
	}

	ctx := context.Background()

	logger := log.New()
	db, err := pg.NewClient(
		ctx,
		pg.WithDSN(cfg.PostgreSQLDSN),
	)
	if err != nil {
		logger.Error("can not init postgresql", "error", err)
		os.Exit(1)
	}

	// Repositories
	documentsRepository := repository.NewDocumentsRepository(db)

	// Services
	openAIClient := newOpenAIClient(cfg)
	embeddingService := embedding.NewEmbeddingService(openAIClient, "nomic-embed-text")

	documentsGeneratorService := documents_generator.NewDocumentsGenerator(
		embeddingService,
		documentsRepository,
		openAIClient,
		logger,
	)
	err = documentsGeneratorService.GenerateDocumentsToDatabase(1_000)
	if err != nil {
		logger.WithCtx(ctx).Error("can not generate documents to database", "error", err)
		os.Exit(1)
	}
}

func newOpenAIClient(cfg config.Config) *openai.Client {
	aiClientConfig := openai.DefaultConfig(cfg.AiAPIKey)
	aiClientConfig.BaseURL = "http://localhost:11434/v1"

	return openai.NewClientWithConfig(aiClientConfig)
}
