package dto

import "time"

type VectorIndex struct {
	UpdatedAt time.Time  `json:"updated_at"`
	Documents []Document `json:"documents"`
}
