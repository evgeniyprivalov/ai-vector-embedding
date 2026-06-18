package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	_ "github.com/joho/godotenv/autoload"
	"github.com/kelseyhightower/envconfig"
	openai "github.com/sashabaranov/go-openai"
	log "gitlab.com/evgeniyprivalov/golib/observability/log"
	"gitlab.com/evgeniyprivalov/golib/pg"

	"github.com/evgeniyprivalov/ai-vector-embedding/backend/core/internal/config"
	"github.com/evgeniyprivalov/ai-vector-embedding/backend/core/internal/repository"
	"github.com/evgeniyprivalov/ai-vector-embedding/backend/core/internal/services/embedding"
	"github.com/evgeniyprivalov/ai-vector-embedding/backend/core/internal/services/index_manager"
	search2 "github.com/evgeniyprivalov/ai-vector-embedding/backend/core/internal/services/search"
)

func main() {
	os.Exit(run())
}

func run() int {
	var cfg config.Config
	if err := envconfig.Process("", &cfg); err != nil {
		fmt.Println(fmt.Errorf("can not parse configs: %w", err))
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan

		fmt.Println("\r- Ctrl+C pressed in Terminal")

		cancel()

		<-sigChan

		fmt.Println("\nForce exit")
		os.Exit(1)
	}()

	logFile, err := os.OpenFile(
		"app.log",
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
		0644,
	)
	if err != nil {
		fmt.Println(fmt.Errorf("can not open log file: %w", err))
		os.Exit(1)
	}
	defer func() {
		_ = logFile.Close()
	}()

	logger := log.New(
		log.WithLevel(log.InfoLevel),
		log.WithOutput(logFile),
	)
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
	openAIClient := newOpenAIClient(cfg)

	// Services
	embeddingService := embedding.NewEmbeddingService(openAIClient, "nomic-embed-text")
	indexManagerService := index_manager.NewIndexManager(embeddingService, logger, "index.json", "documents.txt")
	if err = indexManagerService.AutoSync(ctx); err != nil {
		logger.WithCtx(ctx).Error("Failed to sync index", "error", err)
		os.Exit(1)
	}

	searchService := search2.NewSearchService(
		embeddingService,
		indexManagerService,
		logger,
		documentsRepository,
		openAIClient,
	)

	fmt.Print("> ")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return 0
		default:
		}

		userInput := strings.TrimSpace(scanner.Text())
		if userInput == "" {
			continue
		}

		if userInput == "exit" {
			logger.WithCtx(ctx).Info("Bye!")
			return 0
		}

		searchResults, err := searchService.Search(ctx, userInput)
		if err != nil {
			logger.WithCtx(ctx).Error("Failed to search", "error", err)
			return 1
		}

		fmt.Println("-------- Search results by index --------")
		for idx, searchResult := range searchResults {
			fmt.Printf("%d. [%.1f] %s\n", idx+1, searchResult.Score, searchResult.Text)
		}

		searchResultsByDB, err := searchService.SearchDocuments(ctx, userInput)
		if err != nil {
			logger.WithCtx(ctx).Error("Failed to search in database", "error", err)
			return 1
		}
		fmt.Println("\n-------- Search results by database --------")
		for idx, searchResult := range searchResultsByDB {
			fmt.Printf("%d. [%.1f] %s\n", idx+1, searchResult.Score, searchResult.Text)
		}
		fmt.Println()
		fmt.Print("> ")
	}

	if err := scanner.Err(); err != nil {
		logger.WithCtx(ctx).Error("reading standard input", "error", err)
		return 1
	}

	return 0
}

func newOpenAIClient(cfg config.Config) *openai.Client {
	aiClientConfig := openai.DefaultConfig(cfg.AiAPIKey)
	aiClientConfig.BaseURL = "http://localhost:11434/v1"

	return openai.NewClientWithConfig(aiClientConfig)
}
