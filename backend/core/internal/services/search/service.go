package search

import (
	"context"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/sashabaranov/go-openai"
	log "gitlab.com/evgeniyprivalov/golib/observability/log"

	api2 "github.com/evgeniyprivalov/ai-vector-embedding/backend/core/internal/contracts/public/sse/v1/openapi"
	api "github.com/evgeniyprivalov/ai-vector-embedding/backend/core/internal/contracts/public/v1/openapi"
	"github.com/evgeniyprivalov/ai-vector-embedding/backend/core/internal/services/search/dto"
	"github.com/evgeniyprivalov/ai-vector-embedding/backend/core/pkg/transforms/slice"
)

var ErrOllamaStreamFinished = errors.New("stream finished")

type SearchService interface {
	Search(ctx context.Context, query string) ([]dto.SearchResult, error)
	SearchDocuments(ctx context.Context, query string) ([]dto.SearchResult, error)
	SearchHandler() api.SearchHandlerFunc
	Ask(w http.ResponseWriter, r *http.Request, params api2.AskParams)
}

type searchService struct {
	embeddingService    embeddingService
	indexManagerService indexManagerService
	logger              *log.Logger
	documentsRepository documentsRepository
	aiService           *openai.Client
}

func NewSearchService(
	embeddingService embeddingService,
	indexManagerService indexManagerService,
	logger *log.Logger,
	documentsRepository documentsRepository,
	aiService *openai.Client,
) SearchService {
	return &searchService{
		embeddingService:    embeddingService,
		indexManagerService: indexManagerService,
		logger:              logger,
		documentsRepository: documentsRepository,
		aiService:           aiService,
	}
}

