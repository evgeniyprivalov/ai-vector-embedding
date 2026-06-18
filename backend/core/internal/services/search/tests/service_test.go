package tests

import (
	"context"
	"testing"

	_ "github.com/lib/pq"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	log "gitlab.com/evgeniyprivalov/golib/observability/log"
	assert "gotest.tools/v3/assert"

	root "github.com/evgeniyprivalov/ai-vector-embedding/backend/core"
	api "github.com/evgeniyprivalov/ai-vector-embedding/backend/core/internal/contracts/public/v1/openapi"
	embedding "github.com/evgeniyprivalov/ai-vector-embedding/backend/core/internal/mocks/embedding"
	"github.com/evgeniyprivalov/ai-vector-embedding/backend/core/internal/mocks/index_manager"
	"github.com/evgeniyprivalov/ai-vector-embedding/backend/core/internal/repository"
	"github.com/evgeniyprivalov/ai-vector-embedding/backend/core/internal/services/search"
	"github.com/evgeniyprivalov/ai-vector-embedding/backend/core/pkg"
)

const content = `Exploring Space Space is the vast expanse that exists beyond Earth's atmosphere. It contains planets, moons, stars, galaxies, nebulae, black holes, and countless other objects. Modern astronomy uses telescopes and spacecraft to study the universe and understand its origins. Our solar system consists of the Sun and the objects that orbit it, including eight planets. Earth is the only known world that supports life. Scientists continue to search for signs of life elsewhere, especially on Mars and on the icy moons of the outer planets. Space exploration has expanded human knowledge dramatically. Missions such as lunar landings, space telescopes, and robotic probes have revealed new details about the cosmos and inspired future generations of scientists and engineers. The Future of Space Exploration The future of space exploration includes plans for long-term human missions to the Moon and Mars. Space agencies and private companies are developing new rockets, habitats, and technologies to support deep-space travel. Astronomers are also building more powerful observatories to study distant galaxies and investigate fundamental questions about dark matter, dark energy, and the evolution of the universe. Future discoveries may transform our understanding of physics and our place in the cosmos. As technology advances, humanity may eventually establish permanent settlements beyond Earth, opening a new era of exploration and scientific achievement`

func makeMockedEmbedding() []float32 {
	res := make([]float32, 768)
	for i := range res {
		res[i] = 0.01
	}

	return res
}

func TestSearchHandler__Success(t *testing.T) {
	dbVector := pkg.PgVectorNew(t, &root.MigationsOptions{})
	dbVector.SetFixtures("fixtures")

	logger := log.New()
	ctx := context.Background()

	documentsRepository := repository.NewDocumentsRepository(dbVector.Pool())

	embeddingService := embedding.NewMockEmbeddingService(t)
	embeddingService.EXPECT().Embedding(ctx, mock.Anything).Return(makeMockedEmbedding(), nil)

	indexManagerService := index_manager.NewMockIndexManager(t)

	svc := search.NewSearchService(
		embeddingService,
		indexManagerService,
		logger,
		documentsRepository,
		nil,
	)

	response, err := svc.SearchHandler()(ctx, api.SearchRequestObject{
		Params: api.SearchParams{
			Query: "space",
		},
	})
	assert.NilError(t, err)
	require.IsType(t, api.Search200JSONResponse{}, response)

	assert.DeepEqual(t, api.Search200JSONResponse{
		Content:          content,
		CosineSimilarity: 0.006058302707970142,
	}, response)
}

func TestSearchHandler__EmbeddingError(t *testing.T) {
	dbVector := pkg.PgVectorNew(t, &root.MigationsOptions{})
	logger := log.New()
	ctx := context.Background()

	documentsRepository := repository.NewDocumentsRepository(dbVector.Pool())

	embeddingService := embedding.NewMockEmbeddingService(t)
	embeddingService.EXPECT().Embedding(ctx, mock.Anything).Return(nil, errors.New("some error"))

	indexManagerService := index_manager.NewMockIndexManager(t)

	svc := search.NewSearchService(
		embeddingService,
		indexManagerService,
		logger,
		documentsRepository,
		nil,
	)

	response, err := svc.SearchHandler()(ctx, api.SearchRequestObject{
		Params: api.SearchParams{
			Query: "space",
		},
	})
	require.Error(t, err)
	require.IsType(t, api.Search500JSONResponse{}, response)
	assert.DeepEqual(t, api.Search500JSONResponse{
		Message: "Can not make embedding for query",
	}, response)
}

func TestSearchHandler__NoDocuments(t *testing.T) {
	dbVector := pkg.PgVectorNew(t, &root.MigationsOptions{})
	logger := log.New()
	ctx := context.Background()

	documentsRepository := repository.NewDocumentsRepository(dbVector.Pool())

	embeddingService := embedding.NewMockEmbeddingService(t)
	embeddingService.EXPECT().Embedding(ctx, mock.Anything).Return(makeMockedEmbedding(), nil)

	indexManagerService := index_manager.NewMockIndexManager(t)

	svc := search.NewSearchService(
		embeddingService,
		indexManagerService,
		logger,
		documentsRepository,
		nil,
	)

	response, err := svc.SearchHandler()(ctx, api.SearchRequestObject{
		Params: api.SearchParams{
			Query: "%@^*#%$*@^$%*@#^$%@#%$@&^*#%$",
		},
	})
	require.Empty(t, err)
	require.IsType(t, api.Search404JSONResponse{}, response)
	assert.DeepEqual(t, api.Search404JSONResponse{
		Message: "Data not found",
	}, response)
}
