package models

import (
	"github.com/pgvector/pgvector-go"
)

type Document struct {
	ID        int64           `json:"id"`
	Content   string          `json:"content"`
	Embedding pgvector.Vector `json:"embedding"`
}
