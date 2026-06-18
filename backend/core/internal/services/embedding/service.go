package embedding

import (
	"context"
	"errors"
	"fmt"

	"github.com/sashabaranov/go-openai"
)

type EmbeddingService interface {
	Embedding(ctx context.Context, text string) ([]float32, error)
}

type embeddingService struct {
	client         *openai.Client
	embeddingModel string
}

func NewEmbeddingService(
	client *openai.Client,
	embeddingModel string,
) EmbeddingService {
	return &embeddingService{
		client:         client,
		embeddingModel: embeddingModel,
	}
}

func (svc *embeddingService) Embedding(ctx context.Context, text string) ([]float32, error) {
	resp, err := svc.client.CreateEmbeddings(ctx, openai.EmbeddingRequest{
		Model: openai.EmbeddingModel(svc.embeddingModel),
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
