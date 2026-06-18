package tests

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/textproto"
	"testing"

	_ "github.com/lib/pq"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	log "gitlab.com/evgeniyprivalov/golib/observability/log"
	"google.golang.org/grpc/metadata"
	assert "gotest.tools/v3/assert"

	root "github.com/evgeniyprivalov/ai-vector-embedding/backend/core"
	chunker "github.com/evgeniyprivalov/ai-vector-embedding/backend/core/internal/contracts/chunker/v1"
	api "github.com/evgeniyprivalov/ai-vector-embedding/backend/core/internal/contracts/public/v1/openapi"
	chunkerMock "github.com/evgeniyprivalov/ai-vector-embedding/backend/core/internal/mocks/contracts/chunker"
	embedding "github.com/evgeniyprivalov/ai-vector-embedding/backend/core/internal/mocks/embedding"
	"github.com/evgeniyprivalov/ai-vector-embedding/backend/core/internal/repository"
	document_handler "github.com/evgeniyprivalov/ai-vector-embedding/backend/core/internal/services/document_handler"
	"github.com/evgeniyprivalov/ai-vector-embedding/backend/core/pkg"
)

func makeMockedEmbedding() []float32 {
	res := make([]float32, 768)

	for i := range res {
		res[i] = 0.01
	}

	return res
}

func makeMultipartReader(
	t *testing.T,
	fileName string,
	content []byte,
) *multipart.Reader {
	t.Helper()

	var body bytes.Buffer

	writer := multipart.NewWriter(&body)

	header := make(textproto.MIMEHeader)
	header.Set(
		"Content-Disposition",
		`form-data; name="document"; filename="`+fileName+`"`,
	)
	header.Set("Content-Type", "text/plain")

	part, err := writer.CreatePart(header)
	require.NoError(t, err)

	_, err = part.Write(content)
	require.NoError(t, err)

	require.NoError(t, writer.Close())

	return multipart.NewReader(
		bytes.NewReader(body.Bytes()),
		writer.Boundary(),
	)
}

type fakeChunkStream struct {
	responses []*chunker.ChunkResponse
	index     int
}

func (f *fakeChunkStream) Send(*chunker.ChunkRequest) error {
	return nil
}

func (f *fakeChunkStream) CloseSend() error {
	return nil
}

func (f *fakeChunkStream) Recv() (*chunker.ChunkResponse, error) {
	if f.index >= len(f.responses) {
		return nil, io.EOF
	}

	resp := f.responses[f.index]
	f.index++

	return resp, nil
}

// grpc.BidiStreamingClient implementation

func (f *fakeChunkStream) Header() (metadata.MD, error) {
	return metadata.MD{}, nil
}

func (f *fakeChunkStream) Trailer() metadata.MD {
	return metadata.MD{}
}

func (f *fakeChunkStream) Context() context.Context {
	return context.Background()
}

func (f *fakeChunkStream) SendMsg(any) error {
	return nil
}

func (f *fakeChunkStream) RecvMsg(any) error {
	return nil
}

func TestDocumentUpload__Success(t *testing.T) {
	dbVector := pkg.PgVectorNew(t, &root.MigationsOptions{})
	dbVector.SetFixtures("fixtures")

	ctx := context.Background()
	logger := log.New()

	embeddingService := embedding.NewMockEmbeddingService(t)
	documentsRepository := repository.NewDocumentsRepository(dbVector.Pool())
	chunkerService := chunkerMock.NewMockChunkerServiceClient(t)

	stream := &fakeChunkStream{
		responses: []*chunker.ChunkResponse{
			{
				Content: "test chunk",
			},
		},
	}

	chunkerService.EXPECT().
		ExtractChunks(mock.Anything, mock.Anything).
		Return(stream, nil)

	embeddingService.EXPECT().
		Embedding(mock.Anything, "test chunk").
		Return(makeMockedEmbedding(), nil)

	//documentsRepository.EXPECT().
	//	Create(mock.Anything, mock.AnythingOfType("*models.Document")).
	//	Return(int64(1), nil)

	svc := document_handler.NewDocumentHandler(
		logger,
		embeddingService,
		chunkerService,
		documentsRepository,
	)

	form := makeMultipartReader(
		t,
		"test.txt",
		[]byte("hello world"),
	)

	response, err := svc.DocumentUpload()(ctx, api.DocumentUploadRequestObject{
		Body: form,
	})

	require.NoError(t, err)

	require.IsType(
		t,
		api.DocumentUpload200JSONResponse{},
		response,
	)

	assert.DeepEqual(
		t,
		api.DocumentUpload200JSONResponse{
			FileName:     "test.txt",
			ChunksAmount: 1,
		},
		response,
	)
}

