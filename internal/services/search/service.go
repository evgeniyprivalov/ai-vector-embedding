package search

import (
	"context"
	"math"
	"sort"

	log "gitlab.com/evgeniyprivalov/golib/observability/log"

	dto "ai-vector-embedding/internal/services/search/dto"
)

type SearchService struct {
	embeddingService    embeddingService
	indexManagerService indexManagerService
	logger              *log.Logger
	documentsRepository documentsRepository
}

func NewSearchService(
	embeddingService embeddingService,
	indexManagerService indexManagerService,
	logger *log.Logger,
	documentsRepository documentsRepository,
) *SearchService {
	return &SearchService{
		embeddingService:    embeddingService,
		indexManagerService: indexManagerService,
		logger:              logger,
		documentsRepository: documentsRepository,
	}
}

func (svc *SearchService) Search(ctx context.Context, query string) ([]dto.SearchResult, error) {
	svc.logger.WithCtx(ctx).Info("Looking for cosine similarity for query", "query", query)

	queryEmbedding, err := svc.embeddingService.Embedding(ctx, query)
	if err != nil {
		return nil, err
	}

	vectorIndex := svc.indexManagerService.GetVectorIndex()

	results := make([]dto.SearchResult, 0, len(vectorIndex.Documents))

	for _, document := range vectorIndex.Documents {
		results = append(results, dto.SearchResult{
			Text:  document.Text,
			Score: svc.cosineSimilarity(document.Embedding, queryEmbedding),
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > topResults {
		results = results[:topResults]
	}

	return results, nil
}

func (svc *SearchService) cosineSimilarity(a, b []float32) float32 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}

	var dot, normA, normB float32
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dot / float32(math.Sqrt(float64(normA))*math.Sqrt(float64(normB)))
}

func (svc *SearchService) SearchDocuments(ctx context.Context, query string) ([]dto.SearchResult, error) {
	svc.logger.WithCtx(ctx).Info("Looking for cosine similarity for query", "query", query)

	queryEmbedding, err := svc.embeddingService.Embedding(ctx, query)
	if err != nil {
		return nil, err
	}

	documents, err := svc.documentsRepository.Search(ctx, queryEmbedding, topResults)
	if err != nil {
		return nil, err
	}

	results := make([]dto.SearchResult, 0, len(documents))
	for _, document := range documents {
		results = append(results, dto.SearchResult{
			Text:  document.Content,
			Score: svc.cosineSimilarity(document.Embedding.Slice(), queryEmbedding),
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results, nil
}
