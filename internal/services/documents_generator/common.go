package documents_generator

import (
	"context"

	models "ai-vector-embedding/internal/models"
)

type embeddingService interface {
	Embedding(ctx context.Context, text string) ([]float32, error)
}

type documentsRepository interface {
	Create(ctx context.Context, data *models.Document) (int64, error)
}
