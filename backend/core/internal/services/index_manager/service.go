package index_manager

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	log "gitlab.com/evgeniyprivalov/golib/observability/log"

	index_manager_dto "github.com/evgeniyprivalov/ai-vector-embedding/backend/core/internal/services/index_manager/dto"
	"github.com/evgeniyprivalov/ai-vector-embedding/backend/core/pkg"
)

type IndexManager interface {
	GetVectorIndex() *index_manager_dto.VectorIndex
	AutoSync(ctx context.Context) error
	Sync(ctx context.Context) error
}

type indexManager struct {
	vectorIndex      atomic.Pointer[index_manager_dto.VectorIndex]
	embeddingService embeddingService
	logger           *log.Logger
	indexPath        string
	documentsPath    string
}

func NewIndexManager(
	embeddingService embeddingService,
	logger *log.Logger,
	indexPath string,
	documentsPath string,
) IndexManager {
	return &indexManager{
		embeddingService: embeddingService,
		logger:           logger,
		indexPath:        indexPath,
		documentsPath:    documentsPath,
	}
}

func (svc *indexManager) GetVectorIndex() *index_manager_dto.VectorIndex {
	return svc.vectorIndex.Load()
}

func (svc *indexManager) AutoSync(ctx context.Context) error {
	if err := svc.Sync(ctx); err != nil {
		return err
	}

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := svc.Sync(ctx); err != nil {
					svc.logger.WithCtx(ctx).Error("failed to sync index manager", "error", err)
					continue
				}

				svc.logger.WithCtx(ctx).Info("synced index manager")
			}
		}
	}()

	return nil
}

func (svc *indexManager) Sync(ctx context.Context) error {
	currentIndex := svc.GetVectorIndex()

	if currentIndex == nil {
		index, err := svc.loadIndex()
		switch {
		case err == nil:
			currentIndex = index

		case errors.Is(err, os.ErrNotExist):
			currentIndex = &index_manager_dto.VectorIndex{}

		default:
			return fmt.Errorf("load index: %w", err)
		}
	}

	// hash -> document
	existing := make(map[string]index_manager_dto.Document, len(currentIndex.Documents))

	for _, doc := range currentIndex.Documents {
		existing[doc.Hash] = doc
	}

	// hash -> text
	actualDocuments := make(map[string]string)

	err := svc.streamDocument(
		ctx,
		func(text string) error {
			hash := pkg.CalculateHash(text)

			actualDocuments[hash] = text

			return nil
		},
	)
	if err != nil {
		return err
	}

	// Add new documents
	for hash, text := range actualDocuments {
		if _, ok := existing[hash]; ok {
			continue
		}

		embedding, err := svc.embeddingService.Embedding(ctx, text)
		if err != nil {
			return fmt.Errorf(
				"create embedding for %s: %w",
				hash,
				err,
			)
		}

		doc := index_manager_dto.Document{
			ID:        uuid.NewString(),
			Hash:      hash,
			Text:      text,
			Embedding: embedding,
		}

		existing[hash] = doc

		svc.logger.WithCtx(ctx).Info(
			"document added",
			"id", doc.ID,
			"hash", hash,
		)
	}

	// Remove deleted documents
	for hash, doc := range existing {
		if _, ok := actualDocuments[hash]; ok {
			continue
		}

		delete(existing, hash)

		svc.logger.WithCtx(ctx).Info(
			"document removed",
			"id", doc.ID,
			"hash", hash,
		)
	}

	newIndex := &index_manager_dto.VectorIndex{
		UpdatedAt: time.Now().UTC(),
		Documents: make([]index_manager_dto.Document, 0, len(existing)),
	}

	for _, doc := range existing {
		newIndex.Documents = append(
			newIndex.Documents,
			doc,
		)
	}

	svc.vectorIndex.Store(newIndex)

	if err := svc.saveIndexAtomic(newIndex); err != nil {
		return fmt.Errorf("save index: %w", err)
	}

	return nil
}

func (svc *indexManager) loadIndex() (*index_manager_dto.VectorIndex, error) {
	b, err := os.ReadFile(svc.indexPath)
	if err != nil {
		return nil, fmt.Errorf("read index: %w", err)
	}

	var index index_manager_dto.VectorIndex
	if err := json.Unmarshal(b, &index); err != nil {
		return nil, fmt.Errorf("unmarshal index: %w", err)
	}

	return &index, nil
}

func (svc *indexManager) streamDocument(
	ctx context.Context,
	callback func(text string) error,
) error {
	file, err := os.Open(svc.documentsPath)
	if err != nil {
		return err
	}

	defer func() {
		if err = file.Close(); err != nil {
			svc.logger.WithCtx(ctx).Error("failed to close file", "error", err, "path", svc.documentsPath)
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

func (svc *indexManager) saveIndexAtomic(index *index_manager_dto.VectorIndex) error {
	tmp := svc.indexPath + ".tmp"

	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}

	return os.Rename(tmp, svc.indexPath)
}
