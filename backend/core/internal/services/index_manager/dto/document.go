package dto

type Document struct {
	ID        string    `json:"id"`
	Hash      string    `json:"hash"`
	Text      string    `json:"text"`
	Embedding []float32 `json:"embedding"`
}
