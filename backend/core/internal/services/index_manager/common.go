package index_manager

import "context"

const (
	maxDocumentSize = 10 * 1024 * 1024
	buffSize        = 1024 * 1024
)

type embeddingService interface {
	Embedding(ctx context.Context, text string) ([]float32, error)
}
