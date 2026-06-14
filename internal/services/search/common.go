package search

import "context"

type embeddingService interface {
	Embedding(ctx context.Context, text string) ([]float32, error)
}

const (
	topResults = 3
)
