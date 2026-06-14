package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	openai "github.com/sashabaranov/go-openai"
	log "gitlab.com/evgeniyprivalov/golib/observability/log"

	embedding "ai-vector-embedding/internal/services/embedding"
	index_manager "ai-vector-embedding/internal/services/index_manager"
	search2 "ai-vector-embedding/internal/services/search"
)

func main() {
	os.Exit(run())
}

func run() int {
	query := flag.String("query", "", "Search query")
	flag.Parse()

	if *query == "" {
		flag.PrintDefaults()
		return 1
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

	logger := log.New()

	cfg := openai.DefaultConfig("ollama")
	cfg.BaseURL = "http://localhost:11434/v1"

	embeddingService := embedding.NewEmbeddingService(
		openai.NewClientWithConfig(cfg),
	)
	indexManagerService := index_manager.NewIndexManager(embeddingService, logger)

	vectorIndex, err := indexManagerService.Sync(ctx, "index.json", "documents.txt")
	if err != nil {
		logger.WithCtx(ctx).Error("Failed to sync index", "error", err)
		os.Exit(1)
	}

	searchService := search2.NewSearchService(embeddingService, vectorIndex, logger)
	searchResults, err := searchService.Search(ctx, *query)
	if err != nil {
		logger.WithCtx(ctx).Error("Failed to search", "error", err)
		return 1
	}

	fmt.Println("Search results:")
	for idx, searchResult := range searchResults {
		fmt.Printf("%d. [%.1f] %s\n", idx+1, searchResult.Score, searchResult.Text)
	}

	return 0
}