func TestDocumentUpload__EmbeddingError(t *testing.T) {
	dbVector := pkg.PgVectorNew(t, &root.MigationsOptions{})
	dbVector.SetFixtures("fixtures")

	ctx := context.Background()
	logger := log.New()

	embeddingService := embedding.NewMockEmbeddingService(t)
	documentsRepository := repository.NewDocumentsRepository(dbVector.Pool())
	chunkerService := chunkerMock.NewMockChunkerServiceClient(t)

	stream := &fakeChunkStream{
		responses: []*chunker.ChunkResponse{
			{
				Content: "test chunk",
			},
		},
	}

	chunkerService.EXPECT().
		ExtractChunks(mock.Anything, mock.Anything).
		Return(stream, nil)

	embeddingService.EXPECT().
		Embedding(mock.Anything, "test chunk").
		Return(nil, errors.New("embedding error"))

	svc := document_handler.NewDocumentHandler(
		logger,
		embeddingService,
		chunkerService,
		documentsRepository,
	)

	form := makeMultipartReader(
		t,
		"test.txt",
		[]byte("hello world"),
	)

	response, err := svc.DocumentUpload()(ctx, api.DocumentUploadRequestObject{
		Body: form,
	})

	require.Error(t, err)

	require.IsType(
		t,
		api.DocumentUpload500JSONResponse{},
		response,
	)

	assert.DeepEqual(
		t,
		api.DocumentUpload500JSONResponse{
			Message: "Can not process embedding for chunk",
		},
		response,
	)
}

//
//func TestDocumentUpload__CreateDocumentError(t *testing.T) {
//	dbVector := pkg.PgVectorNew(t, &root.MigationsOptions{})
//	dbVector.SetFixtures("fixtures")
//
//	ctx := context.Background()
//	logger := log.New()
//
//	embeddingService := embedding.NewMockEmbeddingService(t)
//	documentsRepository := repository.NewDocumentsRepository(dbVector.Pool())
//	chunkerService := chunkerMock.NewMockChunkerServiceClient(t)
//
//	stream := &fakeChunkStream{
//		responses: []*chunker.ChunkResponse{
//			{
//				Content: "test chunk",
//			},
//		},
//	}
//
//	chunkerService.EXPECT().
//		ExtractChunks(mock.Anything, mock.Anything).
//		Return(stream, nil)
//
//	embeddingService.EXPECT().
//		Embedding(mock.Anything, "test chunk").
//		Return(makeMockedEmbedding(), nil)
//
//	//documentsRepository.EXPECT().
//	//	Create(mock.Anything, mock.AnythingOfType("*models.Document")).
//	//	Return(int64(0), errors.New("db error"))
//
//	svc := document_handler.NewDocumentHandler(
//		logger,
//		embeddingService,
//		chunkerService,
//		documentsRepository,
//	)
//
//	form := makeMultipartReader(
//		t,
//		"test.txt",
//		[]byte("hello world"),
//	)
//
//	response, err := svc.DocumentUpload()(ctx, api.DocumentUploadRequestObject{
//		Body: form,
//	})
//
//	require.Error(t, err)
//
//	require.IsType(
//		t,
//		api.DocumentUpload500JSONResponse{},
//		response,
//	)
//
//	assert.DeepEqual(
//		t,
//		api.DocumentUpload500JSONResponse{
//			Message: "Can not process chunk",
//		},
//		response,
//	)
//}

func TestDocumentUpload__ChunkerError(t *testing.T) {
	dbVector := pkg.PgVectorNew(t, &root.MigationsOptions{})
	dbVector.SetFixtures("fixtures")

	ctx := context.Background()
	logger := log.New()

	embeddingService := embedding.NewMockEmbeddingService(t)
	documentsRepository := repository.NewDocumentsRepository(dbVector.Pool())
	chunkerService := chunkerMock.NewMockChunkerServiceClient(t)

	chunkerService.EXPECT().
		ExtractChunks(mock.Anything, mock.Anything).
		Return(nil, errors.New("chunker error"))

	svc := document_handler.NewDocumentHandler(
		logger,
		embeddingService,
		chunkerService,
		documentsRepository,
	)

	form := makeMultipartReader(
		t,
		"test.txt",
		[]byte("hello world"),
	)

	response, err := svc.DocumentUpload()(ctx, api.DocumentUploadRequestObject{
		Body: form,
	})

	require.Error(t, err)

	require.IsType(
		t,
		api.DocumentUpload500JSONResponse{},
		response,
	)

	assert.DeepEqual(
		t,
		api.DocumentUpload500JSONResponse{
			Message: "Can not process document",
		},
		response,
	)
}
