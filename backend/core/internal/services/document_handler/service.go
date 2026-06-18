package document_handler

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/pgvector/pgvector-go"
	"gitlab.com/evgeniyprivalov/golib/observability/log"
	"google.golang.org/grpc"

	chunker "github.com/evgeniyprivalov/ai-vector-embedding/backend/core/internal/contracts/chunker/v1"
	api "github.com/evgeniyprivalov/ai-vector-embedding/backend/core/internal/contracts/public/v1/openapi"
	models "github.com/evgeniyprivalov/ai-vector-embedding/backend/core/internal/models"
)

const (
	uploadFormSize        = 1 << 28 // 28 MB
	chunkSize             = 1 << 20 // 1 MB
	chunkOverlap          = 1000
	documentUploadTimeout = 30 * time.Second
)

type embeddingService interface {
	Embedding(ctx context.Context, text string) ([]float32, error)
}

type documentsRepository interface {
	Create(ctx context.Context, data *models.Document) (int64, error)
}

type chunkerService interface {
	ExtractChunks(ctx context.Context, opts ...grpc.CallOption) (grpc.BidiStreamingClient[chunker.ChunkRequest, chunker.ChunkResponse], error)
}

type DocumentHandler struct {
	logger              *log.Logger
	embeddingService    embeddingService
	chunkerService      chunkerService
	documentsRepository documentsRepository
}

func NewDocumentHandler(
	logger *log.Logger,
	embeddingService embeddingService,
	chunkerService chunkerService,
	documentsRepository documentsRepository,
) *DocumentHandler {
	return &DocumentHandler{
		logger:              logger,
		embeddingService:    embeddingService,
		chunkerService:      chunkerService,
		documentsRepository: documentsRepository,
	}
}

func (svc *DocumentHandler) DocumentUpload() api.DocumentUploadHandlerFunc {
	return func(ctx context.Context, request api.DocumentUploadRequestObject) (api.DocumentUploadResponseObject, error) {
		form, err := request.Body.ReadForm(uploadFormSize)
		if err != nil {
			return api.DocumentUpload400JSONResponse{
				Message: fmt.Sprintf("failed to read form"),
			}, err
		}
		defer func() {
			err = form.RemoveAll()
			if err != nil {
				svc.logger.WithCtx(ctx).Error("failed to remove all form")
			}
		}()

		documents := form.File["document"]
		if len(documents) == 0 {
			return api.DocumentUpload404JSONResponse{
				Message: fmt.Sprintf("no document found"),
			}, err
		}

		document := documents[0]
		file, err := document.Open()
		if err != nil {
			return api.DocumentUpload500JSONResponse{
				Message: fmt.Sprintf("failed to open file"),
			}, err
		}
		defer func() {
			err = file.Close()
			if err != nil {
				svc.logger.WithCtx(ctx).Error("failed to close file")
			}
		}()

		ctx, cancel := context.WithTimeout(ctx, documentUploadTimeout)
		defer cancel()

		stream, err := svc.chunkerService.ExtractChunks(ctx)
		if err != nil {
			return api.DocumentUpload500JSONResponse{
				Message: "Can not process document",
			}, err
		}

		buffer := make([]byte, chunkSize)
		errChan := make(chan error, 2)
		go func() {
			for {
				n, err := file.Read(buffer)
				if err != nil {
					if err == io.EOF {
						break
					}

					errChan <- err
					return
				}

				err = stream.Send(&chunker.ChunkRequest{
					FileContent:  buffer[:n],
					FileName:     document.Filename,
					ChunkSize:    chunkSize,
					ChunkOverlap: chunkOverlap,
				})
				if err != nil {
					errChan <- err
					return
				}
			}

			if err := stream.CloseSend(); err != nil {
				errChan <- err
				return
			}
		}()

		chunkRespChan := make(chan *chunker.ChunkResponse)
		go func() {
			defer close(chunkRespChan)

			for {
				chunk, err := stream.Recv()
				if err != nil {
					if err == io.EOF {
						break
					}

					errChan <- err
					return
				}

				chunkRespChan <- chunk
			}
		}()

		var chunksAmount int64
		for {
			select {
			case <-ctx.Done():
				return api.DocumentUpload500JSONResponse{
					Message: "Timeout exceeded",
				}, err
			case err := <-errChan:
				if err != nil {
					return api.DocumentUpload500JSONResponse{
						Message: err.Error(),
					}, err
				}
			case chunkResp, ok := <-chunkRespChan:
				if !ok {
					return api.DocumentUpload200JSONResponse{
						FileName:     document.Filename,
						ChunksAmount: chunksAmount,
					}, nil
				}

				chunkEmbeddingResp, err := svc.embeddingService.Embedding(ctx, chunkResp.GetContent())
				if err != nil {
					return api.DocumentUpload500JSONResponse{
						Message: "Can not process embedding for chunk",
					}, err
				}

				_, err = svc.documentsRepository.Create(ctx, &models.Document{
					Content:   chunkResp.GetContent(),
					Embedding: pgvector.NewVector(chunkEmbeddingResp),
				})
				if err != nil {
					return api.DocumentUpload500JSONResponse{
						Message: "Can not process chunk",
					}, err
				}

				chunksAmount++
			}
		}
	}
}
