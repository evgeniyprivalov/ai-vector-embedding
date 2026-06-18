package documents_generator

import (
	"context"

	"github.com/evgeniyprivalov/ai-vector-embedding/backend/core/internal/models"
)

type embeddingService interface {
	Embedding(ctx context.Context, text string) ([]float32, error)
}

type documentsRepository interface {
	Create(ctx context.Context, data *models.Document) (int64, error)
}
