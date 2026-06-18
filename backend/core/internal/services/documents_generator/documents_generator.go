package documents_generator

import (
	"context"
	"fmt"

	"github.com/pgvector/pgvector-go"
	"github.com/sashabaranov/go-openai"
	"gitlab.com/evgeniyprivalov/golib/observability/log"

	"github.com/evgeniyprivalov/ai-vector-embedding/backend/core/internal/models"
)

type DocumentsGenerator struct {
	embeddingService    embeddingService
	documentsRepository documentsRepository
	aiClient            *openai.Client
	logger              *log.Logger
}

func NewDocumentsGenerator(
	embeddingService embeddingService,
	documentsRepository documentsRepository,
	client *openai.Client,
	logger *log.Logger,
) *DocumentsGenerator {
	return &DocumentsGenerator{
		embeddingService:    embeddingService,
		documentsRepository: documentsRepository,
		aiClient:            client,
		logger:              logger,
	}
}

func (svc *DocumentsGenerator) GenerateDocumentsToDatabase(n int32) error {
	ctx := context.Background()

	for idx := 0; int32(idx) < n; idx++ {
		i := int32(idx)

		resp, err := svc.aiClient.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
			Model: "llama3.1:8b",
			Messages: []openai.ChatCompletionMessage{
				{
					Role: openai.ChatMessageRoleSystem,
					Content: `Generate a realistic document.

Requirements:
- Length: 100-300 words.
- 1-2 paragraphs.
- No title.
- No markdown.
- Choose topic - Space.
- Include concrete facts, entities, places, people, technologies, products, or events when appropriate.
- Documents should be semantically distinct from each other.
- Avoid generic filler text.
Return only the document.
`,
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: "Generate new random text",
				},
			},
		})
		if err != nil {
			return err
		}

		embeddingResp, err := svc.embeddingService.Embedding(ctx, resp.Choices[0].Message.Content)
		if err != nil {
			return err
		}

		if _, err := svc.documentsRepository.Create(ctx, &models.Document{
			Content:   resp.Choices[0].Message.Content,
			Embedding: pgvector.NewVector(embeddingResp),
		}); err != nil {
			return err
		}

		svc.logger.WithCtx(ctx).Info(fmt.Sprintf(
			"Successfully generated document %d to database. Total remaining = %d", i+1, n-(i+1),
		))
	}

	return nil
}