func (svc *searchService) Search(ctx context.Context, query string) ([]dto.SearchResult, error) {
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
			Score: slice.CosineSimilarity(document.Embedding, queryEmbedding),
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

func (svc *searchService) SearchDocuments(ctx context.Context, query string) ([]dto.SearchResult, error) {
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
			Score: slice.CosineSimilarity(document.Embedding.Slice(), queryEmbedding),
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results, nil
}

func (svc *searchService) SearchHandler() api.SearchHandlerFunc {
	return func(ctx context.Context, request api.SearchRequestObject) (api.SearchResponseObject, error) {
		svc.logger.WithCtx(ctx).Info("looking for cosine similarity for query")

		queryEmbedding, err := svc.embeddingService.Embedding(ctx, request.Params.Query)
		if err != nil {
			return api.Search500JSONResponse{
				Message: "Can not make embedding for query",
			}, err
		}

		documents, err := svc.documentsRepository.Search(ctx, queryEmbedding, 1)
		if err != nil {
			return api.Search500JSONResponse{
				Message: "Cannot search documents",
			}, err
		}
		if len(documents) == 0 {
			return api.Search404JSONResponse{
				Message: "Data not found",
			}, nil
		}

		return api.Search200JSONResponse{
			Content:          documents[0].Content,
			CosineSimilarity: float64(slice.CosineSimilarity(documents[0].Embedding.Slice(), queryEmbedding)),
		}, nil
	}
}

func (svc *searchService) Ask(
	w http.ResponseWriter,
	r *http.Request,
	params api2.AskParams,
) {
	ctx := r.Context()

	if strings.TrimSpace(params.Query) == "" {
		http.Error(w, "query is empty", http.StatusBadRequest)

		return
	}

	queryEmbedding, err := svc.embeddingService.Embedding(
		ctx,
		params.Query,
	)
	if err != nil {
		svc.logger.WithCtx(ctx).Error(
			"failed to generate embedding",
			"error",
			err,
		)

		http.Error(
			w,
			"failed to generate embedding",
			http.StatusInternalServerError,
		)

		return
	}

	documents, err := svc.documentsRepository.Search(
		ctx,
		queryEmbedding,
		topResults,
	)
	if err != nil {
		svc.logger.WithCtx(ctx).Error(
			"failed to search documents",
			"error",
			err,
		)

		http.Error(
			w,
			"failed to search documents",
			http.StatusInternalServerError,
		)

		return
	}

	var userContent strings.Builder

	userContent.WriteString("DOCUMENTS:\n")
	for idx, document := range documents {
		userContent.WriteString("[Document ")
		userContent.WriteString(strconv.Itoa(idx + 1))
		userContent.WriteString("]\nID: ")
		userContent.WriteString(strconv.FormatInt(document.ID, 10))
		userContent.WriteString("\nContent: ")
		userContent.WriteString(document.Content)
		userContent.WriteString("\n\n")
	}

	userContent.WriteString("QUESTION:\n")
	userContent.WriteString(params.Query)

	messages := []openai.ChatCompletionMessage{
		{
			Role: openai.ChatMessageRoleSystem,
			Content: `You are a retrieval-based question answering system.

You will receive:
* A set of retrieved documents.
* A user question.

Your responsibilities:

* Answer strictly from the retrieved documents.
* Treat the documents as the only source of truth.
* Do not use any information that is not present in the documents.
* Do not infer facts that are not directly supported by the documents.
* If the answer cannot be supported by the documents, respond exactly:

Insufficient information.

* If multiple documents support the answer, synthesize them.
* If documents disagree, state the disagreement.
* Quote short excerpts when useful.
* Prefer accuracy over completeness.
* Never hallucinate.

Before answering, internally verify that every factual claim in your response is supported by at least one provided document.

Output only the final answer.`,
		},
		{
			Role:    openai.ChatMessageRoleUser,
			Content: userContent.String(),
		},
	}

	stream, err := svc.aiService.CreateChatCompletionStream(
		ctx,
		openai.ChatCompletionRequest{
			Model:    "llama3.1:8b",
			Messages: messages,
			Stream:   true,
		},
	)
	if err != nil {
		svc.logger.WithCtx(ctx).Error(
			"failed to create completion stream",
			"error",
			err,
		)

		http.Error(
			w,
			"failed to create completion stream",
			http.StatusInternalServerError,
		)

		return
	}

	defer func() {
		if closeErr := stream.Close(); closeErr != nil {
			svc.logger.WithCtx(ctx).Error(
				"failed to close completion stream",
				"error",
				closeErr,
			)
		}
	}()

	sse, err := api2.NewSSEWriter(w)
	if err != nil {
		svc.logger.WithCtx(ctx).Error(
			"failed to create sse writer",
			"error",
			err,
		)

		http.Error(
			w,
			"streaming unsupported",
			http.StatusInternalServerError,
		)

		return
	}

	w.WriteHeader(http.StatusOK)

	sendSSEError := func(message string) {
		if err := sse.SendEvent(
			"error",
			map[string]string{
				"message": message,
			},
		); err != nil {
			svc.logger.WithCtx(ctx).Error(
				"failed to send sse error",
				"error",
				err,
			)
		}
	}

	for {
		select {
		case <-ctx.Done():
			svc.logger.WithCtx(ctx).Info(
				"client disconnected",
			)

			return

		default:
		}

		chunk, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) ||
				errors.Is(err, ErrOllamaStreamFinished) {

				break
			}

			svc.logger.WithCtx(ctx).Error(
				"failed to receive stream chunk",
				"error",
				err,
			)

			sendSSEError("stream interrupted")

			return
		}

		for _, choice := range chunk.Choices {
			token := choice.Delta.Content
			if token == "" {
				continue
			}

			if err := sse.SendEvent(
				"token",
				map[string]string{
					"token": token,
				},
			); err != nil {
				svc.logger.WithCtx(ctx).Error(
					"failed to send token",
					"error",
					err,
				)

				return
			}
		}
	}

	if err := sse.SendEvent(
		"done",
		map[string]bool{
			"done": true,
		},
	); err != nil {
		svc.logger.WithCtx(ctx).Error(
			"failed to send done event",
			"error",
			err,
		)

		return
	}

	if err := sse.Done(); err != nil {
		svc.logger.WithCtx(ctx).Error(
			"failed to finalize stream",
			"error",
			err,
		)
	}
}
