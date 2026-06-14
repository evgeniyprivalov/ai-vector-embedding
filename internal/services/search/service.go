package search

import (
	"context"
	"math"
	"sort"

	log "gitlab.com/evgeniyprivalov/golib/observability/log"

	index_manager_dto "ai-vector-embedding/internal/services/index_manager/dto"
	dto "ai-vector-embedding/internal/services/search/dto"
)

type SearchService struct {
	embeddingService embeddingService
	vectorIndex      *index_manager_dto.VectorIndex
	logger           *log.Logger
}

func NewSearchService(
	embeddingService embeddingService,
	vectorIndex *index_manager_dto.VectorIndex,
	logger *log.Logger,
) *SearchService {
	return &SearchService{
		embeddingService: embeddingService,
		vectorIndex:      vectorIndex,
		logger:           logger,
	}
}

func (svc *SearchService) Search(ctx context.Context, query string) ([]dto.SearchResult, error) {
	svc.logger.WithCtx(ctx).Info("Looking for cosine similarity for query", "query", query)

	queryEmbedding, err := svc.embeddingService.Embedding(ctx, query)
	if err != nil {
		return nil, err
	}

	results := make([]dto.SearchResult, 0, len(svc.vectorIndex.Documents))

	for _, document := range svc.vectorIndex.Documents {
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
