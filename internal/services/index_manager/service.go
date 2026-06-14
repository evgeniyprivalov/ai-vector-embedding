package index_manager

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	log "gitlab.com/evgeniyprivalov/golib/observability/log"

	dto "ai-vector-embedding/internal/services/index_manager/dto"
	pkg "ai-vector-embedding/pkg"
)

type IndexManager struct {
	embeddingService embeddingService
	logger           *log.Logger
}

func NewIndexManager(
	embeddingService embeddingService,
	logger *log.Logger,
) *IndexManager {
	return &IndexManager{
		embeddingService: embeddingService,
		logger:           logger,
	}
}

func (svc *IndexManager) Sync(
	ctx context.Context,
	indexPath string,
	sourceFile string,
) (*dto.VectorIndex, error) {
	currentIndex, err := svc.loadIndex(indexPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("load index: %w", err)
	}

	if currentIndex == nil {
		currentIndex = &dto.VectorIndex{}
	}

	existing := make(map[string]dto.Document, len(currentIndex.Documents))

	for _, doc := range currentIndex.Documents {
		existing[doc.Hash] = doc
	}

	newIndex := &dto.VectorIndex{
		UpdatedAt: time.Now().UTC(),
		Documents: make([]dto.Document, 0, len(existing)),
	}

	err = svc.streamDocument(
		ctx,
		sourceFile,
		func(text string) error {
			hash := pkg.CalculateHash(text)

			if cached, ok := existing[hash]; ok {
				svc.logger.WithCtx(ctx).Info("Embedding for document got from cache", "id", cached.ID)

				newIndex.Documents = append(
					newIndex.Documents,
					cached,
				)

				return nil
			}

			embedding, err := svc.embeddingService.Embedding(ctx, text)
			if err != nil {
				return fmt.Errorf("create embedding: %w", err)
			}

			newDocument := dto.Document{
				ID:        uuid.NewString(),
				Hash:      hash,
				Text:      text,
				Embedding: embedding,
			}

			newIndex.Documents = append(
				newIndex.Documents,
				newDocument,
			)

			svc.logger.WithCtx(ctx).Info("Embedding for document has been refreshed", "id", newDocument.ID, "hash", hash, "text", text)

			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	if err := svc.saveIndexAtomic(indexPath, newIndex); err != nil {
		return nil, err
	}

	return newIndex, nil
}

func (svc *IndexManager) loadIndex(indexPath string) (*dto.VectorIndex, error) {
	b, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, fmt.Errorf("read index: %w", err)
	}

	var index dto.VectorIndex
	if err := json.Unmarshal(b, &index); err != nil {
		return nil, fmt.Errorf("unmarshal index: %w", err)
	}

	return &index, nil
}

func (svc *IndexManager) streamDocument(
	ctx context.Context,
	path string,
	callback func(text string) error,
) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}

	defer func() {
		if err = file.Close(); err != nil {
			svc.logger.WithCtx(ctx).Error("failed to close file", "error", err, "path", path)
		}
	}()

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, buffSize)
	scanner.Buffer(buf, maxDocumentSize)

	var current strings.Builder

	flush := func() error {
		text := strings.TrimSpace(current.String())
		current.Reset()
		if text == "" {
			return nil
		}

		return callback(text)
	}

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Text()
		if line == "----" {
			if err := flush(); err != nil {
				return err
			}

			continue
		}

		current.WriteString(line)
		current.WriteByte('\n')
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan source file: %w", err)
	}

	return flush()
}

func (svc *IndexManager) saveIndexAtomic(
	path string,
	index *dto.VectorIndex,
) error {
	tmp := path + ".tmp"

	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}

	return os.Rename(tmp, path)
}
