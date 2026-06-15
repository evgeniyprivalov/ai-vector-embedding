package search

import (
	"context"

	models "ai-vector-embedding/internal/models"
	index_manager_dto "ai-vector-embedding/internal/services/index_manager/dto"
)

const (
	topResults = 3
)

type embeddingService interface {
	Embedding(ctx context.Context, text string) ([]float32, error)
}

type documentsRepository interface {
	Search(ctx context.Context, query []float32, limit uint64) ([]models.Document, error)
}

type indexManagerService interface {
	GetVectorIndex() *index_manager_dto.VectorIndex
}
