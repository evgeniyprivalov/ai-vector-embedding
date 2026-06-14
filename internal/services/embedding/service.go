package embedding

import (
	"context"
	"errors"
	"fmt"

	"github.com/sashabaranov/go-openai"
)

const (
	embeddingModel = "nomic-embed-text"
)

type EmbeddingService struct {
	client *openai.Client
}

func NewEmbeddingService(
	client *openai.Client,
) *EmbeddingService {
	return &EmbeddingService{
		client: client,
	}
}

func (svc *EmbeddingService) Embedding(ctx context.Context, text string) ([]float32, error) {
	resp, err := svc.client.CreateEmbeddings(ctx, openai.EmbeddingRequest{
		Model: embeddingModel,
		Input: text,
	})
	if err != nil {
		return nil, fmt.Errorf("create embedding: %w", err)
	}

	if len(resp.Data) == 0 {
		return nil, errors.New("empty embedding response")
	}

	return resp.Data[0].Embedding, nil
}
