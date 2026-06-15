package repository

import (
	"context"

	"github.com/Masterminds/squirrel"
	"github.com/pgvector/pgvector-go"
	"gitlab.com/evgeniyprivalov/golib/pg"

	models "ai-vector-embedding/internal/models"
)

const (
	documentsTable = "documents"
)

type DocumentsRepository struct {
	db *pg.Pool
	sq squirrel.StatementBuilderType
}

func NewDocumentsRepository(db *pg.Pool) *DocumentsRepository {
	return &DocumentsRepository{
		db: db,
		sq: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}
}

func (repo *DocumentsRepository) Create(ctx context.Context, data *models.Document) (int64, error) {
	sql, args, err := repo.sq.
		Insert(documentsTable).
		SetMap(squirrel.Eq{
			"content":   data.Content,
			"embedding": data.Embedding,
		}).
		Suffix("RETURNING id").
		ToSql()
	if err != nil {
		return 0, err
	}

	err = repo.db.QueryRow(ctx, sql, args...).Scan(&data.ID)
	if err != nil {
		return 0, err
	}

	return data.ID, nil
}

func (repo *DocumentsRepository) Search(ctx context.Context, queryEmbedding []float32, limit uint64) ([]models.Document, error) {
	sql := `
       SELECT id, content, embedding
       FROM documents
       ORDER BY embedding <=> $1
       LIMIT $2
    `

	var documents []models.Document
	rows, err := repo.db.Query(ctx, sql, pgvector.NewVector(queryEmbedding), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var document models.Document
		if err := rows.Scan(&document.ID, &document.Content, &document.Embedding); err != nil {
			return nil, err
		}

		documents = append(documents, document)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return documents, nil
}
